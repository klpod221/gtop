package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"gtop/internal/collector"

	"github.com/spf13/cobra"
)

// AgentPayload contains all collected data
type AgentPayload struct {
	Timestamp  int64                      `json:"timestamp"`
	Host       *collector.HostInfo        `json:"host,omitempty"`
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

var (
	optModules    string
	optOutput     string
	optFormat     string
	optInterval   int
	optCount      int
	optCompact    bool
	optCpuFields  string
	optProcSort   string
	optProcTop    int
	optProcFilter string
	optNoProc     bool
	optNetIface   string
	optDiskMount  string
)

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get system metrics in JSON/Flat format",
	Long:  `Continuously or discretely fetches system metrics via CLI.`,
	Run: func(cmd *cobra.Command, args []string) {
		runGet()
	},
}

func init() {
	rootCmd.AddCommand(getCmd)

	getCmd.Flags().StringVar(&optModules, "modules", "all", "Modules to collect (csv): host,cpu,mem,disk,net,proc,gpu,all")
	getCmd.Flags().StringVar(&optOutput, "output", "", "Output file path (default: stdout)")
	getCmd.Flags().StringVar(&optFormat, "format", "json", "Output format: json, flat")
	getCmd.Flags().IntVar(&optInterval, "interval", 1000, "Collection interval in milliseconds")
	getCmd.Flags().IntVar(&optCount, "count", 1, "Number of collection cycles (0=infinite)")
	getCmd.Flags().BoolVar(&optCompact, "compact", false, "Compact JSON output (no indent)")

	getCmd.Flags().StringVar(&optCpuFields, "cpu-fields", "", "Filter CPU fields (csv): usage,freq,temp,power,loadavg,uptime,name,battery")
	getCmd.Flags().StringVar(&optProcSort, "proc-sort", "cpu", "Sort processes by: cpu, mem, pid, name, io")
	getCmd.Flags().IntVar(&optProcTop, "proc-top", 0, "Limit number of processes (0=all)")
	getCmd.Flags().StringVar(&optProcFilter, "proc-filter", "", "Filter processes by name substring")
	getCmd.Flags().BoolVar(&optNoProc, "no-proc", false, "Skip process collection entirely")

	getCmd.Flags().StringVar(&optNetIface, "net-iface", "", "Filter network interfaces (csv)")
	getCmd.Flags().StringVar(&optDiskMount, "disk-mount", "", "Filter disk mount points (csv)")
}

func runGet() {
	moduleSet := make(map[string]bool)
	if optModules == "all" {
		moduleSet["host"] = true
		moduleSet["cpu"] = true
		moduleSet["mem"] = true
		moduleSet["disk"] = true
		moduleSet["net"] = true
		moduleSet["proc"] = !optNoProc
		moduleSet["gpu"] = true
	} else {
		for _, m := range strings.Split(optModules, ",") {
			moduleSet[strings.TrimSpace(m)] = true
		}
	}
	if optNoProc {
		delete(moduleSet, "proc")
	}

	// Init GPU collectors
	var intelCol *collector.IntelGPUCollector
	if moduleSet["gpu"] {
		var err error
		intelCol, err = collector.NewIntelGPUCollector()
		if err != nil {
			// Only warn if an Intel GPU PMU exists but we lack permissions
			if !strings.Contains(err.Error(), "no Intel GPU PMU device found") &&
				!strings.Contains(err.Error(), "no GPU engines discovered") {
				fmt.Fprintf(os.Stderr, "Intel GPU: %v\n", err)
				fmt.Fprintf(os.Stderr, "  Hint: set cap_perfmon on binary: sudo setcap cap_perfmon,cap_dac_read_search=ep ./gtop\n")
			}
		}
		if intelCol != nil {
			defer intelCol.Close()
		}
		if err := collector.InitNvidia(); err != nil {
			if !strings.Contains(err.Error(), "LIBRARY_NOT_FOUND") {
				fmt.Fprintf(os.Stderr, "NVIDIA GPU: %v\n", err)
			}
		}
		defer collector.ShutdownNvidia()
	}

	if moduleSet["cpu"] {
		collector.CollectCPUStats()
	}
	if moduleSet["gpu"] && intelCol != nil {
		intelCol.Collect()
	}

	iteration := 0
	for {
		time.Sleep(time.Duration(optInterval) * time.Millisecond)
		iteration++

		payload := AgentPayload{
			Timestamp: time.Now().UnixMilli(),
		}

		// Host
		if moduleSet["host"] {
			hostInfo := collector.CollectHostInfo()
			payload.Host = &hostInfo
		}

		if moduleSet["cpu"] {
			cpuStats, _ := collector.CollectCPUStats()
			if optCpuFields != "" {
				cpuStats = filterCPUFields(cpuStats, optCpuFields)
			}
			payload.CPU = &cpuStats
		}
		if moduleSet["mem"] {
			memStats, _ := collector.CollectMem()
			payload.Memory = &memStats
		}
		if moduleSet["disk"] {
			payload.DisksSpace = collector.CollectDisksSpace()
			payload.DisksIO = collector.CollectDisksIO()
			if optDiskMount != "" {
				mounts := strings.Split(optDiskMount, ",")
				payload.DisksSpace = filterDiskMounts(payload.DisksSpace, mounts)
			}
		}
		if moduleSet["net"] {
			payload.Network = collector.CollectNetwork()
			if optNetIface != "" {
				ifaces := strings.Split(optNetIface, ",")
				payload.Network = filterNetIfaces(payload.Network, ifaces)
			}
		}
		if moduleSet["proc"] {
			var totalMem uint64
			if payload.Memory != nil {
				totalMem = payload.Memory.Total
			} else {
				mem, _ := collector.CollectMem()
				totalMem = mem.Total
			}
			procs := collector.CollectProcesses(totalMem)
			if optProcFilter != "" {
				procs = filterProcesses(procs, optProcFilter)
			}
			sortProcesses(procs, optProcSort)
			if optProcTop > 0 && len(procs) > optProcTop {
				procs = procs[:optProcTop]
			}
			payload.Processes = procs
		}
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

		var jsonData []byte
		if optCompact {
			jsonData, _ = json.Marshal(payload)
		} else {
			jsonData, _ = json.MarshalIndent(payload, "", "  ")
		}

		if optFormat == "flat" {
			jsonData = flattenJSON(jsonData)
		}

		if optOutput != "" {
			os.WriteFile(optOutput, jsonData, 0644)
			fmt.Fprintf(os.Stderr, "[%d] Wrote %d bytes to %s\n", iteration, len(jsonData), optOutput)
		} else {
			fmt.Println(string(jsonData))
		}

		if optCount > 0 && iteration >= optCount {
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
		sort.Slice(procs, func(i, j int) bool { return procs[i].CpuPercent > procs[j].CpuPercent })
	case "mem":
		sort.Slice(procs, func(i, j int) bool { return procs[i].MemRSSBytes > procs[j].MemRSSBytes })
	case "pid":
		sort.Slice(procs, func(i, j int) bool { return procs[i].PID < procs[j].PID })
	case "name":
		sort.Slice(procs, func(i, j int) bool { return strings.ToLower(procs[i].Name) < strings.ToLower(procs[j].Name) })
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
