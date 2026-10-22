package xcode

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Package is the part of a Package.swift manifest the selection needs. The manifest
// is Swift code, so this is a tolerant scanner, not a parser: it reads the target,
// product and local package declarations and ignores everything else.
type Package struct {
	Dir     string
	Name    string
	Targets []PackageTarget
	// Products maps a product name to the targets it is built from.
	Products map[string][]string
	// LocalDependencies are absolute directories of `.package(path:)` entries.
	LocalDependencies []string
}

// PackageTarget is one target declaration.
type PackageTarget struct {
	Name   string
	IsTest bool
	// Path is the absolute source directory.
	Path string
	// TargetDeps are dependencies given by target name (same package, or by-name lookup).
	TargetDeps []string
	// ProductDeps are `.product(name:package:)` dependencies.
	ProductDeps []string
}

var (
	reQuoted      = regexp.MustCompile(`"([^"\\]*(?:\\.[^"\\]*)*)"`)
	reProductDep  = regexp.MustCompile(`\.product\s*\(\s*name\s*:\s*"([^"]+)"`)
	reTargetDep   = regexp.MustCompile(`\.(?:target|byName|targetItem)\s*\(\s*name\s*:\s*"([^"]+)"`)
	// Whole calls, removed before the remaining bare strings are read as by-name deps.
	reDepCall     = regexp.MustCompile(`\.(?:product|target|byName|targetItem)\s*\([^)]*\)`)
	rePackagePath = regexp.MustCompile(`\.package\s*\((?:[^()]*?,\s*)?path\s*:\s*"([^"]+)"`)
)

// ParsePackage reads dir/Package.swift.
func ParsePackage(dir string) (*Package, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(abs, "Package.swift"))
	if err != nil {
		return nil, err
	}
	src := stripComments(string(data))
	pkg := &Package{Dir: abs, Products: map[string][]string{}}

	if i := strings.Index(src, "Package("); i >= 0 {
		body := balanced(src, i+len("Package(")-1)
		pkg.Name = topLevelString(body, "name")
	}
	if pkg.Name == "" {
		pkg.Name = filepath.Base(abs)
	}

	for _, m := range rePackagePath.FindAllStringSubmatch(src, -1) {
		pkg.LocalDependencies = append(pkg.LocalDependencies, filepath.Join(abs, m[1]))
	}

	// `.target(name:)` and `.plugin(name:package:)` also appear as items inside
	// dependencies and plugins arrays; those are references, not declarations, so the
	// arrays are blanked out before the declarations are collected.
	declSrc := blankArrays(src, "dependencies")
	declSrc = blankArrays(declSrc, "plugins")

	for _, kind := range []string{".target(", ".testTarget(", ".executableTarget(", ".macro(", ".plugin(", ".binaryTarget(", ".systemLibrary("} {
		// Declarations are located in the blanked copy; their bodies are read from the
		// original so that the dependencies are still there.
		for _, open := range callPositions(declSrc, kind) {
			body := balanced(src, open)
			t := PackageTarget{Name: topLevelString(body, "name"), IsTest: kind == ".testTarget("}
			if t.Name == "" {
				continue
			}
			rel := topLevelString(body, "path")
			if rel == "" {
				if kind == ".binaryTarget(" {
					// A remote binary has no files in the repository.
					pkg.Targets = append(pkg.Targets, t)
					continue
				}
				rel = defaultTargetDir(abs, t.Name, t.IsTest, kind == ".plugin(")
			}
			t.Path = filepath.Join(abs, rel)
			deps := topLevelArray(body, "dependencies")
			for _, m := range reProductDep.FindAllStringSubmatch(deps, -1) {
				t.ProductDeps = append(t.ProductDeps, m[1])
			}
			for _, m := range reTargetDep.FindAllStringSubmatch(deps, -1) {
				t.TargetDeps = append(t.TargetDeps, m[1])
			}
			stripped := reDepCall.ReplaceAllString(deps, "")
			for _, m := range reQuoted.FindAllStringSubmatch(stripped, -1) {
				t.TargetDeps = append(t.TargetDeps, m[1])
			}
			pkg.Targets = append(pkg.Targets, t)
		}
	}

	for _, kind := range []string{".library(", ".executable(", ".plugin("} {
		for _, body := range calls(src, kind) {
			name := topLevelString(body, "name")
			targets := topLevelArray(body, "targets")
			if name == "" || targets == "" {
				continue
			}
			for _, m := range reQuoted.FindAllStringSubmatch(targets, -1) {
				pkg.Products[name] = append(pkg.Products[name], m[1])
			}
		}
	}
	return pkg, nil
}

// defaultTargetDir follows SwiftPM's lookup order for a target without an explicit
// path: the first existing of the conventional roots, else the first root.
func defaultTargetDir(pkgDir, name string, isTest, isPlugin bool) string {
	roots := []string{"Sources", "Source", "src", "srcs"}
	switch {
	case isTest:
		roots = append([]string{"Tests"}, roots...)
	case isPlugin:
		roots = append([]string{"Plugins"}, roots...)
	}
	for _, r := range roots {
		if st, err := os.Stat(filepath.Join(pkgDir, r, name)); err == nil && st.IsDir() {
			return filepath.Join(r, name)
		}
	}
	return filepath.Join(roots[0], name)
}

// stripComments blanks `//` and `/* */` comments outside string literals, keeping
// offsets and newlines. A regex would also eat the `//` inside package URLs.
func stripComments(src string) string {
	out := []byte(src)
	inString := false
	for i := 0; i < len(src); i++ {
		c := src[i]
		if inString {
			if c == '\\' {
				i++
			} else if c == '"' {
				inString = false
			}
			continue
		}
		switch {
		case c == '"':
			inString = true
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				out[i] = ' '
				i++
			}
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			end := strings.Index(src[i+2:], "*/")
			stop := len(src)
			if end >= 0 {
				stop = i + 2 + end + 2
			}
			for ; i < stop; i++ {
				if src[i] != '\n' {
					out[i] = ' '
				}
			}
			i--
		}
	}
	return string(out)
}

// blankArrays replaces the contents of every `label: [ ... ]` array with spaces,
// keeping offsets stable so bodies can still be sliced from the original.
func blankArrays(src, label string) string {
	out := []byte(src)
	from := 0
	for {
		i := topLevelLabelFrom(src, label, from)
		if i < 0 {
			return string(out)
		}
		rest := src[i:]
		j := strings.IndexByte(rest, '[')
		if j < 0 || strings.TrimSpace(rest[:j]) != "" {
			from = i
			continue
		}
		open := i + j
		inner := balanced(src, open)
		for k := open + 1; k < open+1+len(inner) && k < len(out); k++ {
			if out[k] != '\n' {
				out[k] = ' '
			}
		}
		from = open + 1 + len(inner)
	}
}

// topLevelLabelFrom finds `label:` anywhere in src at or after from, regardless of
// bracket depth, and returns the index just after the colon.
func topLevelLabelFrom(src, label string, from int) int {
	for i := from; i < len(src); i++ {
		if !strings.HasPrefix(src[i:], label) || (i > 0 && isIdent(src[i-1])) {
			continue
		}
		k := i + len(label)
		for k < len(src) && (src[k] == ' ' || src[k] == '\t' || src[k] == '\n') {
			k++
		}
		if k < len(src) && src[k] == ':' {
			return k + 1
		}
	}
	return -1
}

// calls returns the argument bodies of every `kind(` call in src.
func calls(src, kind string) []string {
	var out []string
	for _, open := range callPositions(src, kind) {
		out = append(out, balanced(src, open))
	}
	return out
}

// callPositions returns the index of the opening parenthesis of every `kind(` call.
func callPositions(src, kind string) []int {
	var out []int
	from := 0
	for {
		i := strings.Index(src[from:], kind)
		if i < 0 {
			return out
		}
		open := from + i + len(kind) - 1
		out = append(out, open)
		from = open + 1
	}
}

// balanced returns the text between the bracket at src[open] and its match.
func balanced(src string, open int) string {
	if open < 0 || open >= len(src) {
		return ""
	}
	depth := 0
	inString := false
	for i := open; i < len(src); i++ {
		c := src[i]
		if inString {
			if c == '\\' {
				i++
			} else if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth == 0 {
				return src[open+1 : i]
			}
		}
	}
	return src[open+1:]
}

// topLevelString finds `label: "value"` at bracket depth 0 of body.
func topLevelString(body, label string) string {
	i := topLevelLabel(body, label)
	if i < 0 {
		return ""
	}
	rest := body[i:]
	m := reQuoted.FindStringSubmatchIndex(rest)
	if m == nil {
		return ""
	}
	// The value must be the next token, not a string somewhere further along.
	if strings.TrimSpace(rest[:m[0]]) != "" {
		return ""
	}
	return rest[m[2]:m[3]]
}

// topLevelArray finds `label: [ ... ]` at depth 0 and returns the array body.
func topLevelArray(body, label string) string {
	i := topLevelLabel(body, label)
	if i < 0 {
		return ""
	}
	rest := body[i:]
	j := strings.IndexByte(rest, '[')
	if j < 0 || strings.TrimSpace(rest[:j]) != "" {
		return ""
	}
	return balanced(rest, j)
}

// topLevelLabel returns the index just after `label:` when it occurs at depth 0.
func topLevelLabel(body, label string) int {
	depth := 0
	inString := false
	for i := 0; i < len(body); i++ {
		c := body[i]
		if inString {
			if c == '\\' {
				i++
			} else if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		default:
			if depth == 0 && strings.HasPrefix(body[i:], label) {
				after := i + len(label)
				if (i == 0 || !isIdent(body[i-1])) && after < len(body) {
					k := after
					for k < len(body) && (body[k] == ' ' || body[k] == '\t' || body[k] == '\n') {
						k++
					}
					if k < len(body) && body[k] == ':' {
						return k + 1
					}
				}
			}
		}
	}
	return -1
}

func isIdent(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}
