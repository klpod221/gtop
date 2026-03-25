package collector

import (
	"bufio"
	"bytes"
	"os/exec"
	"strings"
	"sync"
)

var (
	gpuNamesOnce sync.Once
	intelGPUName string
	amdGPUName   string
	allGPUNames  []string
)

// DetectGPUNames runs lspci to find the hardware names for GPUs.
func DetectGPUNames() {
	gpuNamesOnce.Do(func() {
		cmd := exec.Command("lspci")
		out, err := cmd.Output()
		if err != nil {
			return
		}

		scanner := bufio.NewScanner(bytes.NewReader(out))
		for scanner.Scan() {
			line := scanner.Text()
			// Match VGA or 3D controller
			if strings.Contains(line, "VGA compatible controller:") || strings.Contains(line, "3D controller:") {
				parts := strings.SplitN(line, ": ", 2)
				if len(parts) == 2 {
					name := strings.TrimSpace(parts[1])
					// Optional: remove trailing "(rev XX)"
					if idx := strings.LastIndex(name, " (rev "); idx != -1 {
						name = strings.TrimSpace(name[:idx])
					}

					allGPUNames = append(allGPUNames, name)

					if strings.Contains(line, "Intel") {
						intelGPUName = name
					} else if strings.Contains(line, "AMD") || strings.Contains(line, "Advanced Micro Devices") || strings.Contains(name, "Radeon") {
						amdGPUName = name
					}
				}
			}
		}

		if intelGPUName == "" {
			intelGPUName = "Intel GPU"
		}
		if amdGPUName == "" {
			amdGPUName = "AMD GPU"
		}
	})
}

// GetIntelGPUName returns the cached Intel GPU string
func GetIntelGPUName() string {
	DetectGPUNames()
	return intelGPUName
}

// GetAmdGPUName returns the cached AMD GPU string
func GetAmdGPUName() string {
	DetectGPUNames()
	return amdGPUName
}

// GetAllGPUNames returns all graphics cards on the system
func GetAllGPUNames() []string {
	DetectGPUNames()
	return allGPUNames
}
