package parser

import (
	"strings"
	"testing"
)

func TestParseProfile(t *testing.T) {
	in := `mode: count
github.com/acme/proj/a.go:10.1,12.2 2 5
github.com/acme/proj/a.go:14.1,14.10 1 0
`
	p, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if p.Mode != ModeCount {
		t.Fatalf("unexpected mode: %q", p.Mode)
	}
	if got := len(p.Blocks); got != 2 {
		t.Fatalf("unexpected block count: %d", got)
	}
	if p.Blocks[0].Filename != "github.com/acme/proj/a.go" {
		t.Fatalf("unexpected filename: %q", p.Blocks[0].Filename)
	}
}

func TestParseInvalidMode(t *testing.T) {
	_, err := Parse(strings.NewReader("mode: weird\n"))
	if err == nil {
		t.Fatalf("expected error for unsupported mode")
	}
}

func TestParseInvalidBlock(t *testing.T) {
	in := `mode: set
bad block
`
	_, err := Parse(strings.NewReader(in))
	if err == nil {
		t.Fatalf("expected error for invalid block")
	}
}

func TestParseFilenameWithSpace(t *testing.T) {
	in := "mode: count\n" +
		"auth-service/global/tnc_service .go:37.54,43.14 4 0\n"
	p, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(p.Blocks) != 1 {
		t.Fatalf("expected one block, got %d", len(p.Blocks))
	}
	if got := p.Blocks[0].Filename; got != "auth-service/global/tnc_service .go" {
		t.Fatalf("unexpected filename: %q", got)
	}
}
