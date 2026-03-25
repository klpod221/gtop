package collector

import (
	"fmt"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

type NvidiaProcess struct {
	PID      uint32
	UsedVRAM uint64
}

type NvidiaGPUStats struct {
	Name            string
	UtilizationGPU  uint32
	UtilizationMem  uint32
	TempC           uint32
	PowerWatts      float64
	PowerLimitWatts float64
	ClockCoreMHz    uint32
	ClockMemMHz     uint32
	VRAMTotal       uint64
	VRAMUsed        uint64
	VRAMFree        uint64
	PCIeTxKBs       uint32
	PCIeRxKBs       uint32
	Processes       []NvidiaProcess
}

var nvmlInitialized bool

func InitNvidia() error {
	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		return fmt.Errorf("NVML init failed: %v", nvml.ErrorString(ret))
	}
	nvmlInitialized = true
	return nil
}

func ShutdownNvidia() {
	if nvmlInitialized {
		nvml.Shutdown()
		nvmlInitialized = false
	}
}

func CollectNvidia() ([]NvidiaGPUStats, error) {
	if !nvmlInitialized {
		return nil, fmt.Errorf("NVML not initialized")
	}

	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("failed to get device count: %v", nvml.ErrorString(ret))
	}

	var statsList []NvidiaGPUStats

	for i := 0; i < count; i++ {
		device, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			continue
		}

		var stats NvidiaGPUStats

		// Name
		name, _ := device.GetName()
		stats.Name = name

		// Utilization
		util, ret := device.GetUtilizationRates()
		if ret == nvml.SUCCESS {
			stats.UtilizationGPU = util.Gpu
			stats.UtilizationMem = util.Memory
		}

		// Temperature
		temp, ret := device.GetTemperature(nvml.TEMPERATURE_GPU)
		if ret == nvml.SUCCESS {
			stats.TempC = temp
		}

		// Power
		power, ret := device.GetPowerUsage()
		if ret == nvml.SUCCESS {
			stats.PowerWatts = float64(power) / 1000.0
		}
		limit, ret := device.GetPowerManagementLimit()
		if ret == nvml.SUCCESS {
			stats.PowerLimitWatts = float64(limit) / 1000.0
		}

		// Clocks
		clockCore, _ := device.GetClockInfo(nvml.CLOCK_GRAPHICS)
		stats.ClockCoreMHz = clockCore
		clockMem, _ := device.GetClockInfo(nvml.CLOCK_MEM)
		stats.ClockMemMHz = clockMem

		// Memory
		memInfo, ret := device.GetMemoryInfo()
		if ret == nvml.SUCCESS {
			stats.VRAMTotal = memInfo.Total
			stats.VRAMUsed = memInfo.Used
			stats.VRAMFree = memInfo.Free
		}

		// PCIe Throughput
		tx, _ := device.GetPcieThroughput(nvml.PCIE_UTIL_TX_BYTES)
		stats.PCIeTxKBs = tx
		rx, _ := device.GetPcieThroughput(nvml.PCIE_UTIL_RX_BYTES)
		stats.PCIeRxKBs = rx

		// Processes
		procs, ret := device.GetComputeRunningProcesses()
		if ret == nvml.SUCCESS {
			for _, p := range procs {
				stats.Processes = append(stats.Processes, NvidiaProcess{
					PID:      p.Pid,
					UsedVRAM: p.UsedGpuMemory,
				})
			}
		}

		statsList = append(statsList, stats)
	}

	return statsList, nil
}
