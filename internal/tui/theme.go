package tui

import "github.com/mum4k/termdash/cell"

// btop-inspired color palette.
var (
	ColorTitle      = cell.ColorNumber(75)  // Light blue
	ColorBorder     = cell.ColorNumber(240) // Gray
	ColorCPUGraph   = cell.ColorNumber(77)  // Green
	ColorCPUBar     = cell.ColorNumber(77)  // Green
	ColorMemUsed    = cell.ColorNumber(214) // Orange
	ColorMemCached  = cell.ColorNumber(33)  // Blue
	ColorMemFree    = cell.ColorNumber(77)  // Green
	ColorSwap       = cell.ColorNumber(196) // Red
	ColorNetRX      = cell.ColorNumber(77)  // Green (download)
	ColorNetTX      = cell.ColorNumber(196) // Red   (upload)
	ColorDiskBar    = cell.ColorNumber(214) // Orange
	ColorProcHeader = cell.ColorNumber(75)  // Light blue
	ColorProcHigh   = cell.ColorNumber(196) // Red (high CPU/MEM)
	ColorProcMed    = cell.ColorNumber(214) // Orange
	ColorProcLow    = cell.ColorNumber(250) // Light gray
	ColorGPU        = cell.ColorNumber(135) // Purple
	ColorLabel      = cell.ColorNumber(250) // Light gray
	ColorValue      = cell.ColorNumber(255) // White
)

// barColor returns green→yellow→red based on percentage.
func barColor(pct int) cell.Color {
	switch {
	case pct >= 90:
		return cell.ColorNumber(196) // Red
	case pct >= 70:
		return cell.ColorNumber(214) // Orange/Yellow
	case pct >= 50:
		return cell.ColorNumber(220) // Yellow
	default:
		return cell.ColorNumber(77) // Green
	}
}
