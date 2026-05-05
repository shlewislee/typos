package main

import (
	"context"
	"log/slog"
	"net/mail"
	"os"

	"github.com/shlewislee/typos/cmd/utils"
	"github.com/urfave/cli/v3"
)

var version string = "dev"

func main() {
	cmd := newMainCmd()

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		slog.Error("Something went wrong", "err", err)
		os.Exit(1)
	}
}

func newMainCmd() *cli.Command {
	deviceFlags := utils.NewDeviceFlags()
	cliXtraFlags := []cli.Flag{
		utils.NewDitherFlag(),
		utils.NewGammaFlag(),
		NewRotateFlag(),
		NewVerboseFlag(),
	}

	cliXtraFlags = append(cliXtraFlags, deviceFlags...)

	h := &appHandler{}

	return &cli.Command{
		Usage: "Print images/Typst document with ESC/POS printer.",
		Authors: []any{
			mail.Address{Name: "shlewislee", Address: "shlewislee@shlewislee.me"},
		},
		Suggest: true,
		Before: func(ctx context.Context, c *cli.Command) (context.Context, error) {
			isDebug := c.Bool("verbose")
			h.logger = utils.NewLogger(isDebug)
			slog.SetDefault(h.logger)
			return ctx, nil
		},
		Version: version,
		// ---
		// Flags start here
		// ---
		Flags: cliXtraFlags,
		// ---
		// Commands start here
		// ---
		Commands: []*cli.Command{
			{
				Name:    "image",
				Aliases: []string{"i"},
				Usage:   "Print out image with ESC/POS printer.",
				Arguments: []cli.Argument{
					&cli.StringArg{
						Name: "input_file",
					},
				},
				Action: h.imageCmdAction,
			},
			{
				Name:    "print",
				Aliases: []string{"p"},
				Flags: []cli.Flag{
					NewInputFlag(),
				},
				Arguments: []cli.Argument{
					&cli.StringArg{
						Name: "input_file",
					},
				},
				Usage:  "Compile Typst file and print it with ESC/POS printer.",
				Action: h.printCmdAction,
			},
			{
				Name:   "status",
				Usage:  "Check printer online/offline status.",
				Action: h.statusCmdAction,
			},
		},
	}
}
