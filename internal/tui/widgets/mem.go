package widgets

import (
	"fmt"
	"sync"

	"gtop/internal/collector"

	"github.com/mum4k/termdash/cell"
	"github.com/mum4k/termdash/widgets/text"
)

// MemWidget renders memory and swap information as text with bar indicators.
type MemWidget struct {
	mu   sync.Mutex
	Text *text.Text
}

// NewMemWidget creates the memory info widget.
func NewMemWidget() (*MemWidget, error) {
	t, err := text.New(text.WrapAtRunes())
	if err != nil {
		return nil, fmt.Errorf("mem text: %w", err)
	}
	return &MemWidget{Text: t}, nil
}

// Update refreshes the memory display.
func (w *MemWidget) Update(stats collector.MemStats) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.Text.Reset()

	var usedPct float64
	if stats.Total > 0 {
		usedPct = float64(stats.Used) / float64(stats.Total) * 100
	}

	// Total
	w.Text.Write(
		fmt.Sprintf(" Total:     %s\n", humanBytes(stats.Total)),
		text.WriteCellOpts(cell.FgColor(cell.ColorNumber(250))),
	)
	// Used + bar
	w.Text.Write(
		fmt.Sprintf(" Used:  %s  %3.0f%%\n", humanBytes(stats.Used), usedPct),
		text.WriteCellOpts(cell.FgColor(cell.ColorNumber(214))),
	)
	w.Text.Write(" "+makeBar(16, usedPct, "█", "░")+"\n",
		text.WriteCellOpts(cell.FgColor(cell.ColorNumber(214))),
	)

	// Available
	w.Text.Write(
		fmt.Sprintf(" Available: %s\n", humanBytes(stats.Available)),
		text.WriteCellOpts(cell.FgColor(cell.ColorNumber(77))),
	)

	// Cached
	var cachedPct float64
	if stats.Total > 0 {
		cachedPct = float64(stats.Cached) / float64(stats.Total) * 100
	}
	w.Text.Write(
		fmt.Sprintf(" Cached:%s  %3.0f%%\n", humanBytes(stats.Cached), cachedPct),
		text.WriteCellOpts(cell.FgColor(cell.ColorNumber(33))),
	)
	w.Text.Write(" "+makeBar(16, cachedPct, "█", "░")+"\n",
		text.WriteCellOpts(cell.FgColor(cell.ColorNumber(33))),
	)

	// Free
	w.Text.Write(
		fmt.Sprintf(" Free:  %s\n", humanBytes(stats.Free)),
		text.WriteCellOpts(cell.FgColor(cell.ColorNumber(250))),
	)

	// Swap
	if stats.SwapTotal > 0 {
		swapPct := float64(stats.SwapUsed) / float64(stats.SwapTotal) * 100
		w.Text.Write("\n", text.WriteCellOpts(cell.FgColor(cell.ColorNumber(250))))
		w.Text.Write(
			fmt.Sprintf(" Swap:  %s / %s  %3.0f%%\n",
				humanBytes(stats.SwapUsed), humanBytes(stats.SwapTotal), swapPct),
			text.WriteCellOpts(cell.FgColor(cell.ColorNumber(196))),
		)
		w.Text.Write(" "+makeBar(16, swapPct, "█", "░")+"\n",
			text.WriteCellOpts(cell.FgColor(cell.ColorNumber(196))),
		)
	}
}

// humanBytes formats bytes into human-readable string (GiB/MiB).
func humanBytes(b uint64) string {
	const (
		gib = 1024 * 1024 * 1024
		mib = 1024 * 1024
	)
	switch {
	case b >= gib:
		return fmt.Sprintf("%5.2f GiB", float64(b)/float64(gib))
	case b >= mib:
		return fmt.Sprintf("%5.0f MiB", float64(b)/float64(mib))
	default:
		return fmt.Sprintf("%5d KiB", b/1024)
	}
}

// makeBar creates a horizontal bar using fill/empty runes.
func makeBar(width int, pct float64, fill, empty string) string {
	filled := int(pct / 100.0 * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	bar := ""
	for i := 0; i < filled; i++ {
		bar += fill
	}
	for i := filled; i < width; i++ {
		bar += empty
	}
	return bar
}
