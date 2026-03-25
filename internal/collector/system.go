package collector

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

type HostInfo struct {
	OSVendor         string   `json:"os_vendor"`
	OSVersion        string   `json:"os_version"`
	KernelVersion    string   `json:"kernel_version"`
	SystemVendor     string   `json:"system_vendor"`
	MotherboardName  string   `json:"motherboard_name"`
	ProductFamily    string   `json:"product_family,omitempty"`
	CPUs             []string `json:"cpus,omitempty"`
	GPUs             []string `json:"gpus,omitempty"`
	PhysicalRAM      []string `json:"physical_ram,omitempty"`
	DiskModels       []string `json:"disk_models,omitempty"`
}

func CollectHostInfo() HostInfo {
	info := HostInfo{}

	info.SystemVendor = readDMI("sys_vendor")
	if info.SystemVendor == "" {
		info.SystemVendor = readDMI("board_vendor")
	}
	info.MotherboardName = readDMI("board_name")
	if info.MotherboardName == "" {
		info.MotherboardName = readDMI("product_name")
	}
	info.ProductFamily = readDMI("product_family")

	// Kernel version via syscall.Uname
	var u syscall.Utsname
	if err := syscall.Uname(&u); err == nil {
		info.KernelVersion = charsToString(u.Release[:])
	}

	// OS Info via /etc/os-release
	if osFile, err := os.Open("/etc/os-release"); err == nil {
		defer osFile.Close()
		scanner := bufio.NewScanner(osFile)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				info.OSVendor = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"'`)
			} else if strings.HasPrefix(line, "NAME=") && info.OSVendor == "" {
				info.OSVendor = strings.Trim(strings.TrimPrefix(line, "NAME="), `"'`)
			} else if strings.HasPrefix(line, "VERSION=") {
				info.OSVersion = strings.Trim(strings.TrimPrefix(line, "VERSION="), `"'`)
			} else if strings.HasPrefix(line, "BUILD_ID=") && info.OSVersion == "" {
				info.OSVersion = strings.Trim(strings.TrimPrefix(line, "BUILD_ID="), `"'`)
			}
		}
	}

	// CPUs
	if cpuFile, err := os.Open("/proc/cpuinfo"); err == nil {
		defer cpuFile.Close()
		scanner := bufio.NewScanner(cpuFile)
		cpuSet := make(map[string]bool)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "model name") {
				parts := strings.Split(line, ":")
				if len(parts) == 2 {
					name := strings.TrimSpace(parts[1])
					if !cpuSet[name] {
						info.CPUs = append(info.CPUs, name)
						cpuSet[name] = true
					}
				}
			}
		}
	}

	// GPUs
	info.GPUs = GetAllGPUNames()

	// RAM
	info.PhysicalRAM = extractPhysicalRAM()
	if len(info.PhysicalRAM) == 0 {
		// Fallback to MemTotal
		if memFile, err := os.Open("/proc/meminfo"); err == nil {
			defer memFile.Close()
			scanner := bufio.NewScanner(memFile)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "MemTotal:") {
					parts := strings.Fields(line)
					if len(parts) >= 2 {
						kb, _ := strconv.ParseUint(parts[1], 10, 64)
						gb := float64(kb) / 1024.0 / 1024.0
						info.PhysicalRAM = append(info.PhysicalRAM, fmt.Sprintf("%.2f GiB Total", gb))
					}
					break
				}
			}
		}
	}

	// Disks
	importPath := "/sys/block"
	if dirs, err := os.ReadDir(importPath); err == nil {
		for _, d := range dirs {
			device := d.Name()
			if strings.HasPrefix(device, "loop") || strings.HasPrefix(device, "ram") || strings.HasPrefix(device, "dm-") || strings.HasPrefix(device, "sr") {
				continue
			}
			model := GetDiskModel(device)
			if model != "" {
				info.DiskModels = append(info.DiskModels, model)
			}
		}
	}

	return info
}

func readDMI(field string) string {
	f, err := os.Open("/sys/devices/virtual/dmi/id/" + field)
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	if scanner.Scan() {
		val := strings.TrimSpace(scanner.Text())
		// Filter out dummy values usually set by lazy BIOS vendors
		if val == "Default string" || val == "To be filled by O.E.M." || val == "System Product Name" {
			return ""
		}
		return val
	}
	return ""
}

func charsToString(chars []int8) string {
	b := make([]byte, 0, len(chars))
	for _, c := range chars {
		if c == 0 {
			break
		}
		b = append(b, byte(c))
	}
	return string(b)
}
