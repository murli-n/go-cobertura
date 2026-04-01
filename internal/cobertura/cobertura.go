package cobertura

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/murli-n/go-cobertura/internal/convert"
)

type Coverage struct {
	XMLName      xml.Name `xml:"coverage"`
	LineRate     string   `xml:"line-rate,attr"`
	BranchRate   string   `xml:"branch-rate,attr"`
	LinesCovered int      `xml:"lines-covered,attr"`
	LinesValid   int      `xml:"lines-valid,attr"`
	LineHits     int      `xml:"line-hits,attr"`
	LineCount    int      `xml:"line-count,attr"`
	Complexity   string   `xml:"complexity,attr"`
	Version      string   `xml:"version,attr"`
	Timestamp    int64    `xml:"timestamp,attr"`
	Sources      Sources  `xml:"sources"`
	Packages     Packages `xml:"packages"`
}

type Sources struct {
	Source []string `xml:"source"`
}

type Packages struct {
	Package []Package `xml:"package"`
}

type Package struct {
	Name       string  `xml:"name,attr"`
	LineRate   string  `xml:"line-rate,attr"`
	BranchRate string  `xml:"branch-rate,attr"`
	LineHits   int     `xml:"line-hits,attr"`
	LineCount  int     `xml:"line-count,attr"`
	Complexity string  `xml:"complexity,attr"`
	Classes    Classes `xml:"classes"`
}

type Classes struct {
	Class []Class `xml:"class"`
}

type Class struct {
	Name       string  `xml:"name,attr"`
	Filename   string  `xml:"filename,attr"`
	LineRate   string  `xml:"line-rate,attr"`
	BranchRate string  `xml:"branch-rate,attr"`
	LineHits   int     `xml:"line-hits,attr"`
	LineCount  int     `xml:"line-count,attr"`
	Complexity string  `xml:"complexity,attr"`
	Methods    Methods `xml:"methods"`
	Lines      Lines   `xml:"lines"`
}

type Methods struct {
	Method []Method `xml:"method,omitempty"`
}

type Method struct {
	Name       string `xml:"name,attr"`
	Signature  string `xml:"signature,attr"`
	LineRate   string `xml:"line-rate,attr"`
	BranchRate string `xml:"branch-rate,attr"`
	Complexity string `xml:"complexity,attr"`
	LineCount  int    `xml:"line-count,attr"`
	LineHits   int    `xml:"line-hits,attr"`
}

type Lines struct {
	Line []Line `xml:"line"`
}

type Line struct {
	Number int   `xml:"number,attr"`
	Hits   int64 `xml:"hits,attr"`
	Branch bool  `xml:"branch,attr"`
}

func Build(project convert.ProjectData, source string, now time.Time) Coverage {
	cov := Coverage{
		LineRate:     formatRate(project.LineRate),
		BranchRate:   formatRate(project.BranchRate),
		LinesCovered: project.LineHits,
		LinesValid:   project.LineCount,
		LineHits:     project.LineHits,
		LineCount:    project.LineCount,
		Complexity:   "0",
		Version:      "go-cobertura",
		Timestamp:    now.Unix(),
		Sources: Sources{
			Source: []string{source},
		},
	}

	for _, p := range project.Packages {
		pkg := Package{
			Name:       p.Name,
			LineRate:   formatRate(p.LineRate),
			BranchRate: formatRate(p.BranchRate),
			LineHits:   p.LineHits,
			LineCount:  p.LineCount,
			Complexity: "0",
		}
		for _, f := range p.Files {
			class := Class{
				Name:       f.ClassName,
				Filename:   f.Filename,
				LineRate:   formatRate(f.LineRate),
				BranchRate: formatRate(f.BranchRate),
				LineHits:   f.LineHits,
				LineCount:  f.LineCount,
				Complexity: "0",
			}
			for _, m := range f.Methods {
				class.Methods.Method = append(class.Methods.Method, Method{
					Name:       m.Name,
					Signature:  m.Signature,
					LineRate:   formatRate(m.LineRate),
					BranchRate: formatRate(m.BranchRate),
					Complexity: "0",
					LineCount:  m.LineCount,
					LineHits:   m.LineHits,
				})
			}
			for _, l := range f.Lines {
				class.Lines.Line = append(class.Lines.Line, Line{
					Number: l.Number,
					Hits:   l.Hits,
					Branch: false,
				})
			}
			pkg.Classes.Class = append(pkg.Classes.Class, class)
		}
		cov.Packages.Package = append(cov.Packages.Package, pkg)
	}

	return cov
}

func Write(w io.Writer, coverage Coverage) error {
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(coverage); err != nil {
		return fmt.Errorf("encode coverage xml: %w", err)
	}
	if err := enc.Flush(); err != nil {
		return fmt.Errorf("flush coverage xml: %w", err)
	}
	return nil
}

func formatRate(v float64) string {
	return strconv.FormatFloat(v, 'f', 6, 64)
}
