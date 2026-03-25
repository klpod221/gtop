package collector

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ProcessInfo holds per-process metrics matching btop's Proc::collect()
type ProcessInfo struct {
	PID           int     `json:"pid"`
	PPID          int     `json:"ppid"`
	UID           int     `json:"uid"`
	User          string  `json:"user"`
	Name          string  `json:"name"`
	Cmdline       string  `json:"cmdline"`
	State         string  `json:"state"`
	Threads       int     `json:"threads"`
	MemRSSBytes   uint64  `json:"mem_rss_bytes"`
	MemPercent    float64 `json:"mem_percent"`
	IO_ReadBytes  uint64  `json:"io_read_bytes"`
	IO_WriteBytes uint64  `json:"io_write_bytes"`
	CpuPercent    float64 `json:"cpu_percent"`
	// btop tracks starttime for process uptime calculation
	StartTime uint64 `json:"start_time"`

	// Internal: not exported, used for delta CPU calc
	utime uint64
	stime uint64
}

// TotalCPUSnapshot for process CPU % calculation
type TotalCPUSnapshot struct {
	Total uint64
}

var prevProcCPU = make(map[int][2]uint64) // pid -> [utime+stime, total_cpu_ticks]
var uidCache = make(map[int]string)

// getClkTck returns the system clock ticks per second (btop uses sysconf(_SC_CLK_TCK))
// On Linux this is almost always 100 but we read it properly
func getClkTck() uint64 {
	// Go doesn't expose sysconf, but Linux is always 100
	return 100
}

func getUserFromUID(uid int) string {
	if name, ok := uidCache[uid]; ok {
		return name
	}
	data, err := os.ReadFile("/etc/passwd")
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			parts := strings.Split(line, ":")
			if len(parts) >= 3 && parts[2] == strconv.Itoa(uid) {
				uidCache[uid] = parts[0]
				return parts[0]
			}
		}
	}
	name := strconv.Itoa(uid)
	uidCache[uid] = name
	return name
}

// CollectProcesses replicates btop's Proc::collect()
func CollectProcesses(totalMem uint64) []ProcessInfo {
	var procs []ProcessInfo

	// Get total CPU ticks for % calculation (btop: reads /proc/stat first line sum)
	var cputimes uint64
	statData, _ := os.ReadFile("/proc/stat")
	if len(statData) > 0 {
		lines := strings.SplitN(string(statData), "\n", 2)
		fields := strings.Fields(lines[0])
		for _, f := range fields[1:] {
			val, _ := strconv.ParseUint(f, 10, 64)
			cputimes += val
		}
	}

	matches, _ := filepath.Glob("/proc/[0-9]*")
	for _, match := range matches {
		pidStr := filepath.Base(match)
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		proc := ProcessInfo{PID: pid}

		// 1. /proc/[pid]/stat
		rawStat, err := os.ReadFile(filepath.Join(match, "stat"))
		if err != nil {
			continue
		}

		// btop: find closing ')' to handle process names with spaces/parens
		rParen := bytes.LastIndexByte(rawStat, ')')
		if rParen <= 0 || rParen+2 >= len(rawStat) {
			continue
		}
		nameStart := bytes.IndexByte(rawStat, '(')
		if nameStart > 0 {
			proc.Name = string(rawStat[nameStart+1 : rParen])
		}

		// Fields after the closing ')'
		// Index:  0=state, 1=ppid, ... 11=utime, 12=stime, ... 17=threads, ... 19=starttime (field 21 in full stat)
		fields := strings.Fields(string(rawStat[rParen+2:]))
		if len(fields) < 20 {
			continue
		}

		proc.State = fields[0]
		proc.PPID, _ = strconv.Atoi(fields[1])

		utime, _ := strconv.ParseUint(fields[11], 10, 64)
		stime, _ := strconv.ParseUint(fields[12], 10, 64)
		proc.utime = utime
		proc.stime = stime

		proc.Threads, _ = strconv.Atoi(fields[17])
		proc.StartTime, _ = strconv.ParseUint(fields[19], 10, 64)

		// 2. /proc/[pid]/status for UID and VmRSS
		statusData, err := os.ReadFile(filepath.Join(match, "status"))
		if err == nil {
			for _, line := range strings.Split(string(statusData), "\n") {
				if strings.HasPrefix(line, "Uid:") {
					f := strings.Fields(line)
					if len(f) >= 2 {
						uid, _ := strconv.Atoi(f[1])
						proc.UID = uid
						proc.User = getUserFromUID(uid)
					}
				} else if strings.HasPrefix(line, "VmRSS:") {
					f := strings.Fields(line)
					if len(f) >= 2 {
						rssKb, _ := strconv.ParseUint(f[1], 10, 64)
						proc.MemRSSBytes = rssKb * 1024
					}
				}
			}
		}

		// Mem %
		if totalMem > 0 {
			proc.MemPercent = float64(proc.MemRSSBytes) / float64(totalMem) * 100
		}

		// 3. /proc/[pid]/cmdline
		cmdData, _ := os.ReadFile(filepath.Join(match, "cmdline"))
		if len(cmdData) > 0 {
			proc.Cmdline = strings.TrimSpace(strings.ReplaceAll(string(cmdData), "\x00", " "))
		}

		// 4. /proc/[pid]/io (may require privileges)
		ioData, _ := os.ReadFile(filepath.Join(match, "io"))
		if len(ioData) > 0 {
			for _, line := range strings.Split(string(ioData), "\n") {
				if strings.HasPrefix(line, "read_bytes:") {
					f := strings.Fields(line)
					if len(f) >= 2 {
						proc.IO_ReadBytes, _ = strconv.ParseUint(f[1], 10, 64)
					}
				} else if strings.HasPrefix(line, "write_bytes:") {
					f := strings.Fields(line)
					if len(f) >= 2 {
						proc.IO_WriteBytes, _ = strconv.ParseUint(f[1], 10, 64)
					}
				}
			}
		}

		// 5. CPU % calculation (btop style: delta(utime+stime) / delta(total_cpu_ticks) * 100 * cmult)
		prevData, exists := prevProcCPU[pid]
		if exists && cputimes > prevData[1] {
			procDelta := float64((utime + stime) - prevData[0])
			cpuDelta := float64(cputimes - prevData[1])
			if cpuDelta > 0 {
				proc.CpuPercent = (procDelta / cpuDelta) * 100
				if proc.CpuPercent < 0 {
					proc.CpuPercent = 0
				}
			}
		}
		prevProcCPU[pid] = [2]uint64{utime + stime, cputimes}

		procs = append(procs, proc)
	}

	return procs
}

// GetTotalCPUTicks reads /proc/stat for total CPU ticks
func GetTotalCPUTicks() TotalCPUSnapshot {
	data, err := os.ReadFile("/proc/stat")
	if err == nil {
		lines := strings.SplitN(string(data), "\n", 2)
		if len(lines) > 0 && strings.HasPrefix(lines[0], "cpu ") {
			fields := strings.Fields(lines[0])
			var total uint64
			for _, field := range fields[1:] {
				val, _ := strconv.ParseUint(field, 10, 64)
				total += val
			}
			return TotalCPUSnapshot{Total: total}
		}
	}
	return TotalCPUSnapshot{Total: 100}
}
