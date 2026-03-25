package collector

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// CoreSnapshot holds per-tick data for delta calculation
type CoreSnapshot struct {
	ID     string
	Active uint64
	Total  uint64
}

// CPUStats holds exhaustive CPU metrics matching btop's Cpu::collect()
type CPUStats struct {
	UsagePercent  float64    `json:"usage_percent"`
	CoresPercent  []float64  `json:"cores_percent"`
	FreqMHz       []uint64   `json:"freq_mhz"`
	CoreTempsC    []int64    `json:"core_temps_c,omitempty"`
	PackageTempC  int64      `json:"package_temp_c"`
	LoadAvg       [3]float64 `json:"load_avg"`
	UptimeSeconds float64    `json:"uptime_seconds"`
	PowerWatts    float64    `json:"power_watts"`
	CpuName       string     `json:"cpu_name"`
	// Battery info (btop also tracks this)
	BatteryPercent int    `json:"battery_percent,omitempty"`
	BatteryStatus  string `json:"battery_status,omitempty"`
}

// State variables
var prevCoreDeltas = make(map[string]CoreSnapshot)
var prevEnergyUJ int64
var prevEnergyTime time.Time

func CollectCPUStats() (CPUStats, error) {
	var stats CPUStats

	// 1. Uptime
	uptimeData, _ := os.ReadFile("/proc/uptime")
	if len(uptimeData) > 0 {
		fields := strings.Fields(string(uptimeData))
		if len(fields) > 0 {
			stats.UptimeSeconds, _ = strconv.ParseFloat(fields[0], 64)
		}
	}

	// 2. Load Average (btop uses getloadavg())
	loadData, _ := os.ReadFile("/proc/loadavg")
	if len(loadData) > 0 {
		fields := strings.Fields(string(loadData))
		if len(fields) >= 3 {
			stats.LoadAvg[0], _ = strconv.ParseFloat(fields[0], 64)
			stats.LoadAvg[1], _ = strconv.ParseFloat(fields[1], 64)
			stats.LoadAvg[2], _ = strconv.ParseFloat(fields[2], 64)
		}
	}

	// 3. CPU Name (btop reads from /proc/cpuinfo "model name")
	cpuInfoData, _ := os.ReadFile("/proc/cpuinfo")
	if len(cpuInfoData) > 0 {
		for _, line := range strings.Split(string(cpuInfoData), "\n") {
			if strings.HasPrefix(line, "model name") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					stats.CpuName = strings.TrimSpace(parts[1])
					break
				}
			}
		}
	}

	// 4. CPU Core Usages from /proc/stat
	// btop logic (line 1104-1118):
	//   fields: user(0), nice(1), system(2), idle(3), iowait(4), irq(5), softirq(6), steal(7), guest(8), guest_nice(9)
	//   totals = sum of ALL fields - sum of fields[8:] (subtract guest, guest_nice)
	//   idles  = idle(3) + iowait(4)
	//   percent = (totals - idles) / totals * 100
	file, err := os.Open("/proc/stat")
	if err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "cpu") {
				continue
			}
			fields := strings.Fields(line)
			id := fields[0]

			vals := make([]uint64, 0, 10)
			var totalSum uint64
			for _, field := range fields[1:] {
				val, _ := strconv.ParseUint(field, 10, 64)
				vals = append(vals, val)
				totalSum += val
			}

			// btop: subtract fields 8-9 and any future unknown fields from totals
			var guestSum uint64
			if len(vals) > 8 {
				for _, v := range vals[8:] {
					guestSum += v
				}
			}
			totals := totalSum - guestSum

			// btop: idles = idle(3) + iowait(4)
			var idles uint64
			if len(vals) > 3 {
				idles = vals[3]
			}
			if len(vals) > 4 {
				idles += vals[4]
			}

			curr := CoreSnapshot{ID: id, Active: totals - idles, Total: totals}
			prev, exists := prevCoreDeltas[id]

			var percent float64
			if exists {
				calcTotals := float64(curr.Total - prev.Total)
				if calcTotals > 0 {
					deltaActive := float64(curr.Active - prev.Active)
					percent = (deltaActive / calcTotals) * 100
					if percent < 0 {
						percent = 0
					} else if percent > 100 {
						percent = 100
					}
				}
			}

			if id == "cpu" {
				stats.UsagePercent = percent
			} else {
				stats.CoresPercent = append(stats.CoresPercent, percent)
			}
			prevCoreDeltas[id] = curr
		}
		file.Close()
	}

	// 5. CPU Frequencies (btop reads scaling_cur_freq)
	matches, _ := filepath.Glob("/sys/devices/system/cpu/cpufreq/policy*/scaling_cur_freq")
	for _, match := range matches {
		data, err := os.ReadFile(match)
		if err == nil {
			val, _ := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
			stats.FreqMHz = append(stats.FreqMHz, val/1000)
		}
	}

	// 6. CPU Temperatures
	// btop fallback chain: /sys/class/hwmon (coretemp) -> /sys/devices/platform/coretemp.0/hwmon -> /sys/class/thermal
	hwmonPaths, _ := filepath.Glob("/sys/class/hwmon/hwmon*")
	foundPackageTemp := false
	for _, hwmon := range hwmonPaths {
		nameData, err := os.ReadFile(filepath.Join(hwmon, "name"))
		if err != nil {
			continue
		}
		name := strings.TrimSpace(string(nameData))
		if name == "coretemp" || name == "k10temp" || name == "zenpower" {
			temps, _ := filepath.Glob(filepath.Join(hwmon, "temp*_input"))
			for _, tPath := range temps {
				labelBytes, _ := os.ReadFile(strings.Replace(tPath, "input", "label", 1))
				label := strings.TrimSpace(string(labelBytes))

				tData, _ := os.ReadFile(tPath)
				tVal, _ := strconv.ParseInt(strings.TrimSpace(string(tData)), 10, 64)
				tCelsius := tVal / 1000

				labelLower := strings.ToLower(label)
				if strings.Contains(labelLower, "package") || strings.Contains(labelLower, "tdie") || strings.Contains(labelLower, "tctl") {
					stats.PackageTempC = tCelsius
					foundPackageTemp = true
				} else if strings.Contains(labelLower, "core") || strings.Contains(labelLower, "tccd") {
					stats.CoreTempsC = append(stats.CoreTempsC, tCelsius)
				}
			}
			if foundPackageTemp {
				break
			}
		}
	}

	// btop fallback: /sys/devices/platform/coretemp.0/hwmon
	if !foundPackageTemp {
		coretemp, _ := filepath.Glob("/sys/devices/platform/coretemp.0/hwmon/hwmon*/temp*_input")
		for _, tPath := range coretemp {
			labelBytes, _ := os.ReadFile(strings.Replace(tPath, "input", "label", 1))
			label := strings.ToLower(strings.TrimSpace(string(labelBytes)))
			tData, _ := os.ReadFile(tPath)
			tVal, _ := strconv.ParseInt(strings.TrimSpace(string(tData)), 10, 64)

			if strings.Contains(label, "package") {
				stats.PackageTempC = tVal / 1000
				foundPackageTemp = true
			} else if strings.Contains(label, "core") {
				stats.CoreTempsC = append(stats.CoreTempsC, tVal/1000)
			}
		}
	}

	// btop fallback: /sys/class/thermal
	if !foundPackageTemp {
		zones, _ := filepath.Glob("/sys/class/thermal/thermal_zone*")
		for _, zone := range zones {
			tData, err := os.ReadFile(filepath.Join(zone, "temp"))
			if err == nil {
				tVal, _ := strconv.ParseInt(strings.TrimSpace(string(tData)), 10, 64)
				if tVal/1000 > stats.PackageTempC {
					stats.PackageTempC = tVal / 1000
				}
			}
		}
	}

	// 7. CPU Power Consumption (btop: get_cpuConsumptionWatts via /sys/class/powercap/intel-rapl:0/energy_uj)
	energyData, err := os.ReadFile("/sys/class/powercap/intel-rapl:0/energy_uj")
	if err == nil {
		currUJ, _ := strconv.ParseInt(strings.TrimSpace(string(energyData)), 10, 64)
		now := time.Now()
		if prevEnergyUJ > 0 && !prevEnergyTime.IsZero() {
			deltaUJ := float64(currUJ - prevEnergyUJ)
			deltaUS := float64(now.Sub(prevEnergyTime).Microseconds())
			if deltaUS > 0 {
				stats.PowerWatts = deltaUJ / deltaUS // microjoules / microseconds = watts
			}
		}
		prevEnergyUJ = currUJ
		prevEnergyTime = now
	}

	// 8. Battery (btop reads /sys/class/power_supply/*/capacity & status)
	batPaths, _ := filepath.Glob("/sys/class/power_supply/BAT*")
	if len(batPaths) > 0 {
		bat := batPaths[0]
		capData, _ := os.ReadFile(filepath.Join(bat, "capacity"))
		if len(capData) > 0 {
			stats.BatteryPercent, _ = strconv.Atoi(strings.TrimSpace(string(capData)))
		}
		statusData, _ := os.ReadFile(filepath.Join(bat, "status"))
		if len(statusData) > 0 {
			stats.BatteryStatus = strings.TrimSpace(string(statusData))
		}
	}

	return stats, nil
}
