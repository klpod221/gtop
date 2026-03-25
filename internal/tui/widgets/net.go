package widgets

import (
	"fmt"
	"sync"

	"gtop/internal/collector"

	"github.com/mum4k/termdash/cell"
	"github.com/mum4k/termdash/widgets/text"
)

const netHistoryLen = 50

// NetWidget renders network traffic info with ASCII sparkline bars.
type NetWidget struct {
	mu   sync.Mutex
	Text *text.Text

	prevRX map[string]uint64
	prevTX map[string]uint64

	totalRX uint64
	totalTX uint64

	rxHistory []uint64
	txHistory []uint64
}

// NewNetWidget creates the network info widget.
func NewNetWidget() (*NetWidget, error) {
	t, err := text.New(text.WrapAtRunes())
	if err != nil {
		return nil, fmt.Errorf("net text: %w", err)
	}

	return &NetWidget{
		Text:      t,
		prevRX:    make(map[string]uint64),
		prevTX:    make(map[string]uint64),
		rxHistory: make([]uint64, 0, netHistoryLen),
		txHistory: make([]uint64, 0, netHistoryLen),
	}, nil
}

// Update processes network interface data and renders combined view.
func (w *NetWidget) Update(ifaces []collector.NetInterface) {
	w.mu.Lock()
	defer w.mu.Unlock()

	var totalDeltaRX, totalDeltaTX uint64
	var primaryIface *collector.NetInterface

	for i, iface := range ifaces {
		if !iface.Connected || iface.Name == "lo" {
			continue
		}
		if primaryIface == nil {
			primaryIface = &ifaces[i]
		}

		if prev, ok := w.prevRX[iface.Name]; ok && iface.RxBytes >= prev {
			totalDeltaRX += iface.RxBytes - prev
		}
		if prev, ok := w.prevTX[iface.Name]; ok && iface.TxBytes >= prev {
			totalDeltaTX += iface.TxBytes - prev
		}

		w.prevRX[iface.Name] = iface.RxBytes
		w.prevTX[iface.Name] = iface.TxBytes
	}

	w.totalRX += totalDeltaRX
	w.totalTX += totalDeltaTX

	// Track history for sparkline
	w.rxHistory = append(w.rxHistory, totalDeltaRX)
	w.txHistory = append(w.txHistory, totalDeltaTX)
	if len(w.rxHistory) > netHistoryLen {
		w.rxHistory = w.rxHistory[len(w.rxHistory)-netHistoryLen:]
	}
	if len(w.txHistory) > netHistoryLen {
		w.txHistory = w.txHistory[len(w.txHistory)-netHistoryLen:]
	}

	// Render
	w.Text.Reset()

	// Interface info line
	if primaryIface != nil {
		w.Text.Write(
			fmt.Sprintf(" net %s", primaryIface.Name),
			text.WriteCellOpts(cell.FgColor(cell.ColorNumber(75))),
		)
		if primaryIface.IPv4 != "" && primaryIface.IPv4 != primaryIface.MAC {
			w.Text.Write(
				fmt.Sprintf("  %s", primaryIface.IPv4),
				text.WriteCellOpts(cell.FgColor(cell.ColorNumber(250))),
			)
		}
		w.Text.Write("\n", text.WriteCellOpts(cell.FgColor(cell.ColorNumber(250))))
	}

	// Download sparkline bar + stats
	w.Text.Write(" ", text.WriteCellOpts(cell.FgColor(cell.ColorNumber(250))))
	w.writeSparkBar(w.rxHistory, cell.ColorNumber(77))
	w.Text.Write("\n", text.WriteCellOpts(cell.FgColor(cell.ColorNumber(250))))

	w.Text.Write(
		fmt.Sprintf("   ↓ %s/s", humanBytesShort(totalDeltaRX)),
		text.WriteCellOpts(cell.FgColor(cell.ColorNumber(77))),
	)
	w.Text.Write("  download\n", text.WriteCellOpts(cell.FgColor(cell.ColorNumber(240))))

	w.Text.Write(
		fmt.Sprintf("   ↓ Top: %s/s", humanBytesShort(maxVal(w.rxHistory))),
		text.WriteCellOpts(cell.FgColor(cell.ColorNumber(240))),
	)
	w.Text.Write(
		fmt.Sprintf("  ↓ Total: %s\n", humanBytesShort(w.totalRX)),
		text.WriteCellOpts(cell.FgColor(cell.ColorNumber(240))),
	)

	// Upload sparkline bar + stats
	w.Text.Write("\n ", text.WriteCellOpts(cell.FgColor(cell.ColorNumber(250))))
	w.writeSparkBar(w.txHistory, cell.ColorNumber(196))
	w.Text.Write("\n", text.WriteCellOpts(cell.FgColor(cell.ColorNumber(250))))

	w.Text.Write(
		fmt.Sprintf("   ↑ %s/s", humanBytesShort(totalDeltaTX)),
		text.WriteCellOpts(cell.FgColor(cell.ColorNumber(196))),
	)
	w.Text.Write("  upload\n", text.WriteCellOpts(cell.FgColor(cell.ColorNumber(240))))

	w.Text.Write(
		fmt.Sprintf("   ↑ Top: %s/s", humanBytesShort(maxVal(w.txHistory))),
		text.WriteCellOpts(cell.FgColor(cell.ColorNumber(240))),
	)
	w.Text.Write(
		fmt.Sprintf("  ↑ Total: %s\n", humanBytesShort(w.totalTX)),
		text.WriteCellOpts(cell.FgColor(cell.ColorNumber(240))),
	)
}

// writeSparkBar renders a Unicode sparkline bar from history data.
func (w *NetWidget) writeSparkBar(history []uint64, color cell.Color) {
	blocks := []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}
	maxV := maxVal(history)
	if maxV == 0 {
		maxV = 1
	}

	barWidth := 30
	start := 0
	if len(history) > barWidth {
		start = len(history) - barWidth
	}

	for i := start; i < len(history); i++ {
		level := int(float64(history[i]) / float64(maxV) * 7)
		if level < 0 {
			level = 0
		}
		if level > 7 {
			level = 7
		}
		w.Text.Write(blocks[level], text.WriteCellOpts(cell.FgColor(color)))
	}

	// Pad remaining
	for i := len(history) - start; i < barWidth; i++ {
		w.Text.Write(" ", text.WriteCellOpts(cell.FgColor(cell.ColorNumber(240))))
	}
}

// humanBytesShort formats bytes compactly.
func humanBytesShort(b uint64) string {
	const (
		gb = 1024 * 1024 * 1024
		mb = 1024 * 1024
		kb = 1024
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GiB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MiB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KiB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// maxVal returns the maximum value in a uint64 slice.
func maxVal(vals []uint64) uint64 {
	var m uint64
	for _, v := range vals {
		if v > m {
			m = v
		}
	}
	return m
}
