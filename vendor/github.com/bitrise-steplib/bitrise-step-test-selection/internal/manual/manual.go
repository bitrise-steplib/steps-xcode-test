// Package manual builds a graph from user-written rules of the form
//
//	<glob> => <test name>[, <test name>...]
//
// for projects the built-in analysers do not understand. Each rule becomes a source
// node owning the files that match its glob; the named tests depend on it.
package manual

import (
	"fmt"
	"strings"

	"github.com/bitrise-steplib/bitrise-step-test-selection/internal/graph"
)

// Parse builds the graph from newline-separated rules. Blank lines and lines starting
// with # are ignored.
func Parse(raw string) (*graph.Graph, error) {
	g := graph.New()
	ruleIdx := 0
	for lineNo, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		glob, tests, ok := strings.Cut(line, "=>")
		if !ok {
			return nil, fmt.Errorf("manual rule on line %d has no '=>': %q", lineNo+1, line)
		}
		glob = strings.TrimSpace(glob)
		if glob == "" {
			return nil, fmt.Errorf("manual rule on line %d has an empty glob", lineNo+1)
		}
		ruleIdx++
		ruleID := fmt.Sprintf("manual:rule%d", ruleIdx)
		g.Add(&graph.Node{ID: ruleID, Name: glob, Kind: graph.KindSource, Ecosystem: "manual", Globs: []string{glob}})
		for _, name := range strings.Split(tests, ",") {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			id := "manual:" + name
			g.Add(&graph.Node{ID: id, Name: name, Kind: graph.KindTest, Ecosystem: "manual", Meta: map[string]string{"only_testing": name}})
			g.AddDep(id, ruleID)
		}
	}
	g.Finalize()
	return g, nil
}
