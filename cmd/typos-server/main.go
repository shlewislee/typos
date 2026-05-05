package main

import (
	"context"
	"log/slog"
	"net/mail"
	"os"

	"github.com/pelletier/go-toml/v2"
	"github.com/shlewislee/typos/cmd/utils"
	"github.com/shlewislee/typos/internal/printer"
	"github.com/shlewislee/typos/internal/server"
	"github.com/urfave/cli/v3"
)

var version string = "dev"

func main() {
	cmd := newMainCmd()

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		slog.Error("Failed to run command", "error", err)
		os.Exit(1)
	}
}

func newMainCmd() *cli.Command {
	flags := utils.NewDeviceFlags()
	flags = append(
		flags,
		NewTemplatesFlag(),
		NewVerboseFlag(),
		NewFontPathFlag(),
		NewAddrFlag(),
		NewMaxJobsFlag(),
		utils.NewGammaFlag(),
		utils.NewDitherFlag(),
	)

	return &cli.Command{
		Usage: "ESC/POS printer server for images/Typst documents",
		Authors: []any{
			mail.Address{Name: "shlewislee", Address: "shlewislee@shlewislee.me"},
		},
		Flags: flags,
		Before: func(ctx context.Context, c *cli.Command) (context.Context, error) {
			isDebug := c.Bool("verbose")
			logger := utils.NewJSONLogger(isDebug)
			slog.SetDefault(logger)
			return ctx, nil
		},
		Version: version,
		Commands: []*cli.Command{
			{
				Name:        "serve",
				Aliases:     []string{"s"},
				Usage:       "Start the HTTP server",
				UsageText:   "typos-server serve [serve options]",
				Description: "Starts the REST API server.",
				Action:      serveCmdAction,
			},
		},
	}
}

type TemplatesConfig struct {
	Templates []server.Template `toml:"templates"`
}

func serveCmdAction(ctx context.Context, cmd *cli.Command) error {
	logger := slog.Default()
	printerOpts := utils.HandleDeviceFlags(cmd)
	printerOpts.Logger = logger

	imageOpts := utils.HandleImageProcessFlags(cmd)

	p := printer.NewPrinter(printerOpts)
	if err := p.OpenSerial(); err != nil {
		logger.Error("Failed to open serial port", "error", err)
		return err
	}
	defer p.Close()

	if status, err := p.Status(); err != nil {
		logger.Warn("Printer status query failed at startup", "error", err)
	} else {
		logger.Info("Printer status at startup", "status", status)
	}

	templates := make(map[string]server.Template)
	templatesPath := cmd.String("templates")
	if templatesPath != "" {
		b, err := os.ReadFile(templatesPath)
		if err != nil {
			return err
		}
		var cfg TemplatesConfig
		if err := toml.Unmarshal(b, &cfg); err != nil {
			return err
		}
		for _, t := range cfg.Templates {
			templates[t.Name] = t
		}
	}

	opts := []server.Option{
		server.WithHost(cmd.String("addr")),
		server.WithPrinter(p),
		server.WithTemplates(templates),
		server.WithLogger(logger),
		server.WithFontPath(cmd.String("font-path")),
		server.WithMaxJobs(int(cmd.Int("max-jobs"))),
		server.WithDefaultGamma(imageOpts.Gamma),
		server.WithDefaultDitherMethod(imageOpts.DitherMethod),
	}

	s := server.NewServer(opts...)
	return s.Start(ctx)
}
