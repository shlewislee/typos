package main

import (
	"fmt"
	"strings"

	"github.com/shlewislee/typos/internal/typst"
	"github.com/urfave/cli/v3"
)

func NewInputFlag() cli.Flag {
	return &cli.StringSliceFlag{
		Name:        "input",
		Usage:       "Input for Typst compiler. Use key=value format.",
		DefaultText: "key=value",
		Validator: func(inputs []string) error {
			for _, i := range inputs {
				key, _, found := strings.Cut(i, "=")
				if !found {
					return fmt.Errorf("invalid format: '%s' is not in key=value format", i)
				}
				if key == "" {
					return fmt.Errorf("invalid format: empty key in '%s'", i)
				}
				if strings.ContainsAny(key, " ") {
					return fmt.Errorf("invalid format: key contains whitespace in '%s'", i)
				}
			}
			return nil
		},
	}
}

func NewRotateFlag() cli.Flag {
	return &cli.BoolFlag{
		Name:    "rotate",
		Aliases: []string{"r"},
		Usage:   "Rotate the image 90deg before print.",
	}
}

func NewVerboseFlag() cli.Flag {
	return &cli.BoolFlag{
		Name:    "verbose",
		Usage:   "Show additional debug info.",
		Hidden:  true,
		Sources: cli.EnvVars("TYPOS_VERBOSE"),
	}
}

func HandleCompileFlag(cmd *cli.Command) (*typst.CompileOptions, error) {
	inputs := cmd.StringSlice("input")
	dpi := cmd.Int("dpi")
	inputRes := make(map[string]string, len(inputs))

	for _, i := range inputs {
		k, v, err := parseKeyValue(i)
		if err != nil {
			return nil, err
		}
		inputRes[k] = v
	}

	return &typst.CompileOptions{
		Input: inputRes,
		DPI:   dpi,
	}, nil
}

func parseKeyValue(input string) (string, string, error) {
	key, value, found := strings.Cut(input, "=")
	if !found {
		return "", "", fmt.Errorf("invalid key=value format: %s", input)
	}
	if key == "" {
		return "", "", fmt.Errorf("empty key in key=value: %s", input)
	}
	return key, value, nil
}
