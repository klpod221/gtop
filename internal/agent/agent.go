package agent

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gtop/internal/collector"
	"gtop/internal/config"
)

// AgentPayload mirrors the cmd.AgentPayload to avoid package coupling.
type AgentPayload struct {
	Timestamp   int64                      `json:"timestamp"`
	MachineID   string                     `json:"machine_id,omitempty"`
	MachineName string                     `json:"machine_name,omitempty"`
	Tags        map[string]string          `json:"tags,omitempty"`
	Host        *collector.HostInfo        `json:"host,omitempty"`
	CPU         *collector.CPUStats        `json:"cpu,omitempty"`
	Memory      *collector.MemStats        `json:"memory,omitempty"`
	DisksSpace  []collector.DiskSpace      `json:"disks_space,omitempty"`
	DisksIO     []collector.DiskIO         `json:"disks_io,omitempty"`
	Network     []collector.NetInterface   `json:"network,omitempty"`
	Processes   []collector.ProcessInfo    `json:"processes,omitempty"`
	IntelGPU    *collector.IntelGPUStats   `json:"intel_gpu,omitempty"`
	NvidiaGPUs  []collector.NvidiaGPUStats `json:"nvidia_gpus,omitempty"`
	AmdGPUs     []collector.AmdGPUStats    `json:"amd_gpus,omitempty"`
}

// RunOptions controls single-run agent behavior (for CLI flags).
type RunOptions struct {
	// ConfigPath overrides the default config file path.
	ConfigPath string

	// DryRun collects data but prints to stderr instead of sending.
	DryRun bool

	// Once exits after a single collection cycle.
	Once bool
}

// Run is the main entry point for the agent daemon.
// It loads config, initialises collectors, and starts the collection loop.
func Run(opts RunOptions) error {
	cfg, err := loadConfig(opts.ConfigPath)
	if err != nil {
		return err
	}

	logger := buildLogger(cfg.Agent)

	machineID, err := resolveMachineID(cfg.Agent.MachineID)
	if err != nil {
		logger.Printf("WARN: could not resolve machine ID: %v", err)
	}
	machineName := resolveMachineName(cfg.Agent.MachineName)

	if err := writePID(cfg.Agent.PIDFile); err != nil {
		logger.Printf("WARN: could not write PID file: %v", err)
	}
	defer removePID(cfg.Agent.PIDFile)

	var sender *Sender
	if !opts.DryRun {
		sender, err = NewSender(cfg.Server)
		if err != nil {
			return fmt.Errorf("initialising sender: %w", err)
		}
	}

	// Initialise stateful collectors that need a warm-up tick.
	var intelCol *collector.IntelGPUCollector
	if cfg.Modules.GPU.Enabled && cfg.Modules.GPU.Intel {
		intelCol, err = collector.NewIntelGPUCollector()
		if err != nil {
			if !strings.Contains(err.Error(), "no Intel GPU PMU device found") &&
				!strings.Contains(err.Error(), "no GPU engines discovered") {
				logger.Printf("WARN Intel GPU: %v", err)
			}
		}
		if intelCol != nil {
			defer intelCol.Close()
			intelCol.Collect() // warm-up
		}
		if cfg.Modules.GPU.Nvidia {
			if nvErr := collector.InitNvidia(); nvErr != nil {
				if !strings.Contains(nvErr.Error(), "LIBRARY_NOT_FOUND") {
					logger.Printf("WARN NVIDIA GPU: %v", nvErr)
				}
			}
			defer collector.ShutdownNvidia()
		}
	}

	// CPU needs one baseline measurement before delta can be computed.
	if cfg.Modules.CPU.Enabled {
		collector.CollectCPUStats()
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	interval := time.Duration(cfg.Agent.IntervalSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	logger.Printf("INFO gtop-agent started (interval=%s, dry_run=%v)", interval, opts.DryRun)

	for {
		select {
		case <-stop:
			logger.Println("INFO gtop-agent shutting down")
			return nil
		case <-ticker.C:
			payload := collect(cfg, intelCol, machineID, machineName)

			jsonData, err := json.Marshal(payload)
			if err != nil {
				logger.Printf("ERROR marshalling payload: %v", err)
				continue
			}

			if opts.DryRun {
				fmt.Fprintf(os.Stderr, "[dry-run] payload (%d bytes):\n%s\n", len(jsonData), prettyJSON(jsonData))
			} else {
				if err := sender.Send(jsonData); err != nil {
					logger.Printf("ERROR sending payload: %v", err)
				} else {
					logger.Printf("DEBUG sent %d bytes to %s", len(jsonData), cfg.Server.Endpoint)
				}
			}

			if opts.Once {
				return nil
			}
		}
	}
}

// collect gathers all enabled metrics and returns an AgentPayload.
func collect(cfg config.AgentConfig, intelCol *collector.IntelGPUCollector, machineID, machineName string) AgentPayload {
	m := cfg.Modules
	payload := AgentPayload{
		Timestamp:   time.Now().UnixMilli(),
		MachineID:   machineID,
		MachineName: machineName,
		Tags:        cfg.Agent.Tags,
	}

	if m.Host.Enabled {
		h := collector.CollectHostInfo()
		payload.Host = &h
	}

	if m.CPU.Enabled {
		stats, _ := collector.CollectCPUStats()
		if len(m.CPU.Fields) > 0 {
			stats = filterCPUFields(stats, m.CPU.Fields)
		}
		payload.CPU = &stats
	}

	if m.Memory.Enabled {
		mem, _ := collector.CollectMem()
		payload.Memory = &mem
	}

	if m.Disk.Enabled {
		payload.DisksSpace = collector.CollectDisksSpace()
		payload.DisksIO = collector.CollectDisksIO()
		if len(m.Disk.MountFilter) > 0 {
			payload.DisksSpace = filterDiskMounts(payload.DisksSpace, m.Disk.MountFilter)
		}
	}

	if m.Network.Enabled {
		nets := collector.CollectNetwork()
		if len(m.Network.IfaceFilter) > 0 {
			nets = filterNetIfaces(nets, m.Network.IfaceFilter)
		}
		if m.Network.ExcludeVirtual {
			nets = excludeVirtualIfaces(nets)
		}
		payload.Network = nets
	}

	if m.Processes.Enabled {
		var totalMem uint64
		if payload.Memory != nil {
			totalMem = payload.Memory.Total
		} else {
			mem, _ := collector.CollectMem()
			totalMem = mem.Total
		}
		procs := collector.CollectProcesses(totalMem)
		if m.Processes.NameFilter != "" {
			procs = filterProcesses(procs, m.Processes.NameFilter)
		}
		sortProcesses(procs, m.Processes.SortBy)
		if m.Processes.TopN > 0 && len(procs) > m.Processes.TopN {
			procs = procs[:m.Processes.TopN]
		}
		payload.Processes = procs
	}

	if m.GPU.Enabled {
		if intelCol != nil && m.GPU.Intel {
			stats := intelCol.Collect()
			if len(stats.Engines) > 0 || stats.FreqActMHz > 0 {
				payload.IntelGPU = &stats
			}
		}
		if m.GPU.Nvidia {
			nv, _ := collector.CollectNvidia()
			if len(nv) > 0 {
				payload.NvidiaGPUs = nv
			}
		}
		if m.GPU.AMD {
			amd := collector.CollectAmd()
			if len(amd) > 0 {
				payload.AmdGPUs = amd
			}
		}
	}

	return payload
}

// --- helpers ---

func loadConfig(path string) (config.AgentConfig, error) {
	if path == "" {
		var err error
		path, err = config.DefaultConfigPath()
		if err != nil {
			return config.AgentConfig{}, err
		}
	}
	return config.Load(path)
}

func buildLogger(cfg config.AgentBehaviorConfig) *log.Logger {
	flags := log.Ldate | log.Ltime
	if cfg.LogFile != "" {
		f, err := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			return log.New(f, "", flags)
		}
		fmt.Fprintf(os.Stderr, "WARN: cannot open log file %s: %v; using stderr\n", cfg.LogFile, err)
	}
	return log.New(os.Stderr, "", flags)
}

func resolveMachineID(configured string) (string, error) {
	if configured != "" {
		return configured, nil
	}
	data, err := os.ReadFile("/etc/machine-id")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func resolveMachineName(configured string) string {
	if configured != "" {
		return configured
	}
	name, _ := os.Hostname()
	return name
}

func writePID(path string) error {
	if path == "" {
		return nil
	}
	return os.WriteFile(path, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644)
}

func removePID(path string) {
	if path != "" {
		os.Remove(path)
	}
}

func prettyJSON(data []byte) []byte {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return data
	}
	out, _ := json.MarshalIndent(v, "", "  ")
	return out
}

// --- filter helpers (mirrors cmd/get.go logic without duplicate package coupling) ---

func filterCPUFields(stats collector.CPUStats, fields []string) collector.CPUStats {
	set := make(map[string]bool, len(fields))
	for _, f := range fields {
		set[f] = true
	}
	var out collector.CPUStats
	if set["usage"] {
		out.UsagePercent = stats.UsagePercent
		out.CoresPercent = stats.CoresPercent
	}
	if set["freq"] {
		out.FreqMHz = stats.FreqMHz
	}
	if set["temp"] {
		out.CoreTempsC = stats.CoreTempsC
		out.PackageTempC = stats.PackageTempC
	}
	if set["power"] {
		out.PowerWatts = stats.PowerWatts
	}
	if set["loadavg"] {
		out.LoadAvg = stats.LoadAvg
	}
	if set["uptime"] {
		out.UptimeSeconds = stats.UptimeSeconds
	}
	if set["name"] {
		out.CpuName = stats.CpuName
	}
	if set["battery"] {
		out.BatteryPercent = stats.BatteryPercent
		out.BatteryStatus = stats.BatteryStatus
	}
	return out
}

func filterDiskMounts(disks []collector.DiskSpace, mounts []string) []collector.DiskSpace {
	var out []collector.DiskSpace
	for _, d := range disks {
		for _, m := range mounts {
			if strings.TrimSpace(m) == d.MountPoint {
				out = append(out, d)
				break
			}
		}
	}
	return out
}

func filterNetIfaces(ifaces []collector.NetInterface, names []string) []collector.NetInterface {
	var out []collector.NetInterface
	for _, iface := range ifaces {
		for _, name := range names {
			if strings.TrimSpace(name) == iface.Name {
				out = append(out, iface)
				break
			}
		}
	}
	return out
}

// excludeVirtualIfaces removes Docker bridges, veth pairs, and loopback.
func excludeVirtualIfaces(ifaces []collector.NetInterface) []collector.NetInterface {
	var out []collector.NetInterface
	for _, iface := range ifaces {
		n := iface.Name
		if strings.HasPrefix(n, "br-") ||
			strings.HasPrefix(n, "veth") ||
			strings.HasPrefix(n, "docker") ||
			n == "lo" {
			continue
		}
		out = append(out, iface)
	}
	return out
}

func filterProcesses(procs []collector.ProcessInfo, filter string) []collector.ProcessInfo {
	low := strings.ToLower(filter)
	var out []collector.ProcessInfo
	for _, p := range procs {
		if strings.Contains(strings.ToLower(p.Name), low) ||
			strings.Contains(strings.ToLower(p.Cmdline), low) {
			out = append(out, p)
		}
	}
	return out
}

func sortProcesses(procs []collector.ProcessInfo, sortBy string) {
	// Import sort inline to avoid an extra import at package level.
	// This mirrors cmd/get.go logic exactly.
	type lessFunc func(i, j int) bool
	doSort := func(less lessFunc) {
		n := len(procs)
		for i := 1; i < n; i++ {
			for j := i; j > 0 && less(j, j-1); j-- {
				procs[j], procs[j-1] = procs[j-1], procs[j]
			}
		}
	}
	switch sortBy {
	case "cpu":
		doSort(func(i, j int) bool { return procs[i].CpuPercent > procs[j].CpuPercent })
	case "mem":
		doSort(func(i, j int) bool { return procs[i].MemRSSBytes > procs[j].MemRSSBytes })
	case "pid":
		doSort(func(i, j int) bool { return procs[i].PID < procs[j].PID })
	case "name":
		doSort(func(i, j int) bool {
			return strings.ToLower(procs[i].Name) < strings.ToLower(procs[j].Name)
		})
	case "io":
		doSort(func(i, j int) bool {
			return (procs[i].IO_ReadBytes + procs[i].IO_WriteBytes) > (procs[j].IO_ReadBytes + procs[j].IO_WriteBytes)
		})
	}
}
