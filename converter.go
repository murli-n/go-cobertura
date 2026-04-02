package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/murli-n/go-cobertura/internal/cobertura"
	"github.com/murli-n/go-cobertura/internal/convert"
	"github.com/murli-n/go-cobertura/internal/parser"
)

type Options struct {
	PathStripPrefix   string
	BranchRateDefault float64
	SourceRoot        string
	Now               time.Time
	Logger            *log.Logger
}

func Convert(r io.Reader, w io.Writer, opts Options) error {
	debugf := func(format string, args ...any) {}
	if opts.Logger != nil {
		debugf = opts.Logger.Printf
	}
	debugf("converter: start conversion")

	profile, err := parser.Parse(r)
	if err != nil {
		return fmt.Errorf("parse coverage profile: %w", err)
	}
	debugf("converter: parsed profile mode=%s blocks=%d", profile.Mode, len(profile.Blocks))

	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}

	source := opts.SourceRoot
	if source == "" {
		cwd, cwdErr := os.Getwd()
		if cwdErr == nil {
			source = cwd
		} else {
			source = "."
		}
	}

	project := convert.BuildProject(profile, convert.Options{
		PathStripPrefix:   opts.PathStripPrefix,
		BranchRateDefault: opts.BranchRateDefault,
		BaseDir:           source,
		Debugf:            debugf,
	})
	debugf("converter: built project packages=%d line-count=%d line-hits=%d", len(project.Packages), project.LineCount, project.LineHits)

	doc := cobertura.Build(project, source, now)
	debugf("converter: built cobertura document")
	if err := cobertura.Write(w, doc); err != nil {
		return fmt.Errorf("write cobertura xml: %w", err)
	}
	debugf("converter: wrote cobertura xml successfully")
	return nil
}
