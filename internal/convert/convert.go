package convert

import (
	"go/ast"
	goparser "go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/murli-n/go-cobertura/internal/parser"
)

type Options struct {
	PathStripPrefix   string
	BranchRateDefault float64
	BaseDir           string
	Debugf            func(format string, args ...any)
}

type LineData struct {
	Number     int
	Hits       int64
	Statements int
}

type FileData struct {
	Filename   string
	Package    string
	ClassName  string
	Lines      []LineData
	Methods    []MethodData
	LineCount  int
	LineHits   int
	LineRate   float64
	BranchRate float64
}

type MethodData struct {
	Name       string
	Signature  string
	LineCount  int
	LineHits   int
	LineRate   float64
	BranchRate float64
}

type PackageData struct {
	Name       string
	Files      []FileData
	LineCount  int
	LineHits   int
	LineRate   float64
	BranchRate float64
}

type ProjectData struct {
	Packages   []PackageData
	LineCount  int
	LineHits   int
	LineRate   float64
	BranchRate float64
}

type lineAgg struct {
	hits       int64
	statements int
}

func BuildProject(profile parser.Profile, opts Options) ProjectData {
	fileLines := make(map[string]map[int]*lineAgg)
	mode := profile.Mode
	debugf(opts.Debugf, "convert: start build project, mode=%s, blocks=%d", mode, len(profile.Blocks))

	for _, b := range profile.Blocks {
		if _, ok := fileLines[b.Filename]; !ok {
			fileLines[b.Filename] = make(map[int]*lineAgg)
		}
		for line := b.StartLine; line <= b.EndLine; line++ {
			l, ok := fileLines[b.Filename][line]
			if !ok {
				l = &lineAgg{}
				fileLines[b.Filename][line] = l
			}
			l.statements += b.NumStmt

			hit := toHits(mode, b.Count)
			if mode == parser.ModeSet {
				if hit > l.hits {
					l.hits = hit
				}
			} else {
				l.hits += hit
			}
		}
	}

	pkgMap := make(map[string][]FileData)
	for rawFilename, linesMap := range fileLines {
		filename := normalizeFilename(rawFilename, opts)
		pkgName := toSlash(filepath.Dir(filename))
		if pkgName == "." {
			pkgName = "root"
		}

		lineNumbers := make([]int, 0, len(linesMap))
		for ln := range linesMap {
			lineNumbers = append(lineNumbers, ln)
		}
		sort.Ints(lineNumbers)

		file := FileData{
			Filename:   toSlash(filename),
			Package:    pkgName,
			ClassName:  strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename)),
			BranchRate: opts.BranchRateDefault,
		}
		for _, ln := range lineNumbers {
			line := linesMap[ln]
			file.Lines = append(file.Lines, LineData{
				Number:     ln,
				Hits:       line.hits,
				Statements: line.statements,
			})
			file.LineCount++
			if line.hits > 0 {
				file.LineHits++
			}
		}
		file.LineRate = ratio(file.LineHits, file.LineCount)
		file.Methods = buildMethods(rawFilename, file, opts)
		debugf(opts.Debugf, "convert: file=%s class=%s lines=%d hits=%d methods=%d", file.Filename, file.ClassName, file.LineCount, file.LineHits, len(file.Methods))
		pkgMap[pkgName] = append(pkgMap[pkgName], file)
	}

	project := ProjectData{
		BranchRate: opts.BranchRateDefault,
	}
	packageNames := make([]string, 0, len(pkgMap))
	for name := range pkgMap {
		packageNames = append(packageNames, name)
	}
	sort.Strings(packageNames)

	for _, name := range packageNames {
		files := pkgMap[name]
		sort.Slice(files, func(i, j int) bool {
			return files[i].Filename < files[j].Filename
		})

		pkg := PackageData{
			Name:       name,
			Files:      files,
			BranchRate: opts.BranchRateDefault,
		}
		for _, f := range files {
			pkg.LineCount += f.LineCount
			pkg.LineHits += f.LineHits
		}
		pkg.LineRate = ratio(pkg.LineHits, pkg.LineCount)
		project.Packages = append(project.Packages, pkg)
		project.LineCount += pkg.LineCount
		project.LineHits += pkg.LineHits
	}
	project.LineRate = ratio(project.LineHits, project.LineCount)
	debugf(opts.Debugf, "convert: completed build project, packages=%d files=%d line-count=%d line-hits=%d", len(project.Packages), len(fileLines), project.LineCount, project.LineHits)

	return project
}

func normalizeFilename(filename string, opts Options) string {
	out := toSlash(filename)
	strip := toSlash(opts.PathStripPrefix)
	if strip != "" {
		out = strings.TrimPrefix(out, strip)
		out = strings.TrimPrefix(out, "/")
	}
	return out
}

func toHits(mode parser.Mode, count int64) int64 {
	if mode == parser.ModeSet {
		if count > 0 {
			return 1
		}
		return 0
	}
	if count < 0 {
		return 0
	}
	return count
}

func ratio(hits, count int) float64 {
	if count == 0 {
		return 0
	}
	return float64(hits) / float64(count)
}

func toSlash(v string) string {
	return strings.ReplaceAll(v, `\`, `/`)
}

func buildMethods(rawFilename string, file FileData, opts Options) []MethodData {
	sourcePath, ok := resolveSourcePath(rawFilename, file.Filename, opts)
	if !ok {
		debugf(opts.Debugf, "convert: source not found for %s, using synthetic method", file.Filename)
		return []MethodData{syntheticMethod(file, opts)}
	}

	fset := token.NewFileSet()
	parsedFile, err := goparser.ParseFile(fset, sourcePath, nil, 0)
	if err != nil {
		debugf(opts.Debugf, "convert: parse source failed for %s (%v), using synthetic method", sourcePath, err)
		return []MethodData{syntheticMethod(file, opts)}
	}

	methods := make([]MethodData, 0)
	for _, decl := range parsedFile.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil || fn.Body == nil {
			continue
		}
		startLine := fset.Position(fn.Pos()).Line
		endLine := fset.Position(fn.End()).Line
		if endLine < startLine {
			continue
		}

		name := fn.Name.Name
		if fn.Recv != nil && len(fn.Recv.List) > 0 {
			name = receiverTypeName(fn.Recv.List[0].Type) + "." + fn.Name.Name
		}

		lineCount := 0
		lineHits := 0
		for _, ln := range file.Lines {
			if ln.Number < startLine || ln.Number > endLine {
				continue
			}
			lineCount++
			if ln.Hits > 0 {
				lineHits++
			}
		}

		methods = append(methods, MethodData{
			Name:       name,
			Signature:  "",
			LineCount:  lineCount,
			LineHits:   lineHits,
			LineRate:   ratio(lineHits, lineCount),
			BranchRate: opts.BranchRateDefault,
		})
	}

	if len(methods) == 0 {
		debugf(opts.Debugf, "convert: no funcs found in %s, using synthetic method", sourcePath)
		return []MethodData{syntheticMethod(file, opts)}
	}
	debugf(opts.Debugf, "convert: extracted %d methods from %s", len(methods), sourcePath)
	return methods
}

func resolveSourcePath(rawFilename, normalizedFilename string, opts Options) (string, bool) {
	candidates := []string{
		rawFilename,
		filepath.FromSlash(normalizedFilename),
	}

	if opts.PathStripPrefix != "" {
		rawSlash := toSlash(rawFilename)
		strip := toSlash(opts.PathStripPrefix)
		trimmed := strings.TrimPrefix(strings.TrimPrefix(rawSlash, strip), "/")
		if trimmed != "" {
			candidates = append(candidates, filepath.FromSlash(trimmed))
		}
	}

	if opts.BaseDir != "" {
		baseDirName := ""
		if absBase, err := filepath.Abs(opts.BaseDir); err == nil {
			baseDirName = filepath.Base(absBase)
		}

		trimmedCandidates := make([]string, 0, len(candidates))
		for _, c := range candidates {
			if c == "" || filepath.IsAbs(c) {
				continue
			}
			trimmed := trimLeadingDirName(c, baseDirName)
			if trimmed != "" && trimmed != c {
				trimmedCandidates = append(trimmedCandidates, trimmed)
				debugf(opts.Debugf, "convert: trimmed base dir prefix %q -> %q", c, trimmed)
			}
		}
		candidates = append(candidates, trimmedCandidates...)

		baseCandidates := make([]string, 0, len(candidates))
		for _, c := range candidates {
			if c == "" || filepath.IsAbs(c) {
				continue
			}
			baseCandidates = append(baseCandidates, filepath.Join(opts.BaseDir, c))
		}
		candidates = append(candidates, baseCandidates...)
	}

	seen := make(map[string]struct{}, len(candidates))
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		info, err := os.Stat(c)
		if err == nil && !info.IsDir() {
			return c, true
		}
	}
	return "", false
}

func syntheticMethod(file FileData, opts Options) MethodData {
	return MethodData{
		Name:       file.ClassName,
		Signature:  "",
		LineCount:  file.LineCount,
		LineHits:   file.LineHits,
		LineRate:   file.LineRate,
		BranchRate: opts.BranchRateDefault,
	}
}

func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return receiverTypeName(t.X)
	case *ast.IndexExpr:
		return receiverTypeName(t.X)
	case *ast.IndexListExpr:
		return receiverTypeName(t.X)
	case *ast.SelectorExpr:
		return receiverTypeName(t.X) + "." + t.Sel.Name
	default:
		return "recv"
	}
}

func debugf(logf func(format string, args ...any), format string, args ...any) {
	if logf == nil {
		return
	}
	logf(format, args...)
}

func trimLeadingDirName(path, dirName string) string {
	if dirName == "" || dirName == "." || dirName == "/" {
		return path
	}

	p := toSlash(filepath.Clean(path))
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimPrefix(p, "/")
	d := toSlash(filepath.Clean(dirName))
	d = strings.TrimPrefix(d, "./")
	d = strings.TrimPrefix(d, "/")
	if d == "" || d == "." {
		return path
	}

	prefix := d + "/"
	if strings.HasPrefix(p, prefix) {
		return strings.TrimPrefix(p, prefix)
	}
	return path
}
