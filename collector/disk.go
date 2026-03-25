package collector

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// DiskSpace holds mount point usage info
type DiskSpace struct {
	MountPoint string  `json:"mount_point"`
	Device     string  `json:"device"`
	FsType     string  `json:"fs_type"`
	Name       string  `json:"name"`
	TotalBytes uint64  `json:"total_bytes"`
	UsedBytes  uint64  `json:"used_bytes"`
	FreeBytes  uint64  `json:"free_bytes"`
	UsedPct    float64 `json:"used_pct"`
}

// DiskIO holds IO statistics from /sys/block/*/stat
type DiskIO struct {
	Device     string `json:"device"`
	ReadBytes  uint64 `json:"read_bytes"`
	WriteBytes uint64 `json:"write_bytes"`
	ReadIOPS   uint64 `json:"read_iops"`
	WriteIOPS  uint64 `json:"write_iops"`
	IOTicksMs  uint64 `json:"io_ticks_ms"`
}

// convertOctalEscapes replicates btop's convert_ascii_escapes() for mount paths
// e.g. \040 -> space
func convertOctalEscapes(input string) string {
	var out strings.Builder
	out.Grow(len(input))
	for i := 0; i < len(input); i++ {
		if input[i] == '\\' && i+3 < len(input) &&
			input[i+1] >= '0' && input[i+1] <= '7' &&
			input[i+2] >= '0' && input[i+2] <= '7' &&
			input[i+3] >= '0' && input[i+3] <= '7' {
			val := (int(input[i+1]-'0') * 64) + (int(input[i+2]-'0') * 8) + int(input[i+3]-'0')
			out.WriteByte(byte(val))
			i += 3
		} else {
			out.WriteByte(input[i])
		}
	}
	return out.String()
}

// getPhysicalFSTypes reads /proc/filesystems to get list of real (non-nodev) filesystem types
// Btop does this to filter only physical mounts
func getPhysicalFSTypes() map[string]bool {
	fstypes := map[string]bool{
		"zfs":   true,
		"wslfs": true,
		"drvfs": true,
	}
	file, err := os.Open("/proc/filesystems")
	if err != nil {
		return fstypes
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) == 1 {
			// Lines without "nodev" prefix are physical
			fs := fields[0]
			if fs != "squashfs" && fs != "nullfs" {
				fstypes[fs] = true
			}
		}
		// Lines starting with "nodev" are virtual -> skip
	}
	return fstypes
}

// CollectDisksSpace reads mount list and gets capacity via unix.Statfs
// btop reads from /etc/mtab (if exists) or /proc/self/mounts
func CollectDisksSpace() []DiskSpace {
	var results []DiskSpace
	physFS := getPhysicalFSTypes()

	// btop: open /etc/mtab if exists, else /proc/self/mounts
	mountSrc := "/proc/self/mounts"
	if _, err := os.Stat("/etc/mtab"); err == nil {
		mountSrc = "/etc/mtab"
	}

	file, err := os.Open(mountSrc)
	if err != nil {
		return results
	}
	defer file.Close()

	seen := make(map[string]bool)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		device := fields[0]
		mount := convertOctalEscapes(fields[1])
		fsType := fields[2]

		if seen[mount] {
			continue
		}

		// btop: only include physical filesystem types
		if !physFS[fsType] {
			continue
		}

		seen[mount] = true

		var stat unix.Statfs_t
		if err := unix.Statfs(mount, &stat); err == nil {
			total := stat.Blocks * uint64(stat.Bsize)
			free := stat.Bavail * uint64(stat.Bsize)
			used := total - free

			var pct float64
			if total > 0 {
				pct = (float64(used) / float64(total)) * 100
			}

			name := filepath.Base(mount)
			if mount == "/" {
				name = "root"
			}

			results = append(results, DiskSpace{
				MountPoint: mount,
				Device:     device,
				FsType:     fsType,
				Name:       name,
				TotalBytes: total,
				FreeBytes:  free,
				UsedBytes:  used,
				UsedPct:    pct,
			})
		}
	}
	return results
}

// CollectDisksIO reads /sys/block/*/stat for IO statistics
// btop: finds the right stat file by stripping trailing chars from device name
func CollectDisksIO() []DiskIO {
	var results []DiskIO

	paths, _ := filepath.Glob("/sys/block/*/stat")
	for _, p := range paths {
		device := filepath.Base(filepath.Dir(p))
		if strings.HasPrefix(device, "loop") || strings.HasPrefix(device, "ram") || strings.HasPrefix(device, "dm-") {
			continue
		}

		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}

		fields := strings.Fields(string(data))
		if len(fields) < 11 {
			continue
		}

		// Kernel doc /sys/block/<dev>/stat:
		// [0]read_ios [1]read_merges [2]read_sectors [3]read_ticks
		// [4]write_ios [5]write_merges [6]write_sectors [7]write_ticks
		// [8]in_flight [9]io_ticks [10]time_in_queue
		rIOPS, _ := strconv.ParseUint(fields[0], 10, 64)
		rSectors, _ := strconv.ParseUint(fields[2], 10, 64)
		wIOPS, _ := strconv.ParseUint(fields[4], 10, 64)
		wSectors, _ := strconv.ParseUint(fields[6], 10, 64)
		ioTicks, _ := strconv.ParseUint(fields[9], 10, 64)

		results = append(results, DiskIO{
			Device:     device,
			ReadIOPS:   rIOPS,
			WriteIOPS:  wIOPS,
			ReadBytes:  rSectors * 512,
			WriteBytes: wSectors * 512,
			IOTicksMs:  ioTicks,
		})
	}
	return results
}
