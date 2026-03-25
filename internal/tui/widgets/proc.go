package widgets

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"gtop/internal/collector"

	"github.com/mum4k/termdash/cell"
	"github.com/mum4k/termdash/widgets/text"
)

const maxProcesses = 40

// ProcWidget renders the process list table.
type ProcWidget struct {
	mu   sync.Mutex
	Text *text.Text
}

// NewProcWidget creates the process table widget.
func NewProcWidget() (*ProcWidget, error) {
	t, err := text.New(text.WrapAtRunes())
	if err != nil {
		return nil, fmt.Errorf("proc text: %w", err)
	}
	return &ProcWidget{Text: t}, nil
}

// Update refreshes the process list, sorted by CPU usage descending.
func (w *ProcWidget) Update(procs []collector.ProcessInfo) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.Text.Reset()

	// Sort by CPU descending
	sort.Slice(procs, func(i, j int) bool {
		return procs[i].CpuPercent > procs[j].CpuPercent
	})

	// Header
	header := fmt.Sprintf(" %-7s %-8s %-15s %6s %6s %-5s %s\n",
		"PID", "User", "Name", "CPU%", "MEM%", "State", "Cmd")
	w.Text.Write(header, text.WriteCellOpts(cell.FgColor(cell.ColorNumber(75)), cell.Bold()))

	// Separator
	w.Text.Write(" "+strings.Repeat("─", 70)+"\n",
		text.WriteCellOpts(cell.FgColor(cell.ColorNumber(240))))

	limit := maxProcesses
	if len(procs) < limit {
		limit = len(procs)
	}

	for _, p := range procs[:limit] {
		name := p.Name
		if len(name) > 15 {
			name = name[:15]
		}
		user := p.User
		if len(user) > 8 {
			user = user[:8]
		}
		cmd := p.Cmdline
		if len(cmd) > 30 {
			cmd = cmd[:30]
		}

		// Color based on CPU usage
		var fg cell.Color
		switch {
		case p.CpuPercent >= 50:
			fg = cell.ColorNumber(196) // Red
		case p.CpuPercent >= 20:
			fg = cell.ColorNumber(214) // Orange
		case p.CpuPercent >= 5:
			fg = cell.ColorNumber(220) // Yellow
		default:
			fg = cell.ColorNumber(250) // Light gray
		}

		line := fmt.Sprintf(" %-7d %-8s %-15s %5.1f%% %5.1f%% %-5s %s\n",
			p.PID, user, name, p.CpuPercent, p.MemPercent, p.State, cmd)
		w.Text.Write(line, text.WriteCellOpts(cell.FgColor(fg)))
	}
}
