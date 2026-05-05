// Package typst handles everything Typst compile.
package typst

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/Dadido3/go-typst"
)

type CompileOptions struct {
	Input     map[string]string
	DPI       int
	Root      string
	FontPaths []string
}

type Compiler struct {
	Logger *slog.Logger
}

type TypstOutput struct {
	Name   string
	File   *os.File
	Logger *slog.Logger
}

func NewCompiler(logger *slog.Logger) *Compiler {
	return &Compiler{
		Logger: logger,
	}
}

func (c *Compiler) readAndCompile(input io.Reader, opts *CompileOptions) (*TypstOutput, error) {
	c.Logger.Debug("readAndCompile started", "dpi", opts.DPI, "inputs_count", len(opts.Input), "root", opts.Root)
	// CreateTemp returns an *os.File, which satisfies io.Writer
	tmpFile, err := os.CreateTemp("", "typos*.png")
	if err != nil {
		return nil, err
	}
	c.Logger.Debug("Temp PNG file created", "path", tmpFile.Name())

	isSuccess := false
	defer func() {
		if !isSuccess {
			if err := os.Remove(tmpFile.Name()); err != nil && !os.IsNotExist(err) {
				c.Logger.Error("failed to remove temporary PNG after failure", "path", tmpFile.Name(), "error", err)
			}
			tmpFile.Close()
		}
	}()

	typstCaller := typst.CLI{}

	c.Logger.Debug("Calling typst compiler")
	if err := typstCaller.Compile(input, tmpFile, &typst.OptionsCompile{
		Format:    typst.OutputFormatPNG,
		PPI:       opts.DPI,
		Input:     opts.Input,
		Root:      opts.Root,
		FontPaths: opts.FontPaths,
	}); err != nil {
		return nil, err
	}
	c.Logger.Debug("Typst file compiled successfully", "output", tmpFile.Name())

	if _, err := tmpFile.Seek(0, 0); err != nil {
		return nil, err
	}

	isSuccess = true
	return &TypstOutput{
		Name:   tmpFile.Name(),
		File:   tmpFile,
		Logger: c.Logger,
	}, nil
}

func (c *Compiler) Compile(filename string, opts *CompileOptions) (res *TypstOutput, err error) {
	c.Logger.Debug("Compiling Typst file", "filename", filename)
	// CreateTemp returns an *os.File, which satisfies io.Writer
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open typst file: %w", err)
	}
	c.Logger.Debug("Typst file opened", "file", filename)
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		c.Logger.Debug("Typst file closed", "file", filename)
	}()

	res, err = c.readAndCompile(file, opts)
	if err != nil {
		return nil, err
	}
	return res, err
}

func (o *TypstOutput) Close() (err error) {
	o.Logger.Debug("Closing and removing Typst output", "path", o.Name)
	if closeErr := o.File.Close(); closeErr != nil {
		o.Logger.Error("failed to close Typst output file", "path", o.Name, "error", closeErr)
		err = errors.Join(err, closeErr)
	}
	// even if close fails, we should attempt to remove the file.
	if removeErr := os.Remove(o.Name); removeErr != nil {
		o.Logger.Error("failed to remove Typst output file", "path", o.Name, "error", removeErr)
		err = errors.Join(err, removeErr)
	}
	return err
}
