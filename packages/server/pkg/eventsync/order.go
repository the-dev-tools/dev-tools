package eventsync

import (
	"fmt"
	"sort"
)

// TopologicalSort computes the order in which event kinds should be published.
// It uses Kahn's algorithm to compute a topological ordering of the dependency graph.
// Returns an error if there's a cycle in the dependencies (should never happen with valid config).
func TopologicalSort(deps map[EventKind][]EventKind) ([]EventKind, error) {
	// Build reverse adjacency (what depends on each kind)
	dependents := make(map[EventKind][]EventKind)
	inDegree := make(map[EventKind]int)

	// Initialize all known kinds
	for kind := range deps {
		if _, exists := inDegree[kind]; !exists {
			inDegree[kind] = 0
		}
		for _, dep := range deps[kind] {
			if _, exists := inDegree[dep]; !exists {
				inDegree[dep] = 0
			}
			dependents[dep] = append(dependents[dep], kind)
			inDegree[kind]++
		}
	}

	// Find all roots (kinds with no dependencies)
	var queue []EventKind
	for kind, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, kind)
		}
	}

	// Sort roots for deterministic output
	sort.Slice(queue, func(i, j int) bool {
		return string(queue[i]) < string(queue[j])
	})

	// Kahn's algorithm
	var result []EventKind
	for len(queue) > 0 {
		// Pop from queue
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// Process dependents
		deps := dependents[current]
		// Sort for deterministic output
		sort.Slice(deps, func(i, j int) bool {
			return string(deps[i]) < string(deps[j])
		})

		for _, dependent := range deps {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	// Check for cycles
	if len(result) != len(inDegree) {
		return nil, fmt.Errorf("cycle detected in event dependencies")
	}

	return result, nil
}

// ComputeEventOrder returns the pre-computed event order based on Dependencies.
// This is cached at package init time for efficiency.
func ComputeEventOrder() []EventKind {
	order, err := TopologicalSort(Dependencies)
	if err != nil {
		// This should never happen with our static Dependencies map
		panic(fmt.Sprintf("invalid event dependencies: %v", err))
	}
	return order
}

// eventOrder is the pre-computed topological order of event kinds.
var eventOrder = ComputeEventOrder()

// kindPriority maps event kinds to their priority (index in eventOrder).
var kindPriority = make(map[EventKind]int)

func init() {
	for i, k := range eventOrder {
		kindPriority[k] = i
	}
}

// GetEventOrder returns the pre-computed event publishing order.
func GetEventOrder() []EventKind {
	return eventOrder
}

// GetEventPriority returns the priority (lower = earlier) for an event kind.
// Returns -1 if the kind is unknown.
func GetEventPriority(kind EventKind) int {
	if p, ok := kindPriority[kind]; ok {
		return p
	}
	return -1
}

// SortEventKinds sorts a slice of event kinds by their publish order.
func SortEventKinds(kinds []EventKind) {
	sort.Slice(kinds, func(i, j int) bool {
		return GetEventPriority(kinds[i]) < GetEventPriority(kinds[j])
	})
}

// ValidateDependencies checks that the Dependencies map has no cycles.
// This is useful for testing configuration.
func ValidateDependencies() error {
	_, err := TopologicalSort(Dependencies)
	return err
}
