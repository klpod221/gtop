package widgets

import (
	"fmt"
	"sync"

	"gtop/collector"

	"github.com/mum4k/termdash/cell"
	"github.com/mum4k/termdash/widgets/sparkline"
	"github.com/mum4k/termdash/widgets/text"
)

const cpuHistoryLen = 200

// CPUWidget renders the CPU graph (sparkline) and per-core info panel.
type CPUWidget struct {
	mu sync.Mutex

	// Left panel: total CPU usage sparkline
	Graph *sparkline.SparkLine

	// Right panel: per-core bars + info text
	Info *text.Text

	history []int
}

// NewCPUWidget creates the CPU sparkline and info text widgets.
func NewCPUWidget() (*CPUWidget, error) {
	graph, err := sparkline.New(
		sparkline.Label("CPU"),
		sparkline.Color(cell.ColorNumber(77)),
	)
	if err != nil {
		return nil, fmt.Errorf("cpu sparkline: %w", err)
	}

	info, err := text.New(text.WrapAtRunes())
	if err != nil {
		return nil, fmt.Errorf("cpu info: %w", err)
	}

	return &CPUWidget{
		Graph:   graph,
		Info:    info,
		history: make([]int, 0, cpuHistoryLen),
	}, nil
}

// Update pushes new CPU stats into both widgets.
func (w *CPUWidget) Update(stats collector.CPUStats) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Update sparkline history
	val := int(stats.UsagePercent)
	if val < 0 {
		val = 0
	} else if val > 100 {
		val = 100
	}
	w.history = append(w.history, val)
	if len(w.history) > cpuHistoryLen {
		w.history = w.history[len(w.history)-cpuHistoryLen:]
	}
	w.Graph.Add([]int{val})

	// Build info panel text
	w.Info.Reset()

	// CPU name + freq + temp
	cpuName := stats.CpuName
	if len(cpuName) > 30 {
		cpuName = cpuName[:30]
	}

	var avgFreq uint64
	if len(stats.FreqMHz) > 0 {
		for _, f := range stats.FreqMHz {
			avgFreq += f
		}
		avgFreq /= uint64(len(stats.FreqMHz))
	}

	header := fmt.Sprintf(" %s\n", cpuName)
	w.Info.Write(header, text.WriteCellOpts(cell.FgColor(cell.ColorNumber(75))))

	freqTempLine := fmt.Sprintf(" %.1f GHz", float64(avgFreq)/1000.0)
	if stats.PackageTempC > 0 {
		freqTempLine += fmt.Sprintf("  %d°C", stats.PackageTempC)
	}
	if stats.PowerWatts > 0 {
		freqTempLine += fmt.Sprintf("  %.1fW", stats.PowerWatts)
	}
	freqTempLine += "\n"
	w.Info.Write(freqTempLine, text.WriteCellOpts(cell.FgColor(cell.ColorNumber(250))))

	// Per-core bars (2 columns)
	numCores := len(stats.CoresPercent)
	half := (numCores + 1) / 2
	for i := 0; i < half; i++ {
		left := formatCoreBar(i, stats.CoresPercent[i])
		right := ""
		if i+half < numCores {
			right = formatCoreBar(i+half, stats.CoresPercent[i+half])
		}
		line := fmt.Sprintf(" %s  %s\n", left, right)
		w.Info.Write(line, text.WriteCellOpts(cell.FgColor(cell.ColorNumber(250))))
	}

	// Load average
	w.Info.Write(
		fmt.Sprintf("\n Load avg: %.2f %.2f %.2f\n", stats.LoadAvg[0], stats.LoadAvg[1], stats.LoadAvg[2]),
		text.WriteCellOpts(cell.FgColor(cell.ColorNumber(250))),
	)

	// Uptime
	upH := int(stats.UptimeSeconds) / 3600
	upM := (int(stats.UptimeSeconds) % 3600) / 60
	w.Info.Write(
		fmt.Sprintf(" Uptime: %dh %dm\n", upH, upM),
		text.WriteCellOpts(cell.FgColor(cell.ColorNumber(240))),
	)
}

// formatCoreBar renders "C0 ████░░░░ 45%" using Unicode block chars.
func formatCoreBar(idx int, pct float64) string {
	barWidth := 8
	filled := int(pct / 100.0 * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	bar := ""
	for i := 0; i < filled; i++ {
		bar += "█"
	}
	for i := filled; i < barWidth; i++ {
		bar += "░"
	}
	return fmt.Sprintf("C%-2d%s%3.0f%%", idx, bar, pct)
}
