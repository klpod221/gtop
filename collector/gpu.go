package collector

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

// EngineUsage holds per-engine utilization data
type EngineUsage struct {
	Name    string  `json:"name"`
	BusyPct float64 `json:"busy_pct"`
}

// IntelGPUStats holds all Intel GPU metrics matching btop's pmu_sample() + pmu_calc()
type IntelGPUStats struct {
	Engines       []EngineUsage `json:"engines"`
	FreqActMHz    float64       `json:"freq_act_mhz"`
	FreqReqMHz    float64       `json:"freq_req_mhz"`
	RC6Pct        float64       `json:"rc6_pct"`
	PowerGPUWatts float64       `json:"power_gpu_watts"`
	PowerPkgWatts float64       `json:"power_pkg_watts"`
	IMCReadsMBs   float64       `json:"imc_reads_mbs"`
	IMCWritesMBs  float64       `json:"imc_writes_mbs"`
}

// pmuCounter mirrors btop's struct pmu_counter
type pmuCounter struct {
	typ     uint64
	config  uint64
	idx     int
	scale   float64
	present bool
	prev    uint64
	cur     uint64
}

// engineInfo mirrors btop's struct engine
type engineInfo struct {
	name string
	busy pmuCounter
}

// IntelGPUCollector manages the perf event group fd lifecycle
type IntelGPUCollector struct {
	fd          int // main group fd
	raplFd      int // RAPL group fd
	imcFd       int // IMC group fd
	numCounters int
	numRapl     int
	numImc      int

	engines []engineInfo
	freqReq pmuCounter
	freqAct pmuCounter
	rc6     pmuCounter
	irq     pmuCounter
	rGpu    pmuCounter
	rPkg    pmuCounter
	imcRead pmuCounter
	imcWrite pmuCounter

	tsPrev uint64
	tsCur  uint64
}

// findIntelPMUDevice finds "i915" device type under /sys/bus/event_source/devices/
func findIntelPMUDevice() (string, uint64, error) {
	// Try "i915" first (most common)
	for _, dev := range []string{"i915", "xe"} {
		typePath := fmt.Sprintf("/sys/bus/event_source/devices/%s/type", dev)
		data, err := os.ReadFile(typePath)
		if err == nil {
			pmuType, _ := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
			if pmuType > 0 {
				return dev, pmuType, nil
			}
		}
	}
	return "", 0, fmt.Errorf("no Intel GPU PMU device found")
}

// pmuParseEvent reads config and scale from sysfs PMU events directory
func pmuParseEvent(basePath string, eventName string) (config uint64, scale float64, err error) {
	// Read config: events/{eventName} contains "config=0xNN" or "event=0xNN"
	eventData, err := os.ReadFile(filepath.Join(basePath, "events", eventName))
	if err != nil {
		return 0, 0, err
	}
	eventStr := strings.TrimSpace(string(eventData))

	// sysfs i915 uses "config=0xNN", some other PMU use "event=0xNN"
	for _, prefix := range []string{"config=", "event="} {
		if strings.HasPrefix(eventStr, prefix) {
			config, _ = strconv.ParseUint(strings.TrimPrefix(eventStr, prefix), 0, 64)
			break
		}
	}

	// Read scale (optional)
	scaleData, err := os.ReadFile(filepath.Join(basePath, "events", eventName+".scale"))
	if err == nil {
		scale, _ = strconv.ParseFloat(strings.TrimSpace(string(scaleData)), 64)
	} else {
		scale = 1.0
	}

	return config, scale, nil
}

// perfOpen opens a perf event, optionally joining a group
// Replicates btop's _perf_open() from igt_perf.c
func perfOpen(pmuType, config uint64, groupFd int) (int, error) {
	attr := unix.PerfEventAttr{
		Type:   uint32(pmuType),
		Config: config,
		Size:   uint32(unsafe.Sizeof(unix.PerfEventAttr{})),
	}

	// btop: attr.use_clockid = 1; attr.clockid = CLOCK_MONOTONIC;
	attr.Clockid = unix.CLOCK_MONOTONIC
	attr.Bits |= 1 << 24 // use_clockid bit

	// btop: group leader gets PERF_FORMAT_TOTAL_TIME_ENABLED | PERF_FORMAT_GROUP
	// group members also get TOTAL_TIME_ENABLED but NOT GROUP
	if groupFd == -1 {
		attr.Read_format = unix.PERF_FORMAT_TOTAL_TIME_ENABLED | unix.PERF_FORMAT_GROUP
	} else {
		attr.Read_format = unix.PERF_FORMAT_TOTAL_TIME_ENABLED
	}

	// NOTE: btop does NOT set PerfBitDisabled — perf events start counting immediately

	// btop: tries cpu 0 first, then iterates on EINVAL
	var fd int
	var err error
	for cpu := 0; cpu < 128; cpu++ {
		fd, err = unix.PerfEventOpen(&attr, -1, cpu, groupFd, 0)
		if err == nil {
			return fd, nil
		}
		if err != unix.EINVAL {
			break
		}
	}
	return -1, fmt.Errorf("perf_event_open failed for config 0x%x: %w", config, err)
}

// NewIntelGPUCollector initializes the Intel GPU collector exactly like btop's discover_engines() + pmu_init()
func NewIntelGPUCollector() (*IntelGPUCollector, error) {
	devName, pmuType, err := findIntelPMUDevice()
	if err != nil {
		return nil, err
	}

	basePath := fmt.Sprintf("/sys/bus/event_source/devices/%s", devName)
	col := &IntelGPUCollector{
		fd:     -1,
		raplFd: -1,
		imcFd:  -1,
	}

	// Step 1: Discover engines from events directory (btop: discover_engines)
	eventsDir := filepath.Join(basePath, "events")
	entries, err := os.ReadDir(eventsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read events dir: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, "-busy") {
			continue
		}
		engineName := strings.TrimSuffix(name, "-busy")

		config, _, err := pmuParseEvent(basePath, name)
		if err != nil {
			continue
		}

		col.engines = append(col.engines, engineInfo{
			name: engineName,
			busy: pmuCounter{typ: pmuType, config: config},
		})
	}

	// Step 2: Open grouped perf events (btop: pmu_init)
	if len(col.engines) == 0 {
		return nil, fmt.Errorf("no GPU engines discovered")
	}

	// btop: uses I915_PMU_INTERRUPTS as group leader
	// Read interrupts config from sysfs
	irqConfig, _, _ := pmuParseEvent(basePath, "interrupts")
	col.irq.config = irqConfig

	fd, err := perfOpen(pmuType, col.irq.config, -1)
	if err != nil {
		return nil, fmt.Errorf("failed to open group leader (interrupts): %w", err)
	}
	col.fd = fd
	col.irq.idx = col.numCounters
	col.irq.present = true
	col.numCounters++

	// Read freq/rc6 configs from sysfs (btop: init_aggregate_counters)
	for _, setup := range []struct {
		pmu       *pmuCounter
		eventName string
	}{
		{&col.freqReq, "requested-frequency"},
		{&col.freqAct, "actual-frequency"},
		{&col.rc6, "rc6-residency"},
	} {
		config, _, err := pmuParseEvent(basePath, setup.eventName)
		if err != nil {
			continue
		}
		_, err = perfOpen(pmuType, config, col.fd)
		if err == nil {
			setup.pmu.config = config
			setup.pmu.idx = col.numCounters
			setup.pmu.present = true
			col.numCounters++
		}
	}

	// Open engine counters into the group
	for i := range col.engines {
		_, err := perfOpen(pmuType, col.engines[i].busy.config, col.fd)
		if err == nil {
			col.engines[i].busy.idx = col.numCounters
			col.engines[i].busy.present = true
			col.numCounters++
		}
	}

	// Step 3: Open RAPL (separate group fd) - btop: rapl_open
	raplBasePath := "/sys/devices/power"
	for _, setup := range []struct {
		pmu  *pmuCounter
		name string
	}{
		{&col.rGpu, "energy-gpu"},
		{&col.rPkg, "energy-pkg"},
	} {
		config, scale, err := pmuParseEvent(raplBasePath, setup.name)
		if err != nil {
			continue
		}

		// Read RAPL PMU type
		raplTypeData, err := os.ReadFile(filepath.Join(raplBasePath, "type"))
		if err != nil {
			continue
		}
		raplType, _ := strconv.ParseUint(strings.TrimSpace(string(raplTypeData)), 10, 64)

		fd, err := perfOpen(raplType, config, col.raplFd)
		if err == nil {
			if col.raplFd == -1 {
				col.raplFd = fd
			}
			setup.pmu.idx = col.numRapl
			setup.pmu.scale = scale
			setup.pmu.present = true
			col.numRapl++
		}
	}

	// Step 4: Open IMC (memory bandwidth) - btop: imc_open
	imcBasePath := "/sys/devices/uncore_imc"
	for _, setup := range []struct {
		pmu  *pmuCounter
		name string
	}{
		{&col.imcRead, "data_reads"},
		{&col.imcWrite, "data_writes"},
	} {
		config, scale, err := pmuParseEvent(imcBasePath, setup.name)
		if err != nil {
			continue
		}
		imcTypeData, err := os.ReadFile(filepath.Join(imcBasePath, "type"))
		if err != nil {
			continue
		}
		imcType, _ := strconv.ParseUint(strings.TrimSpace(string(imcTypeData)), 10, 64)

		fd, err := perfOpen(imcType, config, col.imcFd)
		if err == nil {
			if col.imcFd == -1 {
				col.imcFd = fd
			}
			setup.pmu.idx = col.numImc
			setup.pmu.scale = scale
			setup.pmu.present = true
			col.numImc++
		}
	}

	return col, nil
}

// pmuReadMulti reads grouped perf event values (btop: pmu_read_multi)
// Returns timestamp_enabled and fills vals array
func pmuReadMulti(fd int, numCounters int) (uint64, []uint64, error) {
	// Read format: [nr, time_enabled, val0, val1, ...]
	bufSize := (2 + numCounters) * 8
	buf := make([]byte, bufSize)
	n, err := unix.Read(fd, buf)
	if err != nil || n < bufSize {
		return 0, nil, fmt.Errorf("pmu read failed: %w (read %d of %d)", err, n, bufSize)
	}

	timeEnabled := binary.LittleEndian.Uint64(buf[8:16])
	vals := make([]uint64, numCounters)
	for i := 0; i < numCounters; i++ {
		vals[i] = binary.LittleEndian.Uint64(buf[(2+i)*8 : (3+i)*8])
	}
	return timeEnabled, vals, nil
}

// updateSample updates prev/cur for a counter (btop: update_sample)
func updateSample(c *pmuCounter, vals []uint64) {
	if c.present && c.idx < len(vals) {
		c.prev = c.cur
		c.cur = vals[c.idx]
	}
}

// pmuCalc replicates btop's pmu_calc(). d=deltaNs from timestamp, t=time_scale, s=result_scale
func pmuCalc(prev, cur uint64, deltaNs float64, timeScale float64, resultScale float64) float64 {
	v := float64(cur - prev)
	v /= deltaNs
	v /= timeScale
	v *= resultScale
	if resultScale == 100.0 && v > 100.0 {
		v = 100.0
	}
	return v
}

// Collect performs one sample cycle (btop: pmu_sample + pmu_calc)
func (c *IntelGPUCollector) Collect() IntelGPUStats {
	var stats IntelGPUStats

	if c.fd < 0 {
		return stats
	}

	// Read main group
	c.tsPrev = c.tsCur
	ts, vals, err := pmuReadMulti(c.fd, c.numCounters)
	if err != nil {
		return stats
	}
	c.tsCur = ts

	// Update all counters
	for i := range c.engines {
		updateSample(&c.engines[i].busy, vals)
	}
	updateSample(&c.freqReq, vals)
	updateSample(&c.freqAct, vals)
	updateSample(&c.rc6, vals)

	// Read RAPL group (if available)
	if c.raplFd >= 0 && c.numRapl > 0 {
		_, raplVals, err := pmuReadMulti(c.raplFd, c.numRapl)
		if err == nil {
			updateSample(&c.rGpu, raplVals)
			updateSample(&c.rPkg, raplVals)
		}
	}

	// Read IMC group (if available)
	if c.imcFd >= 0 && c.numImc > 0 {
		_, imcVals, err := pmuReadMulti(c.imcFd, c.numImc)
		if err == nil {
			updateSample(&c.imcRead, imcVals)
			updateSample(&c.imcWrite, imcVals)
		}
	}

	// Calculate values (need at least 2 samples)
	if c.tsPrev == 0 {
		return stats
	}
	deltaNs := float64(c.tsCur - c.tsPrev)
	if deltaNs <= 0 {
		return stats
	}

	// btop convention: t = deltaNs / 1e9 (seconds)
	t := deltaNs / 1e9

	// Engine utilization: pmu_calc(&busy, 1e9, t, 100)
	// v = (cur-prev) / 1e9 / t * 100 = ns_busy / 1e9 / seconds * 100 = busy_%
	for _, eng := range c.engines {
		if eng.busy.present {
			pct := pmuCalc(eng.busy.prev, eng.busy.cur, 1e9, t, 100.0)
			stats.Engines = append(stats.Engines, EngineUsage{
				Name:    eng.name,
				BusyPct: pct,
			})
		}
	}

	// Frequency: pmu_calc(&freq_act, 1, t, 1)
	// v = (cur-prev) / 1 / t * 1 = cumulative / seconds = MHz
	if c.freqAct.present {
		stats.FreqActMHz = pmuCalc(c.freqAct.prev, c.freqAct.cur, 1.0, t, 1.0)
	}
	if c.freqReq.present {
		stats.FreqReqMHz = pmuCalc(c.freqReq.prev, c.freqReq.cur, 1.0, t, 1.0)
	}

	// RC6: pmu_calc(&rc6, 1e9, t, 100)
	if c.rc6.present {
		stats.RC6Pct = pmuCalc(c.rc6.prev, c.rc6.cur, 1e9, t, 100.0)
	}

	// Power: pmu_calc(&r_gpu, 1, t, scale) -> Watts
	if c.rGpu.present && c.rGpu.scale != 0 {
		stats.PowerGPUWatts = pmuCalc(c.rGpu.prev, c.rGpu.cur, 1.0, t, c.rGpu.scale)
	}
	if c.rPkg.present && c.rPkg.scale != 0 {
		stats.PowerPkgWatts = pmuCalc(c.rPkg.prev, c.rPkg.cur, 1.0, t, c.rPkg.scale)
	}

	// IMC: similar to power
	if c.imcRead.present && c.imcRead.scale != 0 {
		stats.IMCReadsMBs = pmuCalc(c.imcRead.prev, c.imcRead.cur, 1.0, t, c.imcRead.scale) / (1024 * 1024)
	}
	if c.imcWrite.present && c.imcWrite.scale != 0 {
		stats.IMCWritesMBs = pmuCalc(c.imcWrite.prev, c.imcWrite.cur, 1.0, t, c.imcWrite.scale) / (1024 * 1024)
	}

	return stats
}

// Close releases all file descriptors
func (c *IntelGPUCollector) Close() {
	if c.fd >= 0 {
		unix.Close(c.fd)
	}
	if c.raplFd >= 0 {
		unix.Close(c.raplFd)
	}
	if c.imcFd >= 0 {
		unix.Close(c.imcFd)
	}
}
