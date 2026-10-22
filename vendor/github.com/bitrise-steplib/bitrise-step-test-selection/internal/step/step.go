// Package step wires the pieces together: changed files → rules → graph → selection.
package step

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bitrise-steplib/bitrise-step-test-selection/internal/changes"
	"github.com/bitrise-steplib/bitrise-step-test-selection/internal/flutter"
	"github.com/bitrise-steplib/bitrise-step-test-selection/internal/gradle"
	"github.com/bitrise-steplib/bitrise-step-test-selection/internal/graph"
	"github.com/bitrise-steplib/bitrise-step-test-selection/internal/manual"
	"github.com/bitrise-steplib/bitrise-step-test-selection/internal/rules"
	"github.com/bitrise-steplib/bitrise-step-test-selection/internal/selection"
	"github.com/bitrise-steplib/bitrise-step-test-selection/internal/xcode"
)

// Config holds the step inputs.
type Config struct {
	ChangedFiles         string `env:"changed_files"`
	ChangedFilesComplete string `env:"changed_files_complete"`
	BaseRef              string `env:"base_ref"`
	ProjectType          string `env:"project_type,opt[auto,xcode,gradle,flutter,manual]"`
	ProjectPath          string `env:"project_path"`
	Scheme               string `env:"scheme"`
	GradleProjectDir     string `env:"gradle_project_dir"`
	GradlewPath          string `env:"gradlew_path"`
	GradleOptions        string `env:"gradle_options"`
	GradleTestTask       string `env:"gradle_test_task"`
	FlutterProjectDir    string `env:"flutter_project_dir"`
	ManualRules          string `env:"manual_rules"`
	BackendConfig        string `env:"config"`
	FullRunPaths         string `env:"full_run_paths"`
	IgnorePaths          string `env:"ignore_paths"`
	MustRun              string `env:"must_run"`
	OnUnknownFile        string `env:"on_unknown_file,opt[full,ignore]"`
	Mode                 string `env:"mode,opt[enforce,shadow]"`
	OnError              string `env:"on_error,opt[full,fail]"`
	RepoDir              string `env:"repo_dir,dir"`
	OutputDir            string `env:"output_dir"`
	Verbose              bool   `env:"verbose,opt[true,false]"`
}

// backendConfig is the JSON the Bitrise backend injects, mirroring the inputs. Lists
// are unioned with the inputs; scalar settings win over the inputs so that rollout
// stays under backend control.
type backendConfig struct {
	Mode          string   `json:"mode"`
	MustRun       []string `json:"must_run"`
	FullRunPaths  []string `json:"full_run_paths"`
	IgnorePaths   []string `json:"ignore_paths"`
	OnUnknownFile string   `json:"on_unknown_file"`
	ForceFullRun  bool     `json:"force_full_run"`
	ForceReason   string   `json:"force_full_run_reason"`
	BaseRef       string   `json:"base_ref"`
}

// Deps are the step's side effects, replaceable in tests.
type Deps struct {
	Git    changes.Runner
	Cmd    gradle.Runner
	Logf   func(format string, v ...interface{})
	Debugf func(format string, v ...interface{})
}

// Run computes the selection. It only returns an error when the step must fail; every
// analysis problem is reported inside the result as a full run instead.
func Run(cfg Config, deps Deps) (selection.Result, error) {
	if deps.Logf == nil {
		deps.Logf = func(string, ...interface{}) {}
	}
	if deps.Debugf == nil {
		deps.Debugf = deps.Logf
	}
	root, err := filepath.Abs(cfg.RepoDir)
	if err != nil {
		return selection.Result{}, err
	}

	res := selection.Result{Version: selection.Version, Mode: selection.ModeSelected}
	fullRun := func(kind selection.ReasonKind, reason string) {
		if res.Mode != selection.ModeFull {
			res.Mode = selection.ModeFull
			res.Reason = reason
			res.ReasonKind = kind
		}
	}
	warn := func(format string, v ...interface{}) {
		msg := fmt.Sprintf(format, v...)
		res.Errors = append(res.Errors, msg)
		deps.Logf("warning: %s", msg)
	}

	var backend backendConfig
	if strings.TrimSpace(cfg.BackendConfig) != "" {
		if err := json.Unmarshal([]byte(cfg.BackendConfig), &backend); err != nil {
			warn("config JSON is invalid and was ignored: %v", err)
			backend = backendConfig{}
		}
	}
	mode := cfg.Mode
	if backend.Mode != "" {
		mode = backend.Mode
	}
	res.Shadow = mode == "shadow"
	onUnknown := cfg.OnUnknownFile
	if backend.OnUnknownFile != "" {
		onUnknown = backend.OnUnknownFile
	}
	baseRef := cfg.BaseRef
	if backend.BaseRef != "" {
		baseRef = backend.BaseRef
	}
	ruleSet := rules.Merge(
		append(rules.SplitLines(cfg.FullRunPaths), backend.FullRunPaths...),
		append(rules.SplitLines(cfg.IgnorePaths), backend.IgnorePaths...),
		append(rules.SplitLines(cfg.MustRun), backend.MustRun...),
	)
	if backend.ForceFullRun {
		reason := backend.ForceReason
		if reason == "" {
			reason = "full run forced by the build configuration"
		}
		fullRun(selection.ReasonForced, reason)
	}

	// 1. Changed files.
	changed := resolveChanges(cfg, baseRef, root, deps, warn)
	res.ChangedFiles = selection.ChangedFiles{Origin: changed.Origin, Complete: changed.Complete, Base: changed.Base, Files: changed.Files}
	if changed.Origin == "none" {
		fullRun(selection.ReasonNoChanges, changed.Detail)
	} else if !changed.Complete {
		fullRun(selection.ReasonIncompleteChanges, "the changed file list is incomplete: "+changed.Detail)
	}
	deps.Debugf("changed files: %d (%s)", len(changed.Files), changed.Detail)

	// 2. File rules.
	var toResolve []string
	for _, f := range changed.Files {
		o := selection.Owner{File: f}
		if p, ok := ruleSet.MatchFullRun(f); ok {
			o.FullRun = p
			fullRun(selection.ReasonGlobalFile, fmt.Sprintf("%s changed (rule %s)", f, p))
		} else if ruleSet.MatchIgnore(f) {
			o.Ignored = true
		} else {
			toResolve = append(toResolve, f)
		}
		res.Owners = append(res.Owners, o)
	}
	ownerIndex := map[string]int{}
	for i, o := range res.Owners {
		ownerIndex[o.File] = i
	}

	// 3. Graph.
	g, candidates, err := buildGraph(cfg, root, deps)
	if err != nil {
		if cfg.OnError == "fail" {
			return res, err
		}
		warn("%v", err)
		fullRun(selection.ReasonAnalysisFailed, "project analysis failed: "+err.Error())
		g = graph.New()
		g.Finalize()
	}

	// 4. Ownership and propagation.
	var ownerIDs []string
	closureByOwner := map[string]map[string]bool{}
	for _, f := range toResolve {
		owners := g.Owners(f)
		o := &res.Owners[ownerIndex[f]]
		if len(owners) == 0 {
			o.Unknown = true
			if onUnknown == "full" {
				fullRun(selection.ReasonUnknownFile, fmt.Sprintf("%s belongs to no known build unit", f))
			}
			continue
		}
		o.Nodes = owners
		for _, id := range owners {
			ownerIDs = append(ownerIDs, id)
			if _, ok := closureByOwner[id]; !ok {
				closureByOwner[id] = g.Dependents([]string{id})
			}
		}
	}
	affected := g.Dependents(ownerIDs)
	for id := range affected {
		res.Affected = append(res.Affected, id)
	}

	// 5. Decide per test unit.
	for _, n := range g.TestNodes() {
		if candidates != nil && (n.Ecosystem == "xcode" || n.Ecosystem == "spm") && !contains(candidates, n.Name) {
			continue
		}
		t := selection.Test{ID: n.ID, Name: n.Name, Ecosystem: n.Ecosystem, Ident: identifiers(n)}
		switch {
		case res.Mode == selection.ModeFull:
			t.Selected = true
			t.Reason = "full run"
		case affected[n.ID]:
			t.Selected = true
			t.Reason = explain(n.ID, res.Owners, closureByOwner, g)
		default:
			if p, ok := ruleSet.MatchMustRun(n.ID, n.Name); ok {
				t.Selected = true
				t.Reason = "must-run (" + p + ")"
			} else {
				t.Reason = "no changed dependency"
			}
		}
		res.Tests = append(res.Tests, t)
	}
	if res.Mode == selection.ModeSelected {
		res.Reason = fmt.Sprintf("%d changed files mapped onto the project graph", len(changed.Files))
		res.ReasonKind = selection.ReasonGraph
	}
	res.Finalize()
	if res.Mode == selection.ModeSelected && res.Stats.Candidates > 0 && res.Stats.Selected == 0 {
		deps.Logf("no test unit is affected by this change; the consuming steps decide what an empty selection means")
	}
	return res, nil
}

func resolveChanges(cfg Config, baseRef, root string, deps Deps, warn func(string, ...interface{})) changes.Result {
	fromEnv := changes.Result{Origin: "none", Detail: "no changed file list was supplied"}
	if strings.TrimSpace(cfg.ChangedFiles) != "" {
		fromEnv = changes.FromList(cfg.ChangedFiles, strings.EqualFold(strings.TrimSpace(cfg.ChangedFilesComplete), "true"))
		if fromEnv.Complete {
			return fromEnv
		}
		fromEnv.Detail = "the supplied changed file list is marked incomplete"
	}
	if baseRef == "" || deps.Git == nil {
		if fromEnv.Origin == "env" {
			warn("changed file list is incomplete and no base ref is set to diff against")
		}
		return fromEnv
	}
	fromGit, err := changes.FromGit(deps.Git, root, baseRef)
	if err != nil {
		warn("git diff against %s failed: %v", baseRef, err)
		if fromEnv.Origin == "env" {
			return fromEnv
		}
		return changes.Result{Origin: "none", Detail: "no changed file list and git diff failed: " + err.Error()}
	}
	return fromGit
}

// buildGraph runs every detected analyser and merges their graphs.
func buildGraph(cfg Config, root string, deps Deps) (*graph.Graph, []string, error) {
	auto := cfg.ProjectType == "" || cfg.ProjectType == "auto"
	gradleDir := filepath.Join(root, cfg.GradleProjectDir)
	gradlew := cfg.GradlewPath
	if gradlew == "" {
		gradlew = "gradlew"
	}
	if !filepath.IsAbs(gradlew) {
		gradlew = filepath.Join(gradleDir, gradlew)
	}
	flutterDir := filepath.Join(root, cfg.FlutterProjectDir)

	useXcode := cfg.ProjectType == "xcode" || (auto && cfg.ProjectPath != "" && (strings.HasSuffix(cfg.ProjectPath, ".xcworkspace") || strings.HasSuffix(cfg.ProjectPath, ".xcodeproj")))
	useGradle := cfg.ProjectType == "gradle" || (auto && fileExists(gradlew))
	useFlutter := cfg.ProjectType == "flutter" || (auto && hasPubspec(flutterDir))
	useManual := cfg.ProjectType == "manual" || strings.TrimSpace(cfg.ManualRules) != ""

	g := graph.New()
	var candidates []string
	var used []string
	if useXcode {
		xg, c, err := xcode.Build(xcode.Options{Root: root, ProjectPath: cfg.ProjectPath, Scheme: cfg.Scheme})
		if err != nil {
			return nil, nil, fmt.Errorf("xcode: %w", err)
		}
		g.Merge(xg)
		candidates = c
		used = append(used, "xcode")
		if cfg.Scheme != "" && c == nil {
			deps.Logf("scheme %s not found; every test target is a candidate", cfg.Scheme)
		}
	}
	if useGradle {
		if deps.Cmd == nil {
			return nil, nil, fmt.Errorf("gradle: no command runner")
		}
		gg, err := gradle.Build(gradle.Options{Root: root, ProjectDir: cfg.GradleProjectDir, Gradlew: gradlew, ExtraArgs: rules.SplitLines(strings.ReplaceAll(cfg.GradleOptions, " ", "\n")), AndroidUnitTestTask: cfg.GradleTestTask}, deps.Cmd)
		if err != nil {
			return nil, nil, fmt.Errorf("gradle: %w", err)
		}
		g.Merge(gg)
		used = append(used, "gradle")
	}
	if useFlutter {
		fg, err := flutter.Build(root, cfg.FlutterProjectDir)
		if err != nil {
			return nil, nil, fmt.Errorf("flutter: %w", err)
		}
		g.Merge(fg)
		used = append(used, "flutter")
	}
	if useManual {
		mg, err := manual.Parse(cfg.ManualRules)
		if err != nil {
			return nil, nil, fmt.Errorf("manual rules: %w", err)
		}
		g.Merge(mg)
		used = append(used, "manual")
	}
	if len(used) == 0 {
		return nil, nil, fmt.Errorf("no project detected: set project_path (Xcode), gradle_project_dir (Gradle), flutter_project_dir (Flutter) or manual_rules")
	}
	g.Finalize()
	deps.Logf("analysed: %s (%d build units, %d test units)", strings.Join(used, ", "), len(g.Nodes()), len(g.TestNodes()))
	return g, candidates, nil
}

// explain names the changed file and owner a test is reached from.
func explain(testID string, owners []selection.Owner, closures map[string]map[string]bool, g *graph.Graph) string {
	var reasons []string
	for _, o := range owners {
		for _, id := range o.Nodes {
			if closures[id][testID] {
				name := id
				if n := g.Node(id); n != nil {
					name = n.Name
				}
				if id == testID {
					reasons = append(reasons, "own file "+o.File)
				} else {
					reasons = append(reasons, "via "+name+" ← "+o.File)
				}
				break
			}
		}
		if len(reasons) >= 2 {
			break
		}
	}
	sort.Strings(reasons)
	return strings.Join(reasons, "; ")
}

func identifiers(n *graph.Node) selection.Identifiers {
	switch n.Ecosystem {
	case "xcode", "spm", "manual":
		return selection.Identifiers{OnlyTesting: n.Name}
	case "gradle":
		return selection.Identifiers{GradleModule: n.Meta["module"], GradleTask: n.Meta["task"]}
	case "flutter":
		return selection.Identifiers{PackageDir: n.Meta["package_dir"]}
	}
	return selection.Identifiers{}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// hasPubspec reports whether dir is, or shallowly contains, a Dart package. A
// monorepo keeps its packages two or three levels down without a root pubspec.
func hasPubspec(dir string) bool {
	if fileExists(filepath.Join(dir, "pubspec.yaml")) {
		return true
	}
	for _, pattern := range []string{"*/pubspec.yaml", "*/*/pubspec.yaml", "*/*/*/pubspec.yaml"} {
		matches, _ := filepath.Glob(filepath.Join(dir, pattern))
		for _, m := range matches {
			rel, _ := filepath.Rel(dir, m)
			top := strings.SplitN(filepath.ToSlash(rel), "/", 2)[0]
			switch top {
			case "ios", "android", "macos", "linux", "windows", "web", "build", ".dart_tool", "node_modules", "Pods":
				continue
			}
			return true
		}
	}
	return false
}
