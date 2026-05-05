// Package printer handles everything related to ESC/POS printer.
package printer

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/shlewislee/typos/internal/typst"
	"go.bug.st/serial"
)

type Printer struct {
	PortName string
	BaudRate int
	WidthPx  int
	Logger   *slog.Logger
	DPI      int

	serialPort serial.Port
}

type Options struct {
	// Serial port to use(e.g. /dev/ttyXXX).
	PortName string
	// Check your printer's specification for detail. If this is incorrect, the printer won't work at all.
	BaudRate int
	// Printable width for the thermal paper. Note that this is different from the actual paper size. Usually, printers for 80mm papers would be 72mm.
	// Check your printer's specification for detail. Note that incorrect width may lead to unexpected results.
	Width int
	// afaik, this is almost always 203.
	DPI    int
	Logger *slog.Logger
}

// Default values for the printer.
func (opts *Options) fillDefaults() {
	if opts.PortName == "" {
		// This is mine.
		opts.PortName = "/dev/ttyACM0"
	}
	if opts.BaudRate == 0 {
		opts.BaudRate = 9600
	}
	if opts.Width == 0 {
		opts.Width = 72
	}
	if opts.DPI == 0 {
		opts.DPI = 203
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
}

func NewPrinter(opts *Options) *Printer {
	if opts == nil {
		opts = &Options{}
	}
	opts.fillDefaults()

	widthPx := calculatePixels(opts.Width, opts.DPI)

	return &Printer{
		PortName: opts.PortName,
		BaudRate: opts.BaudRate,
		WidthPx:  widthPx,
		Logger:   opts.Logger,
		DPI:      opts.DPI,
	}
}

func (p *Printer) OpenSerial() error {
	mode := &serial.Mode{
		BaudRate: p.BaudRate,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}
	port, err := serial.Open(p.PortName, mode)
	if err != nil {
		var portErr *serial.PortError
		if errors.As(err, &portErr) {
			return fmt.Errorf("serial connection error(%v): %v", portErr.Code(), portErr.EncodedErrorString())
		}
		return err
	}
	p.serialPort = port
	return nil
}

func (p *Printer) Status() (string, error) {
	if p.serialPort == nil {
		return "", fmt.Errorf("serial port not open")
	}
	if err := p.serialPort.SetReadTimeout(time.Second); err != nil {
		return "", fmt.Errorf("failed to set read timeout: %w", err)
	}
	defer p.serialPort.SetReadTimeout(serial.NoTimeout)

	if _, err := p.serialPort.Write([]byte{0x10, 0x04, 0x01}); err != nil {
		return "", fmt.Errorf("failed to query status: %w", err)
	}

	buf := make([]byte, 1)
	if _, err := p.serialPort.Read(buf); err != nil {
		return "", fmt.Errorf("failed to read status response: %w", err)
	}

	if (buf[0] & 0x08) != 0 {
		return "offline", nil
	}
	return "online", nil
}

func (p *Printer) Close() error {
	if p.serialPort == nil {
		return nil
	}
	err := p.serialPort.Close()
	p.serialPort = nil
	return err
}

func (p *Printer) PrintImage(filename string, opts *ImageOptions) error {
	if opts == nil {
		opts = &ImageOptions{
			Logger: p.Logger,
		}
	}
	opts.fillDefaults()

	p.Logger.Debug("PrintImage method started")

	if _, err := p.serialPort.Write([]byte{0x1B, 0x40}); err != nil {
		return fmt.Errorf("failed to initialize printer: %w", err)
	}

	escposImg := &escposImage{
		filename: filename,
		maxWidth: p.WidthPx,
	}

	p.Logger.Debug("Preprocessing image...")
	if err := escposImg.preprocessImage(opts); err != nil {
		return err
	}
	p.Logger.Debug("Done")

	return p.writeImage(escposImg)
}

type TypstOptions struct {
	ImageOptions       *ImageOptions
	RenderTypstOptions *typst.CompileOptions
}

func (p *Printer) PrintTypst(filename string, opts *TypstOptions) (err error) {
	p.Logger.Debug("PrintTypst started", "filename", filename)
	c := typst.NewCompiler(p.Logger)
	tmpFile, err := c.Compile(filename, opts.RenderTypstOptions)
	if err != nil {
		return fmt.Errorf("typst compile failed: %w", err)
	}
	defer func() {
		p.Logger.Debug("Cleaning up Typst compiler output")
		if closeErr := tmpFile.Close(); closeErr != nil {
			p.Logger.Error("failed to cleanup Typst compiler output", "error", closeErr)
			err = errors.Join(err, closeErr)
		}
		if err == nil {
			p.Logger.Info("Done.")
		}
	}()

	p.Logger.Info("Done compiling. Printing...", "compile_result", tmpFile.Name)

	imgOpts := opts.ImageOptions
	if err := p.PrintImage(tmpFile.Name, imgOpts); err != nil {
		return fmt.Errorf("print failed: %w", err)
	}

	return err
}
