package xcode

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitrise-steplib/bitrise-step-test-selection/internal/graph"
)

// Options selects what to analyse.
type Options struct {
	// Root is the repository root; every path in the graph is relative to it.
	Root string
	// ProjectPath is an .xcworkspace or .xcodeproj.
	ProjectPath string
	// Scheme restricts the candidate test targets to the scheme's testables. Empty
	// means every test target is a candidate.
	Scheme string
}

// Build returns the graph of the workspace or project plus every local Swift package
// they reference. candidates is the scheme's test target list, or nil when no scheme
// was given or found.
func Build(opts Options) (g *graph.Graph, candidates []string, err error) {
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, nil, err
	}
	projectPath := opts.ProjectPath
	if !filepath.IsAbs(projectPath) {
		projectPath = filepath.Join(root, projectPath)
	}

	var projectPaths, packageDirs, containers []string
	switch {
	case strings.HasSuffix(projectPath, ".xcworkspace"):
		ws, err := ParseWorkspace(projectPath)
		if err != nil {
			return nil, nil, err
		}
		projectPaths = ws.Projects
		packageDirs = ws.PackageDirs
		containers = append([]string{projectPath}, ws.Projects...)
	case strings.HasSuffix(projectPath, ".xcodeproj"):
		projectPaths = []string{projectPath}
		containers = []string{projectPath}
	default:
		return nil, nil, fmt.Errorf("project path must be an .xcworkspace or .xcodeproj: %s", opts.ProjectPath)
	}

	g = graph.New()
	targetNodeByID := map[string]string{}   // pbxproj target ID → node ID
	nodeByProductFile := map[string]string{} // "Core.framework" → node ID
	nodeByName := map[string][]string{}      // target name → node IDs
	var projects []*Project

	for _, pp := range projectPaths {
		// CocoaPods' generated project is not source the user changes.
		if filepath.Base(pp) == "Pods.xcodeproj" {
			continue
		}
		p, err := ParseProject(pp)
		if err != nil {
			return nil, nil, err
		}
		projects = append(projects, p)
		packageDirs = append(packageDirs, p.LocalPackages...)
		// Any edit to the project file (a file added to a target, a setting changed)
		// can affect every target in it, and which one is not knowable without a diff.
		pbxproj, _ := relTo(root, filepath.Join(p.Path, "project.pbxproj"))
		for _, t := range p.Targets {
			id := "xcode:" + p.Name + "/" + t.Name
			n := &graph.Node{
				ID:        id,
				Name:      t.Name,
				Kind:      graph.KindSource,
				Ecosystem: "xcode",
				Files:     map[string]bool{},
				Meta:      map[string]string{"project": p.Name, "product_type": t.ProductType, "product": t.ProductFile},
			}
			if t.IsTest {
				n.Kind = graph.KindTest
			}
			if pbxproj != "" {
				n.Files[pbxproj] = true
			}
			for _, f := range t.Files {
				if rel, ok := relTo(root, f); ok {
					n.Files[rel] = true
				}
			}
			for _, d := range t.Dirs {
				if rel, ok := relTo(root, d); ok {
					n.Dirs = append(n.Dirs, rel)
				}
			}
			g.Add(n)
			targetNodeByID[t.ID] = id
			if t.ProductFile != "" {
				nodeByProductFile[t.ProductFile] = id
			}
			nodeByName[t.Name] = append(nodeByName[t.Name], id)
		}
	}

	// A package that contains the project directory (Package.swift at the repository
	// root, the app in a subfolder) is not always referenced from the project file.
	for _, pp := range projectPaths {
		for dir := filepath.Dir(pp); strings.HasPrefix(dir, root); dir = filepath.Dir(dir) {
			if _, err := os.Stat(filepath.Join(dir, "Package.swift")); err == nil {
				packageDirs = append(packageDirs, dir)
			}
			if dir == root {
				break
			}
		}
	}
	packages, err := loadPackages(packageDirs)
	if err != nil {
		return nil, nil, err
	}
	spmNodeByTarget := map[string]string{}     // "pkg/target" → node ID
	spmNodesByProduct := map[string][]string{} // product name → node IDs
	spmNodesByName := map[string][]string{}    // target name → node IDs
	for _, pkg := range packages {
		for _, t := range pkg.Targets {
			id := "spm:" + pkg.Name + "/" + t.Name
			n := &graph.Node{ID: id, Name: t.Name, Kind: graph.KindSource, Ecosystem: "spm", Meta: map[string]string{"package": pkg.Name}}
			if t.IsTest {
				n.Kind = graph.KindTest
			}
			if rel, ok := relTo(root, t.Path); ok {
				n.Dirs = []string{rel}
			}
			// The manifest itself belongs to every target of the package.
			if rel, ok := relTo(root, filepath.Join(pkg.Dir, "Package.swift")); ok {
				n.Files = map[string]bool{rel: true}
			}
			g.Add(n)
			spmNodeByTarget[pkg.Name+"/"+t.Name] = id
			spmNodesByName[t.Name] = append(spmNodesByName[t.Name], id)
		}
		for product, targets := range pkg.Products {
			for _, t := range targets {
				if id, ok := spmNodeByTarget[pkg.Name+"/"+t]; ok {
					spmNodesByProduct[product] = append(spmNodesByProduct[product], id)
				}
			}
		}
	}

	// Edges between Xcode targets.
	for _, p := range projects {
		for _, t := range p.Targets {
			from := "xcode:" + p.Name + "/" + t.Name
			for _, depID := range t.DependencyIDs {
				if to, ok := targetNodeByID[depID]; ok {
					g.AddDep(from, to)
				}
			}
			for _, prod := range t.LinkedProducts {
				if to, ok := nodeByProductFile[prod]; ok {
					g.AddDep(from, to)
				}
			}
			for _, prod := range t.PackageProducts {
				for _, to := range spmNodesByProduct[prod] {
					g.AddDep(from, to)
				}
				// Xcode also accepts a target name as the product for local packages.
				if len(spmNodesByProduct[prod]) == 0 {
					for _, to := range spmNodesByName[prod] {
						g.AddDep(from, to)
					}
				}
			}
			if t.TestHostProduct != "" {
				for _, to := range nodeByName[t.TestHostProduct] {
					g.AddDep(from, to)
				}
				if to, ok := nodeByProductFile[t.TestHostProduct+".app"]; ok {
					g.AddDep(from, to)
				}
			}
			if t.TestTargetName != "" {
				for _, to := range nodeByName[t.TestTargetName] {
					g.AddDep(from, to)
				}
			}
		}
	}

	// Edges inside and between packages.
	for _, pkg := range packages {
		for _, t := range pkg.Targets {
			from := spmNodeByTarget[pkg.Name+"/"+t.Name]
			for _, dep := range t.TargetDeps {
				if to, ok := spmNodeByTarget[pkg.Name+"/"+dep]; ok {
					g.AddDep(from, to)
					continue
				}
				for _, to := range spmNodesByProduct[dep] {
					g.AddDep(from, to)
				}
				for _, to := range spmNodesByName[dep] {
					g.AddDep(from, to)
				}
			}
			for _, dep := range t.ProductDeps {
				for _, to := range spmNodesByProduct[dep] {
					g.AddDep(from, to)
				}
			}
		}
	}
	g.Finalize()

	if opts.Scheme != "" {
		candidates, err = Testables(opts.Scheme, containers)
		if err != nil {
			return nil, nil, err
		}
	}
	return g, candidates, nil
}

// loadPackages parses every package directory and follows `.package(path:)`
// references to other local packages.
func loadPackages(dirs []string) ([]*Package, error) {
	seen := map[string]bool{}
	var out []*Package
	queue := append([]string{}, dirs...)
	for len(queue) > 0 {
		dir := queue[0]
		queue = queue[1:]
		abs, err := filepath.Abs(dir)
		if err != nil {
			return nil, err
		}
		if seen[abs] {
			continue
		}
		seen[abs] = true
		if _, err := os.Stat(filepath.Join(abs, "Package.swift")); err != nil {
			continue
		}
		pkg, err := ParsePackage(abs)
		if err != nil {
			return nil, err
		}
		out = append(out, pkg)
		queue = append(queue, pkg.LocalDependencies...)
	}
	return out, nil
}

// relTo returns p relative to root, or false when p is outside the repository.
func relTo(root, p string) (string, bool) {
	rel, err := filepath.Rel(root, p)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return filepath.ToSlash(rel), true
}
