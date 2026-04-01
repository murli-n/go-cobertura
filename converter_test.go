package gocobertura

import (
	"bytes"
	"encoding/xml"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type coberturaCoverage struct {
	XMLName      xml.Name `xml:"coverage"`
	LineRate     string   `xml:"line-rate,attr"`
	BranchRate   string   `xml:"branch-rate,attr"`
	LineHits     int      `xml:"line-hits,attr"`
	LineCount    int      `xml:"line-count,attr"`
	LinesCovered int      `xml:"lines-covered,attr"`
	LinesValid   int      `xml:"lines-valid,attr"`
	Packages     struct {
		Package []struct {
			Name       string `xml:"name,attr"`
			LineRate   string `xml:"line-rate,attr"`
			BranchRate string `xml:"branch-rate,attr"`
			LineHits   int    `xml:"line-hits,attr"`
			LineCount  int    `xml:"line-count,attr"`
			Classes    struct {
				Class []struct {
					Name       string `xml:"name,attr"`
					Filename   string `xml:"filename,attr"`
					LineRate   string `xml:"line-rate,attr"`
					BranchRate string `xml:"branch-rate,attr"`
					LineHits   int    `xml:"line-hits,attr"`
					LineCount  int    `xml:"line-count,attr"`
					Methods    struct {
						Method []struct {
							Name string `xml:"name,attr"`
						} `xml:"method"`
					} `xml:"methods"`
					Lines      struct {
						Line []struct {
							Number int   `xml:"number,attr"`
							Hits   int64 `xml:"hits,attr"`
						} `xml:"line"`
					} `xml:"lines"`
				} `xml:"class"`
			} `xml:"classes"`
		} `xml:"package"`
	} `xml:"packages"`
}

func TestConvertSetMode(t *testing.T) {
	in := `mode: set
/workspace/pkg/a.go:10.1,10.8 1 1
/workspace/pkg/a.go:11.1,11.8 1 0
/workspace/pkg/a.go:11.1,11.8 1 1
`
	var out bytes.Buffer
	err := Convert(strings.NewReader(in), &out, Options{
		PathStripPrefix:   "/workspace/",
		BranchRateDefault: 0,
		SourceRoot:        "/workspace",
		Now:               time.Unix(1000, 0),
	})
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}

	var doc coberturaCoverage
	if err := xml.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("unmarshal xml: %v\n%s", err, out.String())
	}

	if doc.LineCount != 2 || doc.LineHits != 2 {
		t.Fatalf("unexpected coverage counts: line-count=%d line-hits=%d", doc.LineCount, doc.LineHits)
	}
	if doc.LineRate != "1.000000" {
		t.Fatalf("unexpected line-rate: %s", doc.LineRate)
	}

	if len(doc.Packages.Package) != 1 {
		t.Fatalf("unexpected package count: %d", len(doc.Packages.Package))
	}
	pkg := doc.Packages.Package[0]
	if pkg.Name != "pkg" {
		t.Fatalf("unexpected package name: %q", pkg.Name)
	}
	class := pkg.Classes.Class[0]
	if class.Filename != "pkg/a.go" {
		t.Fatalf("unexpected class filename: %q", class.Filename)
	}
	if class.LineCount != 2 || class.LineHits != 2 {
		t.Fatalf("unexpected class metrics: count=%d hits=%d", class.LineCount, class.LineHits)
	}
	if !strings.Contains(out.String(), "<class ") {
		t.Fatalf("expected <class> tag in xml")
	}
	if !strings.Contains(out.String(), "<methods>") || !strings.Contains(out.String(), "<method ") {
		t.Fatalf("expected <methods>/<method> tags in xml")
	}
}

func TestConvertCountModeLineHitsAggregated(t *testing.T) {
	in := `mode: count
pkg/b.go:20.1,20.8 1 2
pkg/b.go:20.1,20.8 1 3
pkg/b.go:21.1,21.8 1 0
`
	var out bytes.Buffer
	err := Convert(strings.NewReader(in), &out, Options{
		BranchRateDefault: 1,
		SourceRoot:        ".",
		Now:               time.Unix(2000, 0),
	})
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}

	var doc coberturaCoverage
	if err := xml.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("unmarshal xml: %v", err)
	}
	if doc.BranchRate != "1.000000" {
		t.Fatalf("unexpected branch-rate: %s", doc.BranchRate)
	}
	class := doc.Packages.Package[0].Classes.Class[0]
	if len(class.Lines.Line) != 2 {
		t.Fatalf("unexpected lines count: %d", len(class.Lines.Line))
	}
	if class.Lines.Line[0].Hits != 5 {
		t.Fatalf("expected line 20 hits 5, got %d", class.Lines.Line[0].Hits)
	}
}

func TestConvertIncludesMethodNamesFromSource(t *testing.T) {
	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "pkg")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	srcPath := filepath.Join(srcDir, "a.go")
	src := `package pkg

func Alpha() int {
	return 1
}

func Beta() int {
	return 2
}
`
	if err := os.WriteFile(srcPath, []byte(src), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	in := "mode: count\n" +
		filepath.ToSlash(srcPath) + ":3.1,4.2 1 1\n" +
		filepath.ToSlash(srcPath) + ":7.1,8.2 1 0\n"

	var out bytes.Buffer
	err := Convert(strings.NewReader(in), &out, Options{
		PathStripPrefix:   filepath.ToSlash(tmp) + "/",
		BranchRateDefault: 0,
		SourceRoot:        tmp,
		Now:               time.Unix(3000, 0),
	})
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}

	var doc coberturaCoverage
	if err := xml.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("unmarshal xml: %v\n%s", err, out.String())
	}

	class := doc.Packages.Package[0].Classes.Class[0]
	if len(class.Methods.Method) < 2 {
		t.Fatalf("expected at least 2 methods, got %d", len(class.Methods.Method))
	}
	foundAlpha := false
	foundBeta := false
	for _, m := range class.Methods.Method {
		if m.Name == "Alpha" {
			foundAlpha = true
		}
		if m.Name == "Beta" {
			foundBeta = true
		}
	}
	if !foundAlpha || !foundBeta {
		t.Fatalf("expected method names Alpha and Beta, got %+v", class.Methods.Method)
	}
}

func BenchmarkConvertLargeProfile(b *testing.B) {
	var sb strings.Builder
	sb.WriteString("mode: count\n")
	for i := 1; i <= 12000; i++ {
		sb.WriteString("pkg/large.go:")
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString(".1,")
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString(".10 1 1\n")
	}
	input := sb.String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out bytes.Buffer
		err := Convert(strings.NewReader(input), &out, Options{SourceRoot: ".", Now: time.Unix(1, 0)})
		if err != nil {
			b.Fatalf("Convert() error = %v", err)
		}
	}
}
