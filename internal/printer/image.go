package printer

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log/slog"
	"math"

	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/webp"

	"github.com/disintegration/imaging"
	"github.com/makeworld-the-better-one/dither/v2"
)

type escposImage struct {
	filename string
	maxWidth int

	newHeight int

	resBytes []byte
	wBytes   int
}

type ImageOptions struct {
	ShouldRotate bool         `json:"should_rotate"`
	DitherMethod DitherMethod `json:"dither_method"`
	Gamma        float64      `json:"gamma"`

	Logger *slog.Logger `json:"-"`
}

func (opts *ImageOptions) fillDefaults() {
	if math.IsNaN(opts.Gamma) || opts.Gamma == 0 {
		opts.Gamma = 4.50 // From my testing, seemingly unusually high gamma (like 3.0) is needed to aggressively pre-lighten the image and compensate for thermal paper heat bleed but YMMV.
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
}

type DitherMethod int

const (
	// Since heat bleeds when thermal printer prints, Atkinson will usually give you the best quality.
	DitherMethodAtkinson DitherMethod = iota // Defaults to Atkinson(int zero value).
	DitherMethodFloydSteinberg
	DitherMethodStevenPigeon
)

// preprocessImage opens, decodes, resizes, dithers, and packs the image for ESC/POS.
func (e *escposImage) preprocessImage(opts *ImageOptions) error {
	if opts == nil {
		opts = &ImageOptions{}
	}
	opts.fillDefaults() // Ensure defaults/logger exist

	src, err := imaging.Open(e.filename)
	if err != nil {
		return fmt.Errorf("failed to open image: %w", err)
	}
	opts.Logger.Debug("Image opened", "filename", e.filename)

	var img image.Image
	if opts.ShouldRotate {
		opts.Logger.Debug("Rotating image...")
		rotated := imaging.Rotate90(src)
		opts.Logger.Debug("Done")
		img = imaging.Resize(rotated, e.maxWidth, 0, imaging.Lanczos)
	} else {
		img = imaging.Resize(src, e.maxWidth, 0, imaging.Lanczos)
	}
	opts.Logger.Debug("Image resized")

	opts.Logger.Debug("Adjusting gamma...", "value", opts.Gamma)
	img = imaging.AdjustGamma(img, opts.Gamma)
	opts.Logger.Debug("Done")

	bounds := img.Bounds()

	// Create a solid white background
	bg := image.NewRGBA(bounds)
	draw.Draw(bg, bounds, image.White, image.Point{}, draw.Src)
	draw.Draw(bg, bounds, img, bounds.Min, draw.Over)

	gray := image.NewGray(bounds)
	draw.Draw(gray, bounds, bg, bounds.Min, draw.Src)

	e.newHeight = gray.Bounds().Dy()

	opts.Logger.Debug("Dithering started...")
	var matrix dither.ErrorDiffusionMatrix
	switch opts.DitherMethod {
	case DitherMethodAtkinson:
		matrix = dither.Atkinson
	case DitherMethodFloydSteinberg:
		matrix = dither.FloydSteinberg
	case DitherMethodStevenPigeon:
		matrix = dither.StevenPigeon
	default:
		matrix = dither.Atkinson
	}

	palette := []color.Color{color.Black, color.White}
	d := dither.NewDitherer(palette)
	d.Matrix = matrix

	resImg := d.Dither(gray)
	opts.Logger.Debug("Done")

	opts.Logger.Debug("Packing bits...")
	e.resBytes, e.wBytes = packBits(resImg)
	opts.Logger.Debug("Done")

	return nil
}

// packBits converts an image to the ESC/POS raster format.
func packBits(img image.Image) ([]byte, int) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()

	wBytes := (w + 7) / 8
	data := make([]byte, wBytes*h)

	if paletted, ok := img.(*image.Paletted); ok {
		for y := range h {
			for x := range w {
				offset := (y-b.Min.Y)*paletted.Stride + (x - b.Min.X)
				// The dither package uses index 0 for Black
				if paletted.Pix[offset] == 0 {
					data[y*wBytes+x/8] |= 1 << (7 - (x % 8))
				}
			}
		}
		return data, wBytes
	}

	for y := range h {
		for x := range w {
			r, _, _, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			if r < 0x8000 {
				data[y*wBytes+x/8] |= 1 << (7 - (x % 8))
			}
		}
	}
	return data, wBytes
}

func (p *Printer) writeImage(escposImg *escposImage) error {
	// https://download4.epson.biz/sec_pubs/pos/reference_en/escpos/index.html
	//
	// Yes, `GS v 0` _is_ deprecated. But this seemed like the easiest method I can use.
	// afaic almost every thermal printers will support the command.
	//
	// Construct command: GS v 0 m xL xH yL yH d1...dk
	cmd := []byte{0x1D, 0x76, 0x30, 0}

	// xL, xH (bytes per width)
	cmd = append(cmd, byte(escposImg.wBytes&0xFF), byte(escposImg.wBytes>>8))
	// yL, yH (dots per height)
	cmd = append(cmd, byte(escposImg.newHeight&0xFF), byte(escposImg.newHeight>>8))

	cmd = append(cmd, escposImg.resBytes...)

	p.Logger.Debug("Writing to serial port", "port", p.PortName)
	if _, err := p.serialPort.Write(cmd); err != nil {
		return fmt.Errorf("failed to write image data to serial port: %w", err)
	}

	// Feed twice so that the image won't cut off
	if _, err := p.serialPort.Write([]byte{0x0A, 0x0A}); err != nil {
		return fmt.Errorf("failed to write feed command: %w", err)
	}

	if _, err := p.serialPort.Write([]byte{0x1D, 0x56, 66, 0}); err != nil {
		return fmt.Errorf("failed to write cut command: %w", err)
	}
	p.Logger.Debug("Done")

	return nil
}
