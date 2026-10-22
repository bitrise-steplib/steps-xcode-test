// Package flutter builds the package graph of a Dart/Flutter repository from the
// pubspec.yaml files it contains and their path dependencies.
package flutter

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/bitrise-steplib/bitrise-step-test-selection/internal/graph"
)

// Directories that never contain first-party packages.
var skipDirs = map[string]bool{
	".git": true, ".dart_tool": true, "build": true, ".symlinks": true, "Pods": true,
	"node_modules": true, ".pub-cache": true, ".fvm": true, "ios": true, "android": true,
	"macos": true, "linux": true, "windows": true, "web": true,
}

type pubspec struct {
	Name         string                 `yaml:"name"`
	Dependencies map[string]interface{} `yaml:"dependencies"`
	DevDeps      map[string]interface{} `yaml:"dev_dependencies"`
	Overrides    map[string]interface{} `yaml:"dependency_overrides"`
}

type pkg struct {
	name    string
	dir     string // absolute
	deps    []string
	devDeps []string
}

// Build scans dir (relative to root) for packages and returns their graph.
func Build(root, dir string) (*graph.Graph, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	start := filepath.Join(root, dir)
	var pkgs []*pkg
	err = filepath.WalkDir(start, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != start && (skipDirs[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != "pubspec.yaml" {
			return nil
		}
		pk, err := parse(p)
		if err != nil {
			return err
		}
		pkgs = append(pkgs, pk)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no pubspec.yaml found under %s", dir)
	}

	byDir := map[string]string{}
	g := graph.New()
	for _, pk := range pkgs {
		id := "flutter:" + pk.name
		byDir[pk.dir] = id
		relDir, ok := rel(root, pk.dir)
		if !ok {
			continue
		}
		// A package at the repository root must not own every file in the repository,
		// so it gets the conventional package directories only.
		dirs := []string{relDir}
		if relDir == "." {
			dirs = nil
			for _, d := range []string{"lib", "bin", "assets", "fonts", "images"} {
				if st, err := os.Stat(filepath.Join(pk.dir, d)); err == nil && st.IsDir() {
					dirs = append(dirs, d)
				}
			}
		}
		g.Add(&graph.Node{
			ID: id, Name: pk.name, Kind: graph.KindSource, Ecosystem: "flutter",
			Dirs:  dirs,
			Files: map[string]bool{path.Join(relDir, "pubspec.yaml"): true},
			Meta:  map[string]string{"package_dir": relDir},
		})
		var testDirs []string
		for _, td := range []string{"test", "integration_test"} {
			if st, err := os.Stat(filepath.Join(pk.dir, td)); err == nil && st.IsDir() {
				testDirs = append(testDirs, path.Join(relDir, td))
			}
		}
		if len(testDirs) > 0 {
			g.Add(&graph.Node{
				ID: id + ":test", Name: pk.name + " tests", Kind: graph.KindTest, Ecosystem: "flutter",
				Dirs: testDirs, Deps: []string{id},
				Meta: map[string]string{"package_dir": relDir},
			})
		}
	}
	for _, pk := range pkgs {
		id := "flutter:" + pk.name
		for _, d := range pk.deps {
			if to, ok := byDir[d]; ok {
				g.AddDep(id, to)
			}
		}
		for _, d := range pk.devDeps {
			if to, ok := byDir[d]; ok {
				g.AddDep(id+":test", to)
			}
		}
	}
	g.Finalize()
	return g, nil
}

func parse(path string) (*pkg, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ps pubspec
	if err := yaml.Unmarshal(data, &ps); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	dir := filepath.Dir(path)
	pk := &pkg{name: ps.Name, dir: dir}
	if pk.name == "" {
		pk.name = filepath.Base(dir)
	}
	pk.deps = append(pathDeps(dir, ps.Dependencies), pathDeps(dir, ps.Overrides)...)
	pk.devDeps = pathDeps(dir, ps.DevDeps)
	return pk, nil
}

// pathDeps returns the absolute directories of `path:` dependencies.
func pathDeps(dir string, deps map[string]interface{}) []string {
	var out []string
	for _, v := range deps {
		m, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		if p, ok := m["path"].(string); ok && p != "" {
			out = append(out, filepath.Clean(filepath.Join(dir, p)))
		}
	}
	return out
}

func rel(root, p string) (string, bool) {
	r, err := filepath.Rel(root, p)
	if err != nil || strings.HasPrefix(r, "..") {
		return "", false
	}
	return filepath.ToSlash(r), true
}
