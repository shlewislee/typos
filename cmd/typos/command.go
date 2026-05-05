package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/shlewislee/typos/cmd/utils"
	"github.com/shlewislee/typos/internal/printer"
	"github.com/urfave/cli/v3"
)

type appHandler struct {
	logger *slog.Logger
}

func (app *appHandler) imageCmdAction(ctx context.Context, cmd *cli.Command) (err error) {
	pOpts := utils.HandleDeviceFlags(cmd)
	shouldRotate := cmd.Bool("rotate")
	imagePath := cmd.StringArg("input_file")

	logger := app.logger

	logger.Debug("Command initialized", "image_path", imagePath)

	logger.Info("Initializing printer...", "device_path", pOpts.PortName)

	p := printer.NewPrinter(pOpts)
	if err := p.OpenSerial(); err != nil {
		return fmt.Errorf("printer initialization failed: %w", err)
	}
	defer func() {
		if closeErr := p.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()
	logger.Info("Printer initialized", "device_path", pOpts.PortName)

	logger.Info("Printing image...", "image_path", imagePath)

	imgOpts := &printer.ImageOptions{
		ShouldRotate: shouldRotate,
		Gamma:        cmd.Float("gamma"),
		DitherMethod: printer.DitherMethod(cmd.Int("dither-method")),
	}

	if err := p.PrintImage(imagePath, imgOpts); err != nil {
		return fmt.Errorf("print failed: %w", err)
	}
	logger.Info("Done!")

	return err
}

func (app *appHandler) statusCmdAction(ctx context.Context, cmd *cli.Command) (err error) {
	pOpts := utils.HandleDeviceFlags(cmd)
	logger := app.logger

	logger.Info("Checking printer status...", "device_path", pOpts.PortName)

	p := printer.NewPrinter(pOpts)
	if err := p.OpenSerial(); err != nil {
		return fmt.Errorf("printer initialization failed: %w", err)
	}
	defer func() {
		if closeErr := p.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	status, err := p.Status()
	if err != nil {
		fmt.Println("na")
		return fmt.Errorf("status query failed: %w", err)
	}

	fmt.Println(status)
	return nil
}

func (app *appHandler) printCmdAction(ctx context.Context, cmd *cli.Command) (err error) {
	filename := cmd.StringArg("input_file")
	shouldRotate := cmd.Bool("rotate")

	pOpts := utils.HandleDeviceFlags(cmd)
	cOpts, err := HandleCompileFlag(cmd)
	if err != nil {
		return fmt.Errorf("flag parsing failed: %w", err)
	}

	logger := app.logger
	pOpts.Logger = logger

	logger.Info("Initializing...", "device_path", pOpts.PortName)

	p := printer.NewPrinter(pOpts)
	if err := p.OpenSerial(); err != nil {
		return fmt.Errorf("printer initialization failed: %w", err)
	}
	defer func() {
		if closeErr := p.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	logger.Info("Printer initialized", "device_path", pOpts.PortName)
	imgOpts := &printer.ImageOptions{
		ShouldRotate: shouldRotate,
		DitherMethod: printer.DitherMethod(cmd.Int("dither-method")),
		Gamma:        cmd.Float("gamma"),
		Logger:       p.Logger,
	}

	if err := p.PrintTypst(filename, &printer.TypstOptions{
		ImageOptions:       imgOpts,
		RenderTypstOptions: cOpts,
	}); err != nil {
		return err
	}

	return err
}
