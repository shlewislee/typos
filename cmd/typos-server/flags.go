package main

import "github.com/urfave/cli/v3"

func NewTemplatesFlag() cli.Flag {
	return &cli.StringFlag{
		Name:    "templates",
		Usage:   "Set path to the TOML templates file mapping template names to paths.",
		Sources: cli.EnvVars("TYPOS_TEMPLATES"),
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

func NewFontPathFlag() cli.Flag {
	return &cli.StringFlag{
		Name:    "font-path",
		Usage:   "Adds additional directories that are recursively searched for fonts.",
		Sources: cli.EnvVars("TYPOS_FONT_PATH"),
	}
}

func NewAddrFlag() cli.Flag {
	return &cli.StringFlag{
		Name:    "addr",
		Aliases: []string{"a"},
		Value:   "127.0.0.1:8888",
		Usage:   "Address to listen on (e.g., :8888 or 127.0.0.1:8888).",
		Sources: cli.EnvVars("TYPOS_ADDR"),
	}
}

func NewMaxJobsFlag() cli.Flag {
	return &cli.IntFlag{
		Name:    "max-jobs",
		Value:   1000,
		Usage:   "Maximum number of jobs to keep in history.",
		Sources: cli.EnvVars("TYPOS_MAX_JOBS"),
	}
}
