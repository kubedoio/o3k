package compat

import (
	"fmt"
	"os/exec"
)

const DefaultListenAddr = "127.0.0.1:35357"

type CheckerOptions struct {
	TerraformDir string
	OutputFormat string // "json" or "text"
	ListenAddr   string
}

type Checker struct {
	TerraformDir string
	OutputFormat string
	ListenAddr   string
}

func NewChecker(opts CheckerOptions) *Checker {
	if opts.OutputFormat == "" {
		opts.OutputFormat = "json"
	}
	if opts.ListenAddr == "" {
		opts.ListenAddr = DefaultListenAddr
	}
	return &Checker{
		TerraformDir: opts.TerraformDir,
		OutputFormat: opts.OutputFormat,
		ListenAddr:   opts.ListenAddr,
	}
}

func (c *Checker) Run() (*Report, error) {
	rec := NewRecorder()
	_ = rec // will be wired to the embedded server in Task 4

	if _, err := exec.LookPath("terraform"); err != nil {
		return nil, fmt.Errorf("terraform not found in PATH: %w", err)
	}
	if c.TerraformDir == "" {
		return nil, fmt.Errorf("TerraformDir must be set")
	}

	return &Report{
		Compatible:   true,
		OutputFormat: c.OutputFormat,
		Endpoints:    rec.Results(),
		Summary:      Summary{},
	}, nil
}
