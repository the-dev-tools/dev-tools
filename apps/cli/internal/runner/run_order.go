package runner

import (
	"fmt"
	"sort"
	"strings"

	yamlflowsimplev2 "github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/yamlflowsimplev2"

	"gopkg.in/yaml.v3"
)

// runEntry is a parsed run: block entry: a flow name plus the names of the
// flows (also in the run: block) that must complete successfully first.
type runEntry struct {
	flowName  string
	dependsOn []string
}

// parseRunEntries extracts the run: block from the workflow file using the
// typed yamlflowsimplev2 structs, instead of re-parsing it by hand as
// map[string]interface{}. This gets depends_on's scalar-or-list form
// (StringOrSlice) for free and matches exactly how the rest of the yamlflow
// contract is parsed elsewhere.
func parseRunEntries(fileData []byte) ([]runEntry, error) {
	var doc struct {
		Run []yamlflowsimplev2.YamlRunEntryV2 `yaml:"run"`
	}
	if err := yaml.Unmarshal(fileData, &doc); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	entries := make([]runEntry, 0, len(doc.Run))
	for _, re := range doc.Run {
		if re.Flow == "" {
			continue
		}
		entries = append(entries, runEntry{
			flowName:  re.Flow,
			dependsOn: []string(re.DependsOn),
		})
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("no run field found in workflow")
	}

	return entries, nil
}

// topoSortRunEntries orders run: block entries so that every flow appears
// after all of its dependencies, using Kahn's algorithm. Ties (multiple
// flows simultaneously ready to run) are broken by original run: block
// order, so the result is deterministic for a given file.
//
// Returns an error naming the offending dependency if depends_on references
// a flow that is not itself part of the run: block, or naming an example
// cycle if the dependency graph is not a DAG.
func topoSortRunEntries(entries []runEntry) ([]runEntry, error) {
	byName := make(map[string]runEntry, len(entries))
	declOrder := make(map[string]int, len(entries))
	names := make([]string, 0, len(entries))
	for i, e := range entries {
		byName[e.flowName] = e
		declOrder[e.flowName] = i
		names = append(names, e.flowName)
	}

	for _, e := range entries {
		for _, dep := range e.dependsOn {
			if _, ok := byName[dep]; !ok {
				return nil, fmt.Errorf("unknown dependency %q in run block (known flows: %s)", dep, strings.Join(names, ", "))
			}
		}
	}

	// dependents[X] = flows that declare a dependency on X.
	inDegree := make(map[string]int, len(entries))
	dependents := make(map[string][]string, len(entries))
	for _, e := range entries {
		inDegree[e.flowName] = len(e.dependsOn)
		for _, dep := range e.dependsOn {
			dependents[dep] = append(dependents[dep], e.flowName)
		}
	}

	byDeclOrder := func(s []string) {
		sort.SliceStable(s, func(i, j int) bool { return declOrder[s[i]] < declOrder[s[j]] })
	}

	var ready []string
	for _, name := range names {
		if inDegree[name] == 0 {
			ready = append(ready, name)
		}
	}
	byDeclOrder(ready)

	sorted := make([]runEntry, 0, len(entries))
	for len(ready) > 0 {
		name := ready[0]
		ready = ready[1:]
		sorted = append(sorted, byName[name])

		newlyReady := append([]string(nil), dependents[name]...)
		byDeclOrder(newlyReady)
		for _, dependent := range newlyReady {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				ready = append(ready, dependent)
				byDeclOrder(ready)
			}
		}
	}

	if len(sorted) != len(entries) {
		remaining := make(map[string]bool)
		for _, name := range names {
			if inDegree[name] > 0 {
				remaining[name] = true
			}
		}
		cycle := findCycle(byName, remaining)
		return nil, fmt.Errorf("dependency cycle in run block: %s", strings.Join(cycle, " → "))
	}

	return sorted, nil
}

// findCycle returns one dependency cycle among the given remaining flows, as
// a path starting and ending on the same flow name (e.g. [A B A]). remaining
// is guaranteed non-empty and every entry in it sits on at least one cycle
// (Kahn's algorithm only leaves nodes behind when they are part of a cycle,
// or depend - transitively - on one). The starting node is chosen
// deterministically (smallest name) so the reported cycle is stable.
func findCycle(byName map[string]runEntry, remaining map[string]bool) []string {
	startNames := make([]string, 0, len(remaining))
	for name := range remaining {
		startNames = append(startNames, name)
	}
	sort.Strings(startNames)

	visited := make(map[string]bool)
	onPath := make(map[string]int)
	var path []string

	var visit func(name string) []string
	visit = func(name string) []string {
		if idx, ok := onPath[name]; ok {
			cycle := append([]string(nil), path[idx:]...)
			return append(cycle, name)
		}
		if visited[name] {
			return nil
		}
		visited[name] = true
		onPath[name] = len(path)
		path = append(path, name)

		deps := append([]string(nil), byName[name].dependsOn...)
		sort.Strings(deps)
		for _, dep := range deps {
			if !remaining[dep] {
				continue
			}
			if cycle := visit(dep); cycle != nil {
				return cycle
			}
		}

		path = path[:len(path)-1]
		delete(onPath, name)
		return nil
	}

	for _, name := range startNames {
		if cycle := visit(name); cycle != nil {
			return cycle
		}
	}

	// Unreachable: Kahn's algorithm guarantees every remaining node is part
	// of (or feeds into) a cycle, so visit() above always finds one.
	return startNames
}

// firstUnsuccessfulDependency returns the name of the first dependency (in
// declared order) that is missing a successful result, if any. "Missing" and
// "skipped" both count so that a skip cascades to transitive dependents.
func firstUnsuccessfulDependency(entry runEntry, statusByFlow map[string]string) (string, bool) {
	for _, dep := range entry.dependsOn {
		if status, ok := statusByFlow[dep]; !ok || !strings.EqualFold(status, "success") {
			return dep, true
		}
	}
	return "", false
}
