package utils

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
	"github.com/shlewislee/typos/internal/printer"
	"github.com/urfave/cli/v3"
)

func NewDeviceFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "device",
			Aliases: []string{"d"},
			Value:   "/dev/ttyACM0",
			Usage:   "Device path for the ESC/POS printer.",
			Sources: cli.EnvVars("TYPOS_DEVICE"),
		},
		&cli.IntFlag{
			Name:    "baudrate",
			Value:   9600,
			Usage:   "Baudrate for the ESC/POS printer.",
			Sources: cli.EnvVars("TYPOS_BAUDRATE"),
		},
		&cli.IntFlag{
			Name:    "width",
			Aliases: []string{"w"},
			Value:   72,
			Usage:   "Printable width for the ESC/POS printer in millimeter. Note that this is different from the paper width.",
			Sources: cli.EnvVars("TYPOS_WIDTH"),
		},
		&cli.IntFlag{
			Name:    "dpi",
			Value:   203,
			Usage:   "DPI for the ESC/POS printer.",
			Sources: cli.EnvVars("TYPOS_DPI"),
		},
	}
}

func NewGammaFlag() cli.Flag {
	return &cli.Float64Flag{
		Name:    "gamma",
		Aliases: []string{"g"},
		Value:   4.5,
		Usage:   "Gamma value for image preprocessing.",
		Sources: cli.EnvVars("TYPOS_GAMMA"),
		Action: func(_ context.Context, _ *cli.Command, g float64) error {
			const lowWarn = 1.0
			const highWarn = 6.0
			if g <= lowWarn {
				slog.Warn("Gamma value is unusually low, image may be too dark.", "input_gamma", g)
			}
			if g >= highWarn {
				slog.Warn("Gamma value is unusually high, image may be too bright.", "input_gamma", g)
			}
			return nil
		},
	}
}

func NewDitherFlag() cli.Flag {
	return &cli.IntFlag{
		Name:    "dither-method",
		Value:   0,
		Usage:   "Dither method for the image. (0: Atkinson, 1: FloydSteinberg, 2: StevenPigeon)",
		Sources: cli.EnvVars("TYPOS_DITHER_METHOD"),
		Validator: func(m int) error {
			if m > 2 || m < 0 {
				return fmt.Errorf("invalid method number")
			}
			return nil
		},
	}
}

func HandleDeviceFlags(cmd *cli.Command) *printer.Options {
	return &printer.Options{
		PortName: cmd.String("device"),
		BaudRate: cmd.Int("baudrate"),
		Width:    cmd.Int("width"),
		DPI:      cmd.Int("dpi"),
	}
}

func HandleImageProcessFlags(cmd *cli.Command) *printer.ImageOptions {
	return &printer.ImageOptions{
		Gamma:        cmd.Float("gamma"),
		DitherMethod: printer.DitherMethod(cmd.Int("dither-method")),
	}
}

func NewLogger(isDebug bool) *slog.Logger {
	w := os.Stderr
	var lvl slog.Leveler
	if isDebug {
		lvl = slog.LevelDebug
	} else {
		lvl = slog.LevelInfo
	}

	return slog.New(
		tint.NewHandler(w, &tint.Options{
			Level:      lvl,
			TimeFormat: time.DateTime,
		}),
	)
}

func NewJSONLogger(isDebug bool) *slog.Logger {
	w := os.Stderr
	var lvl slog.Leveler
	if isDebug {
		lvl = slog.LevelDebug
	} else {
		lvl = slog.LevelInfo
	}

	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: lvl,
	}))
}
