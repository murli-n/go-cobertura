package parser

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

type Mode string

const (
	ModeSet    Mode = "set"
	ModeCount  Mode = "count"
	ModeAtomic Mode = "atomic"
)

type Block struct {
	Filename  string
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
	NumStmt   int
	Count     int64
}

type Profile struct {
	Mode   Mode
	Blocks []Block
}

var locationRe = regexp.MustCompile(`^(.*):([0-9]+)\.([0-9]+),([0-9]+)\.([0-9]+)$`)

func Parse(r io.Reader) (Profile, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	lineNo := 0
	var p Profile

	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if lineNo == 1 {
			mode, err := parseModeLine(line)
			if err != nil {
				return Profile{}, fmt.Errorf("line %d: %w", lineNo, err)
			}
			p.Mode = mode
			continue
		}

		block, err := parseBlockLine(line)
		if err != nil {
			return Profile{}, fmt.Errorf("line %d: %w", lineNo, err)
		}
		p.Blocks = append(p.Blocks, block)
	}

	if err := scanner.Err(); err != nil {
		return Profile{}, fmt.Errorf("scan profile: %w", err)
	}
	if p.Mode == "" {
		return Profile{}, fmt.Errorf("missing mode header")
	}

	return p, nil
}

func parseModeLine(line string) (Mode, error) {
	if !strings.HasPrefix(line, "mode:") {
		return "", fmt.Errorf("expected mode header, got %q", line)
	}
	mode := Mode(strings.TrimSpace(strings.TrimPrefix(line, "mode:")))
	switch mode {
	case ModeSet, ModeCount, ModeAtomic:
		return mode, nil
	default:
		return "", fmt.Errorf("unsupported mode %q", mode)
	}
}

func parseBlockLine(line string) (Block, error) {
	line = strings.TrimSpace(line)
	lastSep := strings.LastIndexAny(line, " \t")
	if lastSep <= 0 || lastSep >= len(line)-1 {
		return Block{}, fmt.Errorf("invalid block format")
	}
	countPart := strings.TrimSpace(line[lastSep+1:])
	rest := strings.TrimSpace(line[:lastSep])

	secondSep := strings.LastIndexAny(rest, " \t")
	if secondSep <= 0 || secondSep >= len(rest)-1 {
		return Block{}, fmt.Errorf("invalid block format")
	}
	numStmtPart := strings.TrimSpace(rest[secondSep+1:])
	loc := strings.TrimSpace(rest[:secondSep])

	numStmt, err := strconv.Atoi(numStmtPart)
	if err != nil {
		return Block{}, fmt.Errorf("invalid statement count %q: %w", numStmtPart, err)
	}
	count, err := strconv.ParseInt(countPart, 10, 64)
	if err != nil {
		return Block{}, fmt.Errorf("invalid hit count %q: %w", countPart, err)
	}

	m := locationRe.FindStringSubmatch(loc)
	if len(m) != 6 {
		return Block{}, fmt.Errorf("invalid location %q", loc)
	}

	startLine, _ := strconv.Atoi(m[2])
	startCol, _ := strconv.Atoi(m[3])
	endLine, _ := strconv.Atoi(m[4])
	endCol, _ := strconv.Atoi(m[5])
	if startLine <= 0 || endLine <= 0 {
		return Block{}, fmt.Errorf("line numbers must be positive")
	}
	if endLine < startLine {
		return Block{}, fmt.Errorf("end line before start line")
	}

	return Block{
		Filename:  m[1],
		StartLine: startLine,
		StartCol:  startCol,
		EndLine:   endLine,
		EndCol:    endCol,
		NumStmt:   numStmt,
		Count:     count,
	}, nil
}
