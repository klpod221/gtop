package collector

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type AmdGPUStats struct {
	Name           string
	UtilizationGPU uint32
	VRAMTotal      uint64
	VRAMUsed       uint64
	TempC          int64
	PowerWatts     float64
	ClockCoreMHz   uint32
	ClockMemMHz    uint32
}

func CollectAmd() []AmdGPUStats {
	var statsList []AmdGPUStats

	// AMD GPUs typically expose sysfs bindings through the drm subsystem
	paths, _ := filepath.Glob("/sys/class/drm/card*")
	for _, p := range paths {
		// Verify it's AMD by checking the vendor id (0x1002 is AMD)
		vendorData, err := os.ReadFile(filepath.Join(p, "device", "vendor"))
		if err != nil || !strings.Contains(string(vendorData), "0x1002") {
			continue
		}

		var stats AmdGPUStats
		stats.Name = "AMD Radeon GPU"

		// GPU Utilization
		busyData, err := os.ReadFile(filepath.Join(p, "device", "gpu_busy_percent"))
		if err == nil {
			val, _ := strconv.ParseUint(strings.TrimSpace(string(busyData)), 10, 32)
			stats.UtilizationGPU = uint32(val)
		}

		// VRAM Memory
		vramTotal, errTotal := os.ReadFile(filepath.Join(p, "device", "mem_info_vram_total"))
		vramUsed, errUsed := os.ReadFile(filepath.Join(p, "device", "mem_info_vram_used"))
		if errTotal == nil && errUsed == nil {
			stats.VRAMTotal, _ = strconv.ParseUint(strings.TrimSpace(string(vramTotal)), 10, 64)
			stats.VRAMUsed, _ = strconv.ParseUint(strings.TrimSpace(string(vramUsed)), 10, 64)
		}

		// AMD exposes temps and power through nested hwmon directories
		hwmonDirs, _ := filepath.Glob(filepath.Join(p, "device", "hwmon", "hwmon*"))
		if len(hwmonDirs) > 0 {
			hwmon := hwmonDirs[0]

			// Temperature (Usually temp1 is Edge, temp2 is Junction)
			tempData, err := os.ReadFile(filepath.Join(hwmon, "temp1_input"))
			if err == nil {
				val, _ := strconv.ParseInt(strings.TrimSpace(string(tempData)), 10, 64)
				stats.TempC = val / 1000 // milli-Celsius -> Celsius
			}

			// Power (Average or Input)
			pwrData, err := os.ReadFile(filepath.Join(hwmon, "power1_average"))
			if err != nil {
				pwrData, err = os.ReadFile(filepath.Join(hwmon, "power1_input"))
			}
			if err == nil {
				val, _ := strconv.ParseFloat(strings.TrimSpace(string(pwrData)), 64)
				stats.PowerWatts = val / 1000000.0 // micro-Watts -> Watts
			}
		}

		// Clock Speeds (Parse active state signaled by '*')
		// pp_dpm_sclk for Core/System Clock
		sclkData, err := os.ReadFile(filepath.Join(p, "device", "pp_dpm_sclk"))
		if err == nil {
			stats.ClockCoreMHz = parseAmdClock(string(sclkData))
		}
		// pp_dpm_mclk for Memory Clock
		mclkData, err := os.ReadFile(filepath.Join(p, "device", "pp_dpm_mclk"))
		if err == nil {
			stats.ClockMemMHz = parseAmdClock(string(mclkData))
		}

		statsList = append(statsList, stats)
	}

	return statsList
}

// parseAmdClock parses the active clock from pp_dpm_sclk which looks like:
// 0: 300Mhz
// 1: 1046Mhz *
func parseAmdClock(data string) uint32 {
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		if strings.HasSuffix(strings.TrimSpace(line), "*") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				// e.g "1046Mhz"
				valStr := strings.TrimSuffix(strings.ToUpper(parts[1]), "MHZ")
				val, _ := strconv.ParseUint(valStr, 10, 32)
				return uint32(val)
			}
		}
	}
	return 0
}
