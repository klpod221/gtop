package collector

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// MemStats holds exhaustive memory metrics matching btop's Mem::collect()
type MemStats struct {
	Total       uint64   `json:"total"`
	Free        uint64   `json:"free"`
	Available   uint64   `json:"available"`
	Buffers     uint64   `json:"buffers"`
	Cached      uint64   `json:"cached"`
	SwapTotal   uint64   `json:"swap_total"`
	SwapFree    uint64   `json:"swap_free"`
	PhysicalRAM []string `json:"physical_ram,omitempty"` // Example: "8192 MB DDR4 2666 MT/s Kingston"
	Used        uint64   `json:"used"`
	SwapUsed    uint64   `json:"swap_used"`
	ZFSArc      uint64   `json:"zfs_arc,omitempty"`
}

// CollectMem replicates btop's Mem::collect() for /proc/meminfo and ZFS
func CollectMem() (MemStats, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return MemStats{}, err
	}
	defer file.Close()

	var stats MemStats
	var free uint64
	gotAvail := false

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		key := parts[0]
		val, _ := strconv.ParseUint(parts[1], 10, 64)
		valBytes := val << 10 // kB to Bytes (btop uses <<= 10)

		switch key {
		case "MemTotal:":
			stats.Total = valBytes
		case "MemFree:":
			free = valBytes
			stats.Free = valBytes
		case "MemAvailable:":
			stats.Available = valBytes
			gotAvail = true
		case "Cached:":
			stats.Cached = valBytes
		case "Buffers:":
			stats.Buffers = valBytes
		case "SwapTotal:":
			stats.SwapTotal = valBytes
		case "SwapFree:":
			stats.SwapFree = valBytes
		}
	}

	// btop: if not got_avail, Available = Free + Cached
	if !gotAvail {
		stats.Available = free + stats.Cached
	}

	// btop: ZFS ARC Cache with c_min logic
	var arcSize, arcMinSize uint64
	zfsData, err := os.ReadFile("/proc/spl/kstat/zfs/arcstats")
	if err == nil {
		zfsScanner := bufio.NewScanner(strings.NewReader(string(zfsData)))
		for zfsScanner.Scan() {
			zfsLine := zfsScanner.Text()
			zfsParts := strings.Fields(zfsLine)
			if len(zfsParts) >= 3 {
				switch zfsParts[0] {
				case "c_min":
					arcMinSize, _ = strconv.ParseUint(zfsParts[2], 10, 64)
				case "size":
					arcSize, _ = strconv.ParseUint(zfsParts[2], 10, 64)
				}
			}
		}
		stats.ZFSArc = arcSize

		if arcSize > 0 {
			// btop: cached += arc_size
			stats.Cached += arcSize
			// btop: only add (arc_size - arc_min_size) to available, because ARC won't shrink below c_min
			if arcSize > arcMinSize {
				stats.Available += arcSize - arcMinSize
			}
		}
	}

	// btop: Used = Total - (Available <= Total ? Available : Free)
	if stats.Available <= stats.Total {
		stats.Used = stats.Total - stats.Available
	} else {
		stats.Used = stats.Total - free
	}

	// Swap
	if stats.SwapTotal > stats.SwapFree {
		stats.SwapUsed = stats.SwapTotal - stats.SwapFree
	}

	// Try extracting physical RAM structure (requires root / sudo)
	stats.PhysicalRAM = extractPhysicalRAM()

	return stats, nil
}

// extractPhysicalRAM runs `dmidecode -t memory` and parses populated DIMMs.
// It will silently return nil if dmidecode is missing or permission is denied.
func extractPhysicalRAM() []string {
	cmd := exec.Command("dmidecode", "-t", "memory")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var dimms []string
	var currentSize, currentType, currentSpeed, currentMfr string

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		if line == "Memory Device" {
			// Save previous DIMM if valid
			if currentSize != "" && currentSize != "No Module Installed" {
				dimms = append(dimms, fmt.Sprintf("%s %s %s %s", currentSize, currentType, currentSpeed, currentMfr))
			}
			// Reset for next
			currentSize, currentType, currentSpeed, currentMfr = "", "", "", ""
			continue
		}

		if strings.HasPrefix(line, "Size:") {
			currentSize = strings.TrimSpace(strings.TrimPrefix(line, "Size:"))
		} else if strings.HasPrefix(line, "Type:") && !strings.HasPrefix(line, "Type Detail:") {
			currentType = strings.TrimSpace(strings.TrimPrefix(line, "Type:"))
		} else if strings.HasPrefix(line, "Speed:") {
			currentSpeed = strings.TrimSpace(strings.TrimPrefix(line, "Speed:"))
		} else if strings.HasPrefix(line, "Manufacturer:") {
			currentMfr = strings.TrimSpace(strings.TrimPrefix(line, "Manufacturer:"))
		}
	}
	// Commit last block
	if currentSize != "" && currentSize != "No Module Installed" {
		dimms = append(dimms, fmt.Sprintf("%s %s %s %s", currentSize, currentType, currentSpeed, currentMfr))
	}

	// Clean up unpopulated entries if any slipped through
	var final []string
	for _, dimm := range dimms {
		if !strings.Contains(dimm, "No Module Installed") && !strings.Contains(dimm, "Unknown") {
			final = append(final, strings.Join(strings.Fields(dimm), " ")) // normalizes whitespace
		}
	}

	return final
}
