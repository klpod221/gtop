package widgets

import (
	"fmt"
	"sync"

	"gtop/internal/collector"

	"github.com/mum4k/termdash/cell"
	"github.com/mum4k/termdash/widgets/text"
)

// SystemWidget renders static host info (OS, Motherboard, etc.) similar to fastfetch.
type SystemWidget struct {
	mu   sync.Mutex
	Text *text.Text
}

func NewSystemWidget() (*SystemWidget, error) {
	t, err := text.New()
	if err != nil {
		return nil, err
	}
	return &SystemWidget{Text: t}, nil
}

func (w *SystemWidget) Update(info collector.HostInfo) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.Text.Reset()
	
	// OS & Kernel
	w.Text.Write(" OS     ", text.WriteCellOpts(cell.FgColor(cell.ColorCyan)))
	w.Text.Write(fmt.Sprintf("%s %s\n", info.OSVendor, info.OSVersion))
	
	w.Text.Write(" Kernel ", text.WriteCellOpts(cell.FgColor(cell.ColorCyan)))
	w.Text.Write(info.KernelVersion + "\n")

	// Host/Board
	w.Text.Write(" Host   ", text.WriteCellOpts(cell.FgColor(cell.ColorCyan)))
	board := info.MotherboardName
	if info.SystemVendor != "" {
		board = info.SystemVendor + " " + board
	}
	w.Text.Write(board + "\n")

	// CPU Summary (simplified)
	if len(info.CPUs) > 0 {
		w.Text.Write(" CPU    ", text.WriteCellOpts(cell.FgColor(cell.ColorCyan)))
		w.Text.Write(info.CPUs[0] + "\n")
	}

	// GPU Summary
	for i, gpu := range info.GPUs {
		label := " GPU    "
		if i > 0 {
			label = "        "
		}
		w.Text.Write(label, text.WriteCellOpts(cell.FgColor(cell.ColorCyan)))
		w.Text.Write(gpu + "\n")
	}
}
