package collector

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// MemStats holds exhaustive memory metrics matching btop's Mem::collect()
type MemStats struct {
	Total     uint64 `json:"total"`
	Available uint64 `json:"available"`
	Used      uint64 `json:"used"`
	Free      uint64 `json:"free"`
	Cached    uint64 `json:"cached"`
	Buffers   uint64 `json:"buffers"`
	SwapTotal uint64 `json:"swap_total"`
	SwapFree  uint64 `json:"swap_free"`
	SwapUsed  uint64 `json:"swap_used"`
	ZFSArc    uint64 `json:"zfs_arc,omitempty"`
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

	return stats, nil
}
