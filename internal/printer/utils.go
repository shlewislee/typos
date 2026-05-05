package printer

import "math"

func calculatePixels(width int, dpi int) int {
	dotsPerMm := float64(dpi) / 25.4
	totalDots := int(math.Round(dotsPerMm * float64(width)))

	// ESC/POS raster width MUST be a multiple of 8 (1 byte)
	// This floors it to the nearest byte-aligned width
	printableWidth := (totalDots / 8) * 8
	return printableWidth
}
