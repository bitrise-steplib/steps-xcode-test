// Package graph holds the dependency graph the selection is computed on.
//
// A node is a build unit (Xcode target, SPM target, Gradle module, Flutter package)
// that owns files. Test nodes are the units the step selects; every other node is a
// source node. Ownership is resolved from explicit file membership first, then from
// glob rules, then from the longest matching directory prefix.
package graph

import (
	"path"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// Kind tells whether a node is a test unit or a source unit.
type Kind string

const (
	KindSource Kind = "source"
	KindTest   Kind = "test"
)

// Node is one build unit.
type Node struct {
	ID        string
	Name      string
	Kind      Kind
	Ecosystem string
	// Dirs are repo-relative directories owned by the node (prefix match).
	Dirs []string
	// Files are repo-relative files explicitly owned by the node.
	Files map[string]bool
	// Globs are ownership patterns used by manual rules.
	Globs []string
	// Deps are IDs of the nodes this node depends on.
	Deps []string
	// Meta carries ecosystem-specific details for the output (product, project, module task).
	Meta map[string]string
}

// Graph is a set of nodes with dependency edges.
type Graph struct {
	nodes   map[string]*Node
	reverse map[string][]string
}

// New returns an empty graph.
func New() *Graph {
	return &Graph{nodes: map[string]*Node{}, reverse: map[string][]string{}}
}

// Add inserts a node, merging with an existing node of the same ID.
func (g *Graph) Add(n *Node) {
	if n.Files == nil {
		n.Files = map[string]bool{}
	}
	if n.Meta == nil {
		n.Meta = map[string]string{}
	}
	for i, d := range n.Dirs {
		n.Dirs[i] = Normalize(d)
	}
	files := map[string]bool{}
	for f := range n.Files {
		files[Normalize(f)] = true
	}
	n.Files = files

	existing, ok := g.nodes[n.ID]
	if !ok {
		g.nodes[n.ID] = n
		return
	}
	existing.Dirs = append(existing.Dirs, n.Dirs...)
	for f := range n.Files {
		existing.Files[f] = true
	}
	existing.Globs = append(existing.Globs, n.Globs...)
	existing.Deps = append(existing.Deps, n.Deps...)
	for k, v := range n.Meta {
		existing.Meta[k] = v
	}
	if n.Kind == KindTest {
		existing.Kind = KindTest
	}
}

// AddDep records that from depends on to. Unknown IDs are kept; they are ignored at query time.
func (g *Graph) AddDep(from, to string) {
	n, ok := g.nodes[from]
	if !ok || from == to {
		return
	}
	n.Deps = append(n.Deps, to)
}

// Node returns the node with the given ID, or nil.
func (g *Graph) Node(id string) *Node {
	return g.nodes[id]
}

// Nodes returns every node sorted by ID.
func (g *Graph) Nodes() []*Node {
	out := make([]*Node, 0, len(g.nodes))
	for _, n := range g.nodes {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// TestNodes returns the test nodes sorted by ID.
func (g *Graph) TestNodes() []*Node {
	var out []*Node
	for _, n := range g.Nodes() {
		if n.Kind == KindTest {
			out = append(out, n)
		}
	}
	return out
}

// Merge copies every node of other into g.
func (g *Graph) Merge(other *Graph) {
	for _, n := range other.Nodes() {
		g.Add(n)
	}
}

// Finalize dedupes edges and builds the reverse index. Call it after the last Add.
func (g *Graph) Finalize() {
	g.reverse = map[string][]string{}
	for _, n := range g.nodes {
		n.Deps = dedupe(n.Deps)
		n.Dirs = dedupe(n.Dirs)
		n.Globs = dedupe(n.Globs)
		for _, d := range n.Deps {
			if _, ok := g.nodes[d]; ok {
				g.reverse[d] = append(g.reverse[d], n.ID)
			}
		}
	}
}

// Owners returns the IDs of the nodes that own the file. Explicit membership wins,
// then glob rules, then the nodes with the longest matching directory prefix.
func (g *Graph) Owners(file string) []string {
	file = Normalize(file)
	var out []string
	for _, n := range g.Nodes() {
		if n.Files[file] {
			out = append(out, n.ID)
		}
	}
	if len(out) > 0 {
		return out
	}
	for _, n := range g.Nodes() {
		for _, glob := range n.Globs {
			if ok, _ := doublestar.Match(glob, file); ok {
				out = append(out, n.ID)
				break
			}
		}
	}
	if len(out) > 0 {
		return out
	}
	best := -1
	for _, n := range g.Nodes() {
		for _, d := range n.Dirs {
			l, ok := prefixLen(d, file)
			if !ok {
				continue
			}
			if l > best {
				best = l
				out = out[:0]
			}
			if l == best {
				out = append(out, n.ID)
			}
		}
	}
	return dedupe(out)
}

// Dependents returns the transitive reverse closure of the given node IDs, including
// the IDs themselves. Unknown IDs are dropped.
func (g *Graph) Dependents(ids []string) map[string]bool {
	seen := map[string]bool{}
	var queue []string
	for _, id := range ids {
		if _, ok := g.nodes[id]; ok && !seen[id] {
			seen[id] = true
			queue = append(queue, id)
		}
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, up := range g.reverse[cur] {
			if !seen[up] {
				seen[up] = true
				queue = append(queue, up)
			}
		}
	}
	return seen
}

// Normalize turns a path into the repo-relative form used for matching: forward
// slashes, no leading "./", no trailing slash, "." for the root.
func Normalize(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	p = path.Clean(p)
	p = strings.TrimPrefix(p, "./")
	if p == "" || p == "/" {
		return "."
	}
	return p
}

// prefixLen reports whether dir is an ancestor of (or equal to) file, and the
// matched length. The repo root (".") matches everything with length 0.
func prefixLen(dir, file string) (int, bool) {
	if dir == "." {
		return 0, true
	}
	if file == dir {
		return len(dir), true
	}
	if strings.HasPrefix(file, dir+"/") {
		return len(dir), true
	}
	return 0, false
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
