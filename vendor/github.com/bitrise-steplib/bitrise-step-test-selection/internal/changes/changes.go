// Package changes produces the list of changed files a selection is computed from.
// The list comes from the environment when the backend supplied a complete one, and
// from git otherwise.
package changes

import (
	"fmt"
	"sort"
	"strings"
)

// Result is the changed-file set and where it came from.
type Result struct {
	Files    []string
	Complete bool
	// Origin is "env", "git" or "none".
	Origin string
	// Base is the commit the diff was taken against (git origin only).
	Base string
	// Detail explains how the set was obtained, for the log.
	Detail string
}

// FromList parses a newline-separated list, as the backend exports it.
func FromList(raw string, complete bool) Result {
	return Result{Files: normalizeList(strings.Split(raw, "\n")), Complete: complete, Origin: "env", Detail: "changed files supplied by the build environment"}
}

// Runner runs git in a directory.
type Runner interface {
	Run(dir string, args ...string) (string, error)
}

// Deepen steps used when the clone is shallow and the merge base is missing. The
// last step unshallows the repository.
var deepenSteps = []int{50, 200, 1000, 5000}

// FromGit diffs HEAD against the merge base with baseRef, deepening a shallow clone
// as needed. baseRef may be a branch name, a remote-tracking ref or a commit.
func FromGit(git Runner, repoDir, baseRef string) (Result, error) {
	if strings.TrimSpace(baseRef) == "" {
		return Result{}, fmt.Errorf("no base ref given")
	}
	baseSHA, err := resolveBase(git, repoDir, baseRef)
	if err != nil {
		return Result{}, err
	}

	mergeBase, err := findMergeBase(git, repoDir, baseSHA)
	if err != nil {
		return Result{}, err
	}

	out, err := git.Run(repoDir, "diff", "--name-status", "-M", "--no-color", mergeBase, "HEAD")
	if err != nil {
		return Result{}, fmt.Errorf("git diff: %w", err)
	}
	files := parseNameStatus(out)
	return Result{
		Files:    files,
		Complete: true,
		Origin:   "git",
		Base:     mergeBase,
		Detail:   fmt.Sprintf("git diff against merge base %s of %s", short(mergeBase), baseRef),
	}, nil
}

func resolveBase(git Runner, dir, ref string) (string, error) {
	candidates := []string{ref}
	if !strings.HasPrefix(ref, "origin/") && !strings.HasPrefix(ref, "refs/") {
		candidates = append(candidates, "origin/"+ref)
	}
	for _, c := range candidates {
		if sha, err := git.Run(dir, "rev-parse", "--verify", "--quiet", c+"^{commit}"); err == nil && sha != "" {
			return strings.TrimSpace(sha), nil
		}
	}
	// Shallow clones made by the git-clone step usually lack the base branch entirely.
	if _, err := git.Run(dir, "fetch", "--no-tags", "--depth=1", "origin", ref); err != nil {
		return "", fmt.Errorf("base ref %q not found locally and fetch failed: %w", ref, err)
	}
	sha, err := git.Run(dir, "rev-parse", "--verify", "--quiet", "FETCH_HEAD^{commit}")
	if err != nil || sha == "" {
		return "", fmt.Errorf("base ref %q could not be resolved after fetch", ref)
	}
	return strings.TrimSpace(sha), nil
}

func findMergeBase(git Runner, dir, baseSHA string) (string, error) {
	if mb, err := git.Run(dir, "merge-base", "HEAD", baseSHA); err == nil && mb != "" {
		return strings.TrimSpace(mb), nil
	}
	shallow, _ := git.Run(dir, "rev-parse", "--is-shallow-repository")
	if strings.TrimSpace(shallow) != "true" {
		return "", fmt.Errorf("no merge base between HEAD and %s", short(baseSHA))
	}
	for i, n := range deepenSteps {
		args := []string{"fetch", "--no-tags", "origin", fmt.Sprintf("--deepen=%d", n), "HEAD", baseSHA}
		if i == len(deepenSteps)-1 {
			args = []string{"fetch", "--no-tags", "--unshallow", "origin", "HEAD", baseSHA}
		}
		if _, err := git.Run(dir, args...); err != nil {
			// Some providers refuse fetching by SHA; retry with the bare deepen.
			if _, err2 := git.Run(dir, args[:len(args)-2]...); err2 != nil {
				return "", fmt.Errorf("deepening the shallow clone failed: %w", err)
			}
		}
		if mb, err := git.Run(dir, "merge-base", "HEAD", baseSHA); err == nil && mb != "" {
			return strings.TrimSpace(mb), nil
		}
	}
	return "", fmt.Errorf("no merge base between HEAD and %s even after unshallowing", short(baseSHA))
}

// parseNameStatus turns `git diff --name-status -M` output into a file list. Renames
// contribute both the old and the new path.
func parseNameStatus(out string) []string {
	var files []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}
		files = append(files, parts[1:]...)
	}
	return normalizeList(files)
}

func normalizeList(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range in {
		f = strings.TrimSpace(f)
		f = strings.TrimPrefix(f, "./")
		if f == "" || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

func short(sha string) string {
	if len(sha) > 10 {
		return sha[:10]
	}
	return sha
}
