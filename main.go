package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"gtop/collector"
	"gtop/tui"
)

// AgentPayload contains all collected data
type AgentPayload struct {
	Timestamp  int64                      `json:"timestamp"`
	CPU        *collector.CPUStats        `json:"cpu,omitempty"`
	Memory     *collector.MemStats        `json:"memory,omitempty"`
	DisksSpace []collector.DiskSpace      `json:"disks_space,omitempty"`
	DisksIO    []collector.DiskIO         `json:"disks_io,omitempty"`
	Network    []collector.NetInterface   `json:"network,omitempty"`
	Processes  []collector.ProcessInfo    `json:"processes,omitempty"`
	IntelGPU   *collector.IntelGPUStats   `json:"intel_gpu,omitempty"`
	NvidiaGPUs []collector.NvidiaGPUStats `json:"nvidia_gpus,omitempty"`
	AmdGPUs    []collector.AmdGPUStats    `json:"amd_gpus,omitempty"`
}

func printUsage() {
	banner := `
   ██████╗ ████████╗ ██████╗ ██████╗
  ██╔════╝ ╚══██╔══╝██╔═══██╗██╔══██╗
  ██║  ███╗   ██║   ██║   ██║██████╔╝
  ██║   ██║   ██║   ██║   ██║██╔═══╝
  ╚██████╔╝   ██║   ╚██████╔╝██║
   ╚═════╝    ╚═╝    ╚═════╝ ╚═╝
  Go Based Linux System Monitor by klpod221
`
	fmt.Fprint(os.Stderr, banner)
	fmt.Fprintf(os.Stderr, "\nUsage: gtop [flags]\n\n")
	fmt.Fprintf(os.Stderr, "Modes:\n")
	fmt.Fprintf(os.Stderr, "  --tui              Launch interactive TUI dashboard (btop-style)\n")
	fmt.Fprintf(os.Stderr, "                     Without --tui, runs in CLI mode (JSON output)\n\n")
	fmt.Fprintf(os.Stderr, "Output (CLI mode):\n")
	fmt.Fprintf(os.Stderr, "  --modules string   Modules to collect: cpu,mem,disk,net,proc,gpu,all (default \"all\")\n")
	fmt.Fprintf(os.Stderr, "  --output string    Write to file instead of stdout\n")
	fmt.Fprintf(os.Stderr, "  --format string    Output format: json, flat (default \"json\")\n")
	fmt.Fprintf(os.Stderr, "  --compact          Single-line JSON output\n")
	fmt.Fprintf(os.Stderr, "  --interval int     Collection interval in ms (default 1000)\n")
	fmt.Fprintf(os.Stderr, "  --count int        Collection cycles, 0=infinite (default 1)\n\n")
	fmt.Fprintf(os.Stderr, "Filters:\n")
	fmt.Fprintf(os.Stderr, "  --cpu-fields str   Filter CPU fields: usage,freq,temp,power,loadavg,uptime,name,battery\n")
	fmt.Fprintf(os.Stderr, "  --proc-sort str    Sort processes by: cpu, mem, pid, name, io (default \"cpu\")\n")
	fmt.Fprintf(os.Stderr, "  --proc-top int     Limit processes shown, 0=all (default 0)\n")
	fmt.Fprintf(os.Stderr, "  --proc-filter str  Filter processes by name substring\n")
	fmt.Fprintf(os.Stderr, "  --no-proc          Skip process collection\n")
	fmt.Fprintf(os.Stderr, "  --net-iface str    Filter network interfaces (csv)\n")
	fmt.Fprintf(os.Stderr, "  --disk-mount str   Filter disk mount points (csv)\n\n")
	fmt.Fprintf(os.Stderr, "Examples:\n")
	fmt.Fprintf(os.Stderr, "  gtop --tui                     # Interactive dashboard\n")
	fmt.Fprintf(os.Stderr, "  gtop --modules cpu,mem          # CPU + Memory JSON\n")
	fmt.Fprintf(os.Stderr, "  gtop --compact --count 0        # Continuous compact JSON\n")
	fmt.Fprintf(os.Stderr, "  gtop --proc-sort mem --proc-top 10\n\n")
}

func main() {
	// Custom help
	flag.Usage = printUsage

	// TUI mode flag
	tuiMode := flag.Bool("tui", false, "Launch interactive TUI dashboard (btop-style)")

	// CLI Flags
	modules := flag.String("modules", "all", "Modules to collect (csv): cpu,mem,disk,net,proc,gpu,all")
	output := flag.String("output", "", "Output file path (default: stdout)")
	format := flag.String("format", "json", "Output format: json, flat")
	interval := flag.Int("interval", 1000, "Collection interval in milliseconds")
	count := flag.Int("count", 1, "Number of collection cycles (0=infinite)")
	compact := flag.Bool("compact", false, "Compact JSON output (no indent)")

	// CPU flags
	cpuFields := flag.String("cpu-fields", "", "Filter CPU fields (csv): usage,freq,temp,power,loadavg,uptime,name,battery")

	// Process flags
	procSort := flag.String("proc-sort", "cpu", "Sort processes by: cpu, mem, pid, name, io")
	procTop := flag.Int("proc-top", 0, "Limit number of processes (0=all)")
	procFilter := flag.String("proc-filter", "", "Filter processes by name substring")
	noProc := flag.Bool("no-proc", false, "Skip process collection entirely")

	// Network flags
	netIface := flag.String("net-iface", "", "Filter network interfaces (csv)")

	// Disk flags
	diskMount := flag.String("disk-mount", "", "Filter disk mount points (csv)")

	flag.Parse()

	// Launch TUI if requested
	if *tuiMode {
		if err := tui.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Parse modules
	moduleSet := make(map[string]bool)
	if *modules == "all" {
		for _, m := range []string{"cpu", "mem", "disk", "net", "proc", "gpu"} {
			moduleSet[m] = true
		}
	} else {
		for _, m := range strings.Split(*modules, ",") {
			moduleSet[strings.TrimSpace(m)] = true
		}
	}
	if *noProc {
		delete(moduleSet, "proc")
	}

	// Init GPU collectors
	var intelCol *collector.IntelGPUCollector
	if moduleSet["gpu"] {
		var err error
		intelCol, err = collector.NewIntelGPUCollector()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Intel GPU: %v\n", err)
			fmt.Fprintf(os.Stderr, "  Hint: set cap_perfmon on binary: sudo setcap cap_perfmon,cap_dac_read_search=ep ./gtop\n")
		}
		if intelCol != nil {
			defer intelCol.Close()
		}
		if err := collector.InitNvidia(); err != nil {
			// Silence "library not found" — means no NVIDIA GPU, which is normal
			if !strings.Contains(err.Error(), "LIBRARY_NOT_FOUND") {
				fmt.Fprintf(os.Stderr, "NVIDIA GPU: %v\n", err)
			}
		}
		defer collector.ShutdownNvidia()
	}

	// Initial CPU read (for delta calculation)
	if moduleSet["cpu"] {
		collector.CollectCPUStats()
	}

	// Initial GPU read (for delta calculation, same as CPU)
	if moduleSet["gpu"] && intelCol != nil {
		intelCol.Collect()
	}

	iteration := 0
	for {
		time.Sleep(time.Duration(*interval) * time.Millisecond)
		iteration++

		payload := AgentPayload{
			Timestamp: time.Now().UnixMilli(),
		}

		// CPU
		if moduleSet["cpu"] {
			cpuStats, _ := collector.CollectCPUStats()
			if *cpuFields != "" {
				cpuStats = filterCPUFields(cpuStats, *cpuFields)
			}
			payload.CPU = &cpuStats
		}

		// Memory
		if moduleSet["mem"] {
			memStats, _ := collector.CollectMem()
			payload.Memory = &memStats
		}

		// Disk
		if moduleSet["disk"] {
			payload.DisksSpace = collector.CollectDisksSpace()
			payload.DisksIO = collector.CollectDisksIO()

			if *diskMount != "" {
				mounts := strings.Split(*diskMount, ",")
				payload.DisksSpace = filterDiskMounts(payload.DisksSpace, mounts)
			}
		}

		// Network
		if moduleSet["net"] {
			payload.Network = collector.CollectNetwork()
			if *netIface != "" {
				ifaces := strings.Split(*netIface, ",")
				payload.Network = filterNetIfaces(payload.Network, ifaces)
			}
		}

		// Processes
		if moduleSet["proc"] {
			var totalMem uint64
			if payload.Memory != nil {
				totalMem = payload.Memory.Total
			} else {
				mem, _ := collector.CollectMem()
				totalMem = mem.Total
			}
			procs := collector.CollectProcesses(totalMem)

			// Filter
			if *procFilter != "" {
				procs = filterProcesses(procs, *procFilter)
			}

			// Sort
			sortProcesses(procs, *procSort)

			// Top N
			if *procTop > 0 && len(procs) > *procTop {
				procs = procs[:*procTop]
			}

			payload.Processes = procs
		}

		// GPU
		if moduleSet["gpu"] {
			if intelCol != nil {
				stats := intelCol.Collect()
				if len(stats.Engines) > 0 || stats.FreqActMHz > 0 {
					payload.IntelGPU = &stats
				}
			}
			nv, _ := collector.CollectNvidia()
			if len(nv) > 0 {
				payload.NvidiaGPUs = nv
			}
			amd := collector.CollectAmd()
			if len(amd) > 0 {
				payload.AmdGPUs = amd
			}
		}

		// Serialize
		var jsonData []byte
		if *compact {
			jsonData, _ = json.Marshal(payload)
		} else {
			jsonData, _ = json.MarshalIndent(payload, "", "  ")
		}

		// Output
		if *format == "flat" {
			jsonData = flattenJSON(jsonData)
		}

		if *output != "" {
			os.WriteFile(*output, jsonData, 0644)
			fmt.Fprintf(os.Stderr, "[%d] Wrote %d bytes to %s\n", iteration, len(jsonData), *output)
		} else {
			fmt.Println(string(jsonData))
		}

		if *count > 0 && iteration >= *count {
			break
		}
	}
}

func filterCPUFields(stats collector.CPUStats, fields string) collector.CPUStats {
	fieldSet := make(map[string]bool)
	for _, f := range strings.Split(fields, ",") {
		fieldSet[strings.TrimSpace(f)] = true
	}

	var filtered collector.CPUStats
	if fieldSet["usage"] {
		filtered.UsagePercent = stats.UsagePercent
		filtered.CoresPercent = stats.CoresPercent
	}
	if fieldSet["freq"] {
		filtered.FreqMHz = stats.FreqMHz
	}
	if fieldSet["temp"] {
		filtered.CoreTempsC = stats.CoreTempsC
		filtered.PackageTempC = stats.PackageTempC
	}
	if fieldSet["power"] {
		filtered.PowerWatts = stats.PowerWatts
	}
	if fieldSet["loadavg"] {
		filtered.LoadAvg = stats.LoadAvg
	}
	if fieldSet["uptime"] {
		filtered.UptimeSeconds = stats.UptimeSeconds
	}
	if fieldSet["name"] {
		filtered.CpuName = stats.CpuName
	}
	if fieldSet["battery"] {
		filtered.BatteryPercent = stats.BatteryPercent
		filtered.BatteryStatus = stats.BatteryStatus
	}
	return filtered
}

func filterDiskMounts(disks []collector.DiskSpace, mounts []string) []collector.DiskSpace {
	var filtered []collector.DiskSpace
	for _, d := range disks {
		for _, m := range mounts {
			if strings.TrimSpace(m) == d.MountPoint {
				filtered = append(filtered, d)
				break
			}
		}
	}
	return filtered
}

func filterNetIfaces(ifaces []collector.NetInterface, names []string) []collector.NetInterface {
	var filtered []collector.NetInterface
	for _, iface := range ifaces {
		for _, name := range names {
			if strings.TrimSpace(name) == iface.Name {
				filtered = append(filtered, iface)
				break
			}
		}
	}
	return filtered
}

func filterProcesses(procs []collector.ProcessInfo, filter string) []collector.ProcessInfo {
	var filtered []collector.ProcessInfo
	filterLower := strings.ToLower(filter)
	for _, p := range procs {
		if strings.Contains(strings.ToLower(p.Name), filterLower) ||
			strings.Contains(strings.ToLower(p.Cmdline), filterLower) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func sortProcesses(procs []collector.ProcessInfo, sortBy string) {
	switch sortBy {
	case "cpu":
		sort.Slice(procs, func(i, j int) bool {
			return procs[i].CpuPercent > procs[j].CpuPercent
		})
	case "mem":
		sort.Slice(procs, func(i, j int) bool {
			return procs[i].MemRSSBytes > procs[j].MemRSSBytes
		})
	case "pid":
		sort.Slice(procs, func(i, j int) bool {
			return procs[i].PID < procs[j].PID
		})
	case "name":
		sort.Slice(procs, func(i, j int) bool {
			return strings.ToLower(procs[i].Name) < strings.ToLower(procs[j].Name)
		})
	case "io":
		sort.Slice(procs, func(i, j int) bool {
			return (procs[i].IO_ReadBytes + procs[i].IO_WriteBytes) > (procs[j].IO_ReadBytes + procs[j].IO_WriteBytes)
		})
	}
}

func flattenJSON(data []byte) []byte {
	var m map[string]interface{}
	json.Unmarshal(data, &m)
	flat := make(map[string]interface{})
	flatten("", m, flat)
	result, _ := json.MarshalIndent(flat, "", "  ")
	return result
}

func flatten(prefix string, src map[string]interface{}, dst map[string]interface{}) {
	for k, v := range src {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch val := v.(type) {
		case map[string]interface{}:
			flatten(key, val, dst)
		default:
			dst[key] = val
		}
	}
}
