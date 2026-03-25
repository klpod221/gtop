package widgets

import (
	"fmt"
	"sync"

	"gtop/collector"

	"github.com/mum4k/termdash/cell"
	"github.com/mum4k/termdash/widgets/text"
)

// DiskWidget renders disk usage bars per mount point.
type DiskWidget struct {
	mu   sync.Mutex
	Text *text.Text
}

// NewDiskWidget creates the disk usage widget.
func NewDiskWidget() (*DiskWidget, error) {
	t, err := text.New(text.WrapAtRunes())
	if err != nil {
		return nil, fmt.Errorf("disk text: %w", err)
	}
	return &DiskWidget{Text: t}, nil
}

// Update refreshes disk usage display.
func (w *DiskWidget) Update(disks []collector.DiskSpace) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.Text.Reset()

	for _, d := range disks {
		pct := int(d.UsedPct)
		name := d.Name
		if len(name) > 8 {
			name = name[:8]
		}

		// Disk name
		w.Text.Write(
			fmt.Sprintf(" %-8s ", name),
			text.WriteCellOpts(cell.FgColor(cell.ColorNumber(75))),
		)

		// Usage bar
		color := barColorForPct(pct)
		w.Text.Write(
			makeBar(10, d.UsedPct, "█", "░"),
			text.WriteCellOpts(cell.FgColor(color)),
		)

		// Percentage and size
		w.Text.Write(
			fmt.Sprintf(" %3d%%\n", pct),
			text.WriteCellOpts(cell.FgColor(cell.ColorNumber(250))),
		)
		w.Text.Write(
			fmt.Sprintf("   Used: %-6s  %s\n", humanBytes(d.UsedBytes), humanBytes(d.TotalBytes)),
			text.WriteCellOpts(cell.FgColor(cell.ColorNumber(240))),
		)
	}
}

// barColorForPct returns color based on usage percentage.
func barColorForPct(pct int) cell.Color {
	switch {
	case pct >= 90:
		return cell.ColorNumber(196) // Red
	case pct >= 70:
		return cell.ColorNumber(214) // Orange
	case pct >= 50:
		return cell.ColorNumber(220) // Yellow
	default:
		return cell.ColorNumber(77) // Green
	}
}
