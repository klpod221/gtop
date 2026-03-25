package widgets

import (
	"fmt"
	"sync"

	"gtop/collector"

	"github.com/mum4k/termdash/cell"
	"github.com/mum4k/termdash/widgets/text"
)

// GPUWidget renders GPU information for Intel, NVIDIA, or AMD GPUs.
type GPUWidget struct {
	mu      sync.Mutex
	Text    *text.Text
	Visible bool
}

// NewGPUWidget creates the GPU info widget.
func NewGPUWidget() (*GPUWidget, error) {
	t, err := text.New(text.WrapAtRunes())
	if err != nil {
		return nil, fmt.Errorf("gpu text: %w", err)
	}
	return &GPUWidget{Text: t}, nil
}

// UpdateIntel refreshes with Intel GPU stats.
func (w *GPUWidget) UpdateIntel(stats *collector.IntelGPUStats) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if stats == nil {
		return
	}
	w.Visible = true
	w.Text.Reset()

	w.Text.Write(" Intel GPU\n", text.WriteCellOpts(cell.FgColor(cell.ColorNumber(135)), cell.Bold()))

	// Frequency
	w.Text.Write(
		fmt.Sprintf(" Freq: %.0f MHz (req %.0f MHz)\n", stats.FreqActMHz, stats.FreqReqMHz),
		text.WriteCellOpts(cell.FgColor(cell.ColorNumber(250))),
	)

	// RC6 residency
	w.Text.Write(
		fmt.Sprintf(" RC6:  %.1f%%\n", stats.RC6Pct),
		text.WriteCellOpts(cell.FgColor(cell.ColorNumber(250))),
	)

	// Power
	if stats.PowerGPUWatts > 0 || stats.PowerPkgWatts > 0 {
		w.Text.Write(
			fmt.Sprintf(" Power: GPU %.1fW  Pkg %.1fW\n", stats.PowerGPUWatts, stats.PowerPkgWatts),
			text.WriteCellOpts(cell.FgColor(cell.ColorNumber(250))),
		)
	}

	// Engine utilization
	for _, eng := range stats.Engines {
		pct := int(eng.BusyPct)
		color := barColorForPct(pct)
		w.Text.Write(
			fmt.Sprintf(" %-10s ", eng.Name),
			text.WriteCellOpts(cell.FgColor(cell.ColorNumber(240))),
		)
		w.Text.Write(
			makeBar(10, eng.BusyPct, "█", "░"),
			text.WriteCellOpts(cell.FgColor(color)),
		)
		w.Text.Write(
			fmt.Sprintf(" %3d%%\n", pct),
			text.WriteCellOpts(cell.FgColor(cell.ColorNumber(250))),
		)
	}
}

// UpdateNvidia refreshes with NVIDIA GPU stats.
func (w *GPUWidget) UpdateNvidia(stats []collector.NvidiaGPUStats) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(stats) == 0 {
		return
	}
	w.Visible = true
	w.Text.Reset()

	for _, gpu := range stats {
		w.Text.Write(
			fmt.Sprintf(" %s\n", gpu.Name),
			text.WriteCellOpts(cell.FgColor(cell.ColorNumber(135)), cell.Bold()),
		)

		// Utilization bar
		pct := int(gpu.UtilizationGPU)
		w.Text.Write(" GPU  ", text.WriteCellOpts(cell.FgColor(cell.ColorNumber(240))))
		w.Text.Write(
			makeBar(10, float64(gpu.UtilizationGPU), "█", "░"),
			text.WriteCellOpts(cell.FgColor(barColorForPct(pct))),
		)
		w.Text.Write(fmt.Sprintf(" %3d%%\n", pct),
			text.WriteCellOpts(cell.FgColor(cell.ColorNumber(250))))

		// VRAM
		if gpu.VRAMTotal > 0 {
			vramPct := float64(gpu.VRAMUsed) / float64(gpu.VRAMTotal) * 100
			w.Text.Write(" VRAM ", text.WriteCellOpts(cell.FgColor(cell.ColorNumber(240))))
			w.Text.Write(
				makeBar(10, vramPct, "█", "░"),
				text.WriteCellOpts(cell.FgColor(barColorForPct(int(vramPct)))),
			)
			w.Text.Write(
				fmt.Sprintf(" %s/%s\n", humanBytes(gpu.VRAMUsed), humanBytes(gpu.VRAMTotal)),
				text.WriteCellOpts(cell.FgColor(cell.ColorNumber(250))),
			)
		}

		// Temp, Power, Clocks
		w.Text.Write(
			fmt.Sprintf(" %d°C  %.1fW/%.0fW  Core %dMHz  Mem %dMHz\n",
				gpu.TempC, gpu.PowerWatts, gpu.PowerLimitWatts,
				gpu.ClockCoreMHz, gpu.ClockMemMHz),
			text.WriteCellOpts(cell.FgColor(cell.ColorNumber(250))),
		)
	}
}

// UpdateAmd refreshes with AMD GPU stats.
func (w *GPUWidget) UpdateAmd(stats []collector.AmdGPUStats) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(stats) == 0 {
		return
	}
	w.Visible = true
	w.Text.Reset()

	for _, gpu := range stats {
		w.Text.Write(
			fmt.Sprintf(" %s\n", gpu.Name),
			text.WriteCellOpts(cell.FgColor(cell.ColorNumber(135)), cell.Bold()),
		)

		pct := int(gpu.UtilizationGPU)
		w.Text.Write(" GPU  ", text.WriteCellOpts(cell.FgColor(cell.ColorNumber(240))))
		w.Text.Write(
			makeBar(10, float64(gpu.UtilizationGPU), "█", "░"),
			text.WriteCellOpts(cell.FgColor(barColorForPct(pct))),
		)
		w.Text.Write(fmt.Sprintf(" %3d%%\n", pct),
			text.WriteCellOpts(cell.FgColor(cell.ColorNumber(250))))

		if gpu.VRAMTotal > 0 {
			vramPct := float64(gpu.VRAMUsed) / float64(gpu.VRAMTotal) * 100
			w.Text.Write(" VRAM ", text.WriteCellOpts(cell.FgColor(cell.ColorNumber(240))))
			w.Text.Write(
				makeBar(10, vramPct, "█", "░"),
				text.WriteCellOpts(cell.FgColor(barColorForPct(int(vramPct)))),
			)
			w.Text.Write(
				fmt.Sprintf(" %s/%s\n", humanBytes(gpu.VRAMUsed), humanBytes(gpu.VRAMTotal)),
				text.WriteCellOpts(cell.FgColor(cell.ColorNumber(250))),
			)
		}

		w.Text.Write(
			fmt.Sprintf(" %d°C  %.1fW  Core %dMHz  Mem %dMHz\n",
				gpu.TempC, gpu.PowerWatts, gpu.ClockCoreMHz, gpu.ClockMemMHz),
			text.WriteCellOpts(cell.FgColor(cell.ColorNumber(250))),
		)
	}
}
