package tui

import (
	"gtop/internal/tui/widgets"

	"github.com/mum4k/termdash/container"
	"github.com/mum4k/termdash/container/grid"
	"github.com/mum4k/termdash/linestyle"
	"github.com/mum4k/termdash/widgetapi"
)

// borderOpts returns standard border container options with a title.
func borderOpts(title string) []container.Option {
	return []container.Option{
		container.Border(linestyle.Light),
		container.BorderTitle(title),
		container.BorderTitleAlignCenter(),
		container.BorderColor(ColorBorder),
		container.FocusedColor(ColorBorder),
	}
}

// buildLayout creates the btop-style grid layout.
//
// Layout:
//
//	┌──── CPU Graph ────────┬── CPU Cores ──────┐  ~30%
//	│ sparkline             │ per-core bars+info │
//	├──── mem ───┬─ disks ──┼──── proc ─────────┤
//	│ RAM/Swap   │ mounts   │ process table     │
//	├──── gpu ──────────────┤                   │
//	│ engine bars/info      │                   │
//	├──── net ──────────────┤                   │
//	│ ↓↑ sparkline bars     │                   │
//	└───────────────────────┴───────────────────┘
func buildLayout(
	cpuGraph widgetapi.Widget,
	cpuInfo widgetapi.Widget,
	memText widgetapi.Widget,
	diskText widgetapi.Widget,
	netText widgetapi.Widget,
	procText widgetapi.Widget,
	gpuText widgetapi.Widget,
) []container.Option {
	builder := grid.New()

	// --- Top row: CPU Graph (left 60%) | CPU Cores (right 40%) — 30% height ---
	topRow := grid.RowHeightPerc(30,
		grid.ColWidthPerc(60,
			grid.Widget(cpuGraph, borderOpts(" cpu ")...),
		),
		grid.ColWidthPerc(40,
			grid.Widget(cpuInfo, borderOpts(" cores ")...),
		),
	)

	// --- Bottom section: Left panels (50%) | Proc (50%) — 69% height ---
	// Left column: mem+disks (35%) | gpu (25%) | net (39%)
	leftElements := []grid.Element{
		grid.RowHeightPerc(35,
			grid.ColWidthPerc(50,
				grid.Widget(memText, borderOpts(" mem ")...),
			),
			grid.ColWidthPerc(50,
				grid.Widget(diskText, borderOpts(" disks ")...),
			),
		),
		grid.RowHeightPerc(25,
			grid.Widget(gpuText, borderOpts(" gpu ")...),
		),
		grid.RowHeightPerc(39,
			grid.Widget(netText, borderOpts(" net ")...),
		),
	}

	bottomRow := grid.RowHeightPerc(69,
		grid.ColWidthPerc(50,
			leftElements...,
		),
		grid.ColWidthPerc(50,
			grid.Widget(procText, borderOpts(" proc ")...),
		),
	)

	builder.Add(topRow)
	builder.Add(bottomRow)

	gridOpts, _ := builder.Build()
	return gridOpts
}

// BuildContainer creates the btop-style container with all widgets.
func BuildContainer(w *widgets.CPUWidget, mem *widgets.MemWidget,
	disk *widgets.DiskWidget, net *widgets.NetWidget,
	proc *widgets.ProcWidget, gpu *widgets.GPUWidget) []container.Option {

	return buildLayout(
		w.Graph,
		w.Info,
		mem.Text,
		disk.Text,
		net.Text,
		proc.Text,
		gpu.Text,
	)
}
