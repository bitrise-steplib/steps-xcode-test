// Package rules holds the file-level and test-level safety rules: files whose change
// forces a full run, files that never affect tests, and tests that always run.
package rules

import (
	"path"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// DefaultFullRun lists files that influence every test.
//
// Pattern forms: a leading slash anchors the pattern to the repository root; a pattern
// without a slash matches by base name anywhere in the tree; anything else is a
// doublestar glob on the full path.
//
// Root Gradle files are anchored on purpose: a module's build.gradle belongs to the
// module, and when the Gradle build lives in a subdirectory its root files have no
// owner, which the unknown-file rule already turns into a full run.
var DefaultFullRun = []string{
	"/bitrise.yml",
	"Podfile",
	"Podfile.lock",
	"Package.resolved",
	"Cartfile",
	"Cartfile.resolved",
	"*.xcconfig",
	"*.xctestplan",
	"/build.gradle",
	"/build.gradle.kts",
	"/settings.gradle",
	"/settings.gradle.kts",
	"/gradle.properties",
	"/gradle/**",
	"/gradlew",
	"/gradlew.bat",
	"/buildSrc/**",
	"/build-logic/**",
	"pubspec.lock",
	"Gemfile",
	"Gemfile.lock",
	".ruby-version",
	".tool-versions",
	"Brewfile",
	"Brewfile.lock.json",
	"/Tuist.swift",
	"/Tuist/**",
	"/Project.swift",
	"/Workspace.swift",
	"/project.yml",
}

// DefaultIgnore lists files that never affect a test run: documentation, repository
// metadata, lint configuration, and the configuration of other CI systems.
var DefaultIgnore = []string{
	"*.md",
	"*.markdown",
	"docs/**",
	"LICENSE",
	"LICENSE.*",
	"CHANGELOG*",
	"CODEOWNERS",
	".gitignore",
	".gitattributes",
	".editorconfig",
	".swiftlint.yml",
	".swiftformat",
	"Dangerfile",
	"Dangerfile.*",
	".github/**",
	".circleci/**",
	".ci/**",
	".ci.yaml",
	".gitlab-ci.yml",
	".travis.yml",
	"Jenkinsfile",
	"azure-pipelines.yml",
	"codemagic.yaml",
}

// Rules is the merged rule set for one run.
type Rules struct {
	FullRun []string
	Ignore  []string
	MustRun []string
}

// Merge returns a rule set with the defaults and every extra pattern.
func Merge(fullRun, ignore, mustRun []string) Rules {
	return Rules{
		FullRun: append(append([]string{}, DefaultFullRun...), clean(fullRun)...),
		Ignore:  append(append([]string{}, DefaultIgnore...), clean(ignore)...),
		MustRun: clean(mustRun),
	}
}

// MatchFullRun returns the first full-run pattern the file matches.
func (r Rules) MatchFullRun(file string) (string, bool) {
	return matchAny(r.FullRun, file)
}

// MatchIgnore reports whether the file is ignored.
func (r Rules) MatchIgnore(file string) bool {
	_, ok := matchAny(r.Ignore, file)
	return ok
}

// MatchMustRun reports whether a test name matches a must-run pattern. Patterns are
// globs on the test's name, or on its ID when they contain a colon.
func (r Rules) MatchMustRun(id, name string) (string, bool) {
	for _, p := range r.MustRun {
		target := name
		if strings.Contains(p, ":") {
			target = id
		}
		if ok, _ := doublestar.Match(p, target); ok {
			return p, true
		}
	}
	return "", false
}

func matchAny(patterns []string, file string) (string, bool) {
	file = strings.TrimPrefix(file, "./")
	base := path.Base(file)
	for _, p := range patterns {
		if anchored, ok := strings.CutPrefix(p, "/"); ok {
			if ok, _ := doublestar.Match(anchored, file); ok {
				return p, true
			}
			continue
		}
		if ok, _ := doublestar.Match(p, file); ok {
			return p, true
		}
		if !strings.Contains(p, "/") {
			if ok, _ := doublestar.Match(p, base); ok {
				return p, true
			}
		}
	}
	return "", false
}

func clean(in []string) []string {
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		out = append(out, s)
	}
	return out
}

// SplitLines splits a newline- or comma-separated input into trimmed entries.
func SplitLines(raw string) []string {
	var out []string
	for _, line := range strings.FieldsFunc(raw, func(r rune) bool { return r == '\n' || r == ',' }) {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
