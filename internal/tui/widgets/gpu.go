package widgets

import (
	"fmt"
	"sync"

	"gtop/internal/collector"

	"github.com/mum4k/termdash/cell"
	"github.com/mum4k/termdash/widgets/text"
)

// GPUWidget renders GPU information for Intel, NVIDIA, and/or AMD GPUs simultaneously.
type GPUWidget struct {
	mu      sync.Mutex
	Text    *text.Text
	Visible bool

	// Cached stats for combined rendering
	intelStats  *collector.IntelGPUStats
	nvidiaStats []collector.NvidiaGPUStats
	amdStats    []collector.AmdGPUStats
}

// NewGPUWidget creates the GPU info widget.
func NewGPUWidget() (*GPUWidget, error) {
	t, err := text.New(text.WrapAtRunes())
	if err != nil {
		return nil, fmt.Errorf("gpu text: %w", err)
	}
	return &GPUWidget{Text: t}, nil
}

// UpdateIntel caches Intel GPU stats and triggers a full re-render.
func (w *GPUWidget) UpdateIntel(stats *collector.IntelGPUStats) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.intelStats = stats
	w.render()
}

// UpdateNvidia caches NVIDIA GPU stats and triggers a full re-render.
func (w *GPUWidget) UpdateNvidia(stats []collector.NvidiaGPUStats) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.nvidiaStats = stats
	w.render()
}

// UpdateAmd caches AMD GPU stats and triggers a full re-render.
func (w *GPUWidget) UpdateAmd(stats []collector.AmdGPUStats) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.amdStats = stats
	w.render()
}

// render redraws all cached GPU stats into a single combined view.
// Must be called with w.mu held.
func (w *GPUWidget) render() {
	hasAny := w.intelStats != nil || len(w.nvidiaStats) > 0 || len(w.amdStats) > 0
	if !hasAny {
		return
	}

	w.Visible = true
	w.Text.Reset()

	// Intel
	if s := w.intelStats; s != nil && (len(s.Engines) > 0 || s.FreqActMHz > 0) {
		title := " Intel GPU"
		if s.Name != "" {
			title = " " + s.Name
		}
		w.Text.Write(title+"\n", text.WriteCellOpts(cell.FgColor(cell.ColorNumber(75)), cell.Bold()))

		w.Text.Write(
			fmt.Sprintf(" Freq: %.0f MHz (req %.0f MHz)\n", s.FreqActMHz, s.FreqReqMHz),
			text.WriteCellOpts(cell.FgColor(cell.ColorNumber(250))),
		)
		w.Text.Write(
			fmt.Sprintf(" RC6:  %.1f%%\n", s.RC6Pct),
			text.WriteCellOpts(cell.FgColor(cell.ColorNumber(250))),
		)
		if s.PowerGPUWatts > 0 || s.PowerPkgWatts > 0 {
			w.Text.Write(
				fmt.Sprintf(" Power: GPU %.1fW  Pkg %.1fW\n", s.PowerGPUWatts, s.PowerPkgWatts),
				text.WriteCellOpts(cell.FgColor(cell.ColorNumber(250))),
			)
		}
		for _, eng := range s.Engines {
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

	// NVIDIA
	for i, gpu := range w.nvidiaStats {
		if i > 0 || w.intelStats != nil {
			w.Text.Write("\n")
		}
		w.Text.Write(
			fmt.Sprintf(" %s\n", gpu.Name),
			text.WriteCellOpts(cell.FgColor(cell.ColorNumber(76)), cell.Bold()),
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
			fmt.Sprintf(" %d°C  %.1fW/%.0fW  Core %dMHz  Mem %dMHz\n",
				gpu.TempC, gpu.PowerWatts, gpu.PowerLimitWatts,
				gpu.ClockCoreMHz, gpu.ClockMemMHz),
			text.WriteCellOpts(cell.FgColor(cell.ColorNumber(250))),
		)
	}

	// AMD
	for i, gpu := range w.amdStats {
		if i > 0 || w.intelStats != nil || len(w.nvidiaStats) > 0 {
			w.Text.Write("\n")
		}
		w.Text.Write(
			fmt.Sprintf(" %s\n", gpu.Name),
			text.WriteCellOpts(cell.FgColor(cell.ColorNumber(196)), cell.Bold()),
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
