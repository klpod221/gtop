package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"gtop/internal/collector"
	"gtop/internal/tui/widgets"

	"github.com/mum4k/termdash"
	"github.com/mum4k/termdash/container"
	"github.com/mum4k/termdash/keyboard"
	"github.com/mum4k/termdash/terminal/tcell"
	"github.com/mum4k/termdash/terminal/terminalapi"
)

// Run starts the TUI dashboard. Blocks until user quits (q/Ctrl+C).
func Run() error {
	// Initialize terminal
	t, err := tcell.New()
	if err != nil {
		return fmt.Errorf("tcell init: %w", err)
	}
	defer t.Close()

	// Create widgets
	cpuW, err := widgets.NewCPUWidget()
	if err != nil {
		return err
	}
	memW, err := widgets.NewMemWidget()
	if err != nil {
		return err
	}
	diskW, err := widgets.NewDiskWidget()
	if err != nil {
		return err
	}
	netW, err := widgets.NewNetWidget()
	if err != nil {
		return err
	}
	procW, err := widgets.NewProcWidget()
	if err != nil {
		return err
	}
	gpuW, err := widgets.NewGPUWidget()
	if err != nil {
		return err
	}

	// Initialize GPU collectors
	var intelCol *collector.IntelGPUCollector
	intelCol, err = collector.NewIntelGPUCollector()
	if err != nil {
		intelCol = nil
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

	// Initial reads for delta calculation
	collector.CollectCPUStats()
	if intelCol != nil {
		intelCol.Collect()
	}

	// Build layout (GPU panel always present)
	layoutOpts := BuildContainer(cpuW, memW, diskW, netW, procW, gpuW)
	c, err := container.New(t, layoutOpts...)
	if err != nil {
		return fmt.Errorf("container: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Data collection goroutine
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cpuStats, _ := collector.CollectCPUStats()
				cpuW.Update(cpuStats)

				memStats, _ := collector.CollectMem()
				memW.Update(memStats)

				disks := collector.CollectDisksSpace()
				diskW.Update(disks)

				netIfaces := collector.CollectNetwork()
				netW.Update(netIfaces)

				procs := collector.CollectProcesses(memStats.Total)
				procW.Update(procs)

				// GPU: update whichever is available
				if intelCol != nil {
					stats := intelCol.Collect()
					if len(stats.Engines) > 0 || stats.FreqActMHz > 0 {
						gpuW.UpdateIntel(&stats)
					}
				}
				nv, _ := collector.CollectNvidia()
				if len(nv) > 0 {
					gpuW.UpdateNvidia(nv)
				}
				amd := collector.CollectAmd()
				if len(amd) > 0 {
					gpuW.UpdateAmd(amd)
				}
			}
		}
	}()

	// Run termdash — blocks until quit
	return termdash.Run(ctx, t, c,
		termdash.KeyboardSubscriber(func(k *terminalapi.Keyboard) {
			switch k.Key {
			case keyboard.KeyCtrlC, 'q':
				cancel()
			}
		}),
		termdash.RedrawInterval(500*time.Millisecond),
	)
}
