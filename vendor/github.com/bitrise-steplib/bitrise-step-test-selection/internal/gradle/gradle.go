// Package gradle builds the module graph of a Gradle build. The graph is dumped by an
// init script during Gradle's configuration phase, so the step needs no knowledge of
// the build files' syntax.
package gradle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitrise-steplib/bitrise-step-test-selection/internal/graph"
)

// InitScript registers a task that prints every project, its source directories and
// its project dependencies as JSON. It is Groovy so that it works for Groovy and
// Kotlin builds alike.
const InitScript = `import groovy.json.JsonOutput
import org.gradle.api.artifacts.ProjectDependency

gradle.rootProject { root ->
    def task = root.tasks.register("bitriseTestSelectionGraph") {
        doLast {
            def rootDir = root.projectDir
            def rel = { File f -> rootDir.toPath().relativize(f.toPath().toAbsolutePath()).toString().replace('\\', '/') }
            def projects = []
            root.allprojects { p ->
                def mainDirs = new LinkedHashSet()
                def testDirs = new LinkedHashSet()
                def classify = { String name, Collection dirs ->
                    def n = name.toLowerCase()
                    if (name == "testFixtures" || !(n.contains("test"))) mainDirs.addAll(dirs) else testDirs.addAll(dirs)
                }
                def ss = p.extensions.findByName("sourceSets")
                if (ss != null) {
                    try { ss.each { s -> classify(s.name, s.allSource.srcDirs.collect { rel(it) }) } } catch (ignored) { }
                }
                def android = p.extensions.findByName("android")
                if (android != null) {
                    try {
                        android.sourceSets.each { s ->
                            def dirs = []
                            ["java", "kotlin", "res", "assets", "resources", "aidl", "renderscript", "jni", "jniLibs", "shaders", "mlModels"].each { k ->
                                try { dirs.addAll(s."$k".srcDirs.collect { rel(it) }) } catch (ignored) { }
                            }
                            try { dirs.add(rel(s.manifest.srcFile)) } catch (ignored) { }
                            classify(s.name, dirs)
                        }
                    } catch (ignored) { }
                }
                def deps = new LinkedHashSet()
                p.configurations.each { c ->
                    try {
                        c.dependencies.each { d ->
                            if (d instanceof ProjectDependency) {
                                def path
                                try { path = d.path } catch (ignored) { path = d.dependencyProject.path }
                                deps.add([path: path, configuration: c.name])
                            }
                        }
                    } catch (ignored) { }
                }
                projects.add([
                    path: p.path,
                    dir: rel(p.projectDir),
                    buildFile: p.buildFile.exists() ? rel(p.buildFile) : null,
                    android: android != null,
                    mainDirs: mainDirs.toList(),
                    testDirs: testDirs.toList(),
                    deps: deps.toList(),
                ])
            }
            def json = JsonOutput.toJson([rootDir: rootDir.absolutePath, projects: projects])
            def out = root.findProperty("bitriseTestSelectionOut")
            if (out) { new File(out.toString()).text = json } else { println json }
        }
    }
    try {
        task.configure { it.notCompatibleWithConfigurationCache("reads the project model at execution time") }
    } catch (ignored) { }
}
`

// Dump is the JSON the init script writes.
type Dump struct {
	RootDir  string    `json:"rootDir"`
	Projects []Project `json:"projects"`
}

// Project is one Gradle project in the dump.
type Project struct {
	Path      string `json:"path"`
	Dir       string `json:"dir"`
	BuildFile string `json:"buildFile"`
	Android   bool     `json:"android"`
	MainDirs  []string `json:"mainDirs"`
	TestDirs  []string `json:"testDirs"`
	Deps      []Dep    `json:"deps"`
}

// Dep is a project dependency with the configuration it was declared in.
type Dep struct {
	Path          string `json:"path"`
	Configuration string `json:"configuration"`
}

// Runner runs a command in a directory and returns its combined output.
type Runner interface {
	Run(dir, name string, args ...string) (string, error)
}

// Options locate the Gradle build.
type Options struct {
	// Root is the repository root.
	Root string
	// ProjectDir is the Gradle root project directory, relative to Root.
	ProjectDir string
	// Gradlew is the wrapper path, relative to ProjectDir. Defaults to ./gradlew.
	Gradlew string
	// ExtraArgs are appended to the Gradle invocation.
	ExtraArgs []string
	// AndroidUnitTestTask is the unit test task name for Android modules; the plain
	// `test` lifecycle task exists on every module, variant-specific ones only when the
	// variant does. Defaults to "test".
	AndroidUnitTestTask string
}

// Build runs the init script and converts the dump into a graph.
func Build(opts Options, run Runner) (*graph.Graph, error) {
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, err
	}
	projectDir := filepath.Join(root, opts.ProjectDir)
	gradlew := opts.Gradlew
	if gradlew == "" {
		gradlew = "./gradlew"
	}
	if !filepath.IsAbs(gradlew) {
		gradlew = filepath.Join(projectDir, gradlew)
	}
	if _, err := os.Stat(gradlew); err != nil {
		return nil, fmt.Errorf("gradle wrapper not found at %s", gradlew)
	}

	tmp, err := os.MkdirTemp("", "bitrise-test-selection-gradle")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	initPath := filepath.Join(tmp, "graph.init.gradle")
	if err := os.WriteFile(initPath, []byte(InitScript), 0o644); err != nil {
		return nil, err
	}
	outPath := filepath.Join(tmp, "graph.json")

	args := []string{
		"--quiet",
		"--init-script", initPath,
		"-PbitriseTestSelectionOut=" + outPath,
		// The dump reads every project's model from one task, which isolated projects
		// forbids. The configuration cache is handled by marking the task incompatible;
		// disabling it here would fail on builds that enable isolated projects.
		"-Dorg.gradle.unsafe.isolated-projects=false",
	}
	args = append(args, opts.ExtraArgs...)
	args = append(args, "bitriseTestSelectionGraph")
	if out, err := run.Run(projectDir, gradlew, args...); err != nil {
		return nil, fmt.Errorf("gradle graph dump failed: %w\n%s", err, tail(out, 40))
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		return nil, fmt.Errorf("gradle graph dump produced no output: %w", err)
	}
	return BuildFromDumpWithTask(root, projectDir, data, opts.AndroidUnitTestTask)
}

// BuildFromDump converts the init script's JSON into a graph. projectDir is the
// absolute Gradle root; dump paths are relative to it.
func BuildFromDump(root, projectDir string, data []byte) (*graph.Graph, error) {
	return BuildFromDumpWithTask(root, projectDir, data, "test")
}

// BuildFromDumpWithTask is BuildFromDump with the Android unit test task name.
func BuildFromDumpWithTask(root, projectDir string, data []byte, androidUnitTestTask string) (*graph.Graph, error) {
	if androidUnitTestTask == "" {
		androidUnitTestTask = "test"
	}
	var dump Dump
	if err := json.Unmarshal(data, &dump); err != nil {
		return nil, fmt.Errorf("parse gradle graph dump: %w", err)
	}
	rel := func(p string) (string, bool) {
		if p == "" {
			return "", false
		}
		r, err := filepath.Rel(root, filepath.Join(projectDir, p))
		if err != nil || strings.HasPrefix(r, "..") {
			return "", false
		}
		return filepath.ToSlash(r), true
	}
	exists := func(p string) bool {
		_, err := os.Stat(filepath.Join(projectDir, p))
		return err == nil
	}

	g := graph.New()
	for _, p := range dump.Projects {
		moduleID := "gradle:" + p.Path
		module := &graph.Node{
			ID:        moduleID,
			Name:      p.Path,
			Kind:      graph.KindSource,
			Ecosystem: "gradle",
			Files:     map[string]bool{},
			Meta:      map[string]string{"module": p.Path},
		}
		if p.Android {
			module.Meta["android"] = "true"
		}
		// The root project would otherwise own every unclassified file in the repository.
		if p.Path != ":" {
			if d, ok := rel(p.Dir); ok {
				module.Dirs = append(module.Dirs, d)
			}
		}
		for _, d := range p.MainDirs {
			if r, ok := rel(d); ok {
				module.Dirs = append(module.Dirs, r)
			}
		}
		if bf, ok := rel(p.BuildFile); ok && p.Path != ":" {
			module.Files[bf] = true
		}
		g.Add(module)

		var unitDirs, androidDirs []string
		for _, d := range p.TestDirs {
			r, ok := rel(d)
			if !ok || !exists(d) {
				continue
			}
			if strings.Contains(strings.ToLower(filepath.Base(d)), "androidtest") || strings.Contains(d, "/androidTest") {
				androidDirs = append(androidDirs, r)
			} else {
				unitDirs = append(unitDirs, r)
			}
		}
		if len(unitDirs) > 0 {
			id := moduleID + ":unitTest"
			task := p.Path + ":test"
			if p.Android {
				task = p.Path + ":" + androidUnitTestTask
			}
			g.Add(&graph.Node{ID: id, Name: p.Path + " unit tests", Kind: graph.KindTest, Ecosystem: "gradle", Dirs: unitDirs, Deps: []string{moduleID}, Meta: map[string]string{"module": p.Path, "task": task, "kind": "unit"}})
		}
		if len(androidDirs) > 0 {
			id := moduleID + ":androidTest"
			g.Add(&graph.Node{ID: id, Name: p.Path + " instrumented tests", Kind: graph.KindTest, Ecosystem: "gradle", Dirs: androidDirs, Deps: []string{moduleID}, Meta: map[string]string{"module": p.Path, "task": p.Path + ":connectedAndroidTest", "kind": "instrumented"}})
		}
	}

	for _, p := range dump.Projects {
		moduleID := "gradle:" + p.Path
		for _, d := range p.Deps {
			to := "gradle:" + d.Path
			conf := strings.ToLower(d.Configuration)
			switch {
			case strings.HasPrefix(conf, "androidtest"):
				g.AddDep(moduleID+":androidTest", to)
			case strings.HasPrefix(conf, "test") || strings.Contains(conf, "unittest"):
				g.AddDep(moduleID+":unitTest", to)
			default:
				g.AddDep(moduleID, to)
			}
		}
	}
	g.Finalize()
	return g, nil
}

func tail(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
