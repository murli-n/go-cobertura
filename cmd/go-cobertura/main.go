package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	gocobertura "github.com/murli-n/go-cobertura"
)

func main() {
	var (
		inPath            string
		outPath           string
		pathStripPrefix   string
		sourceRoot        string
		branchRateDefault float64
		debug             bool
	)

	flag.StringVar(&inPath, "in", "", "input coverage profile path (default: stdin)")
	flag.StringVar(&outPath, "out", "", "output cobertura xml path (default: stdout)")
	flag.StringVar(&pathStripPrefix, "path-strip-prefix", "", "prefix to remove from file paths in output")
	flag.StringVar(&sourceRoot, "source-root", "", "source root written to <sources><source> (default: current working directory)")
	flag.Float64Var(&branchRateDefault, "branch-rate-default", 0, "default branch-rate value when branch data is unavailable")
	flag.BoolVar(&debug, "debug", false, "enable debug logs on stderr")
	flag.Parse()

	in, err := openInput(inPath)
	if err != nil {
		fail(err)
	}
	defer in.Close()

	out, err := openOutput(outPath)
	if err != nil {
		fail(err)
	}
	defer out.Close()

	var logger *log.Logger
	if debug {
		logger = log.New(os.Stderr, "go-cobertura debug: ", log.LstdFlags|log.Lmicroseconds)
	}

	if err := gocobertura.Convert(in, out, gocobertura.Options{
		PathStripPrefix:   pathStripPrefix,
		BranchRateDefault: branchRateDefault,
		SourceRoot:        sourceRoot,
		Logger:            logger,
	}); err != nil {
		fail(err)
	}
}

func openInput(path string) (io.ReadCloser, error) {
	if path == "" {
		return io.NopCloser(os.Stdin), nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open input %q: %w", path, err)
	}
	return f, nil
}

func openOutput(path string) (io.WriteCloser, error) {
	if path == "" {
		return nopWriteCloser{Writer: os.Stdout}, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create output %q: %w", path, err)
	}
	return f, nil
}

func fail(err error) {
	_, _ = fmt.Fprintf(os.Stderr, "go-cobertura: %v\n", err)
	os.Exit(1)
}

type nopWriteCloser struct {
	io.Writer
}

func (n nopWriteCloser) Close() error { return nil }
