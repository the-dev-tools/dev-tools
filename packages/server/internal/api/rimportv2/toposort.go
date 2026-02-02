package rimportv2

import (
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

// ErrCycleDetected is returned when a cycle is detected in the dependency graph.
var ErrCycleDetected = fmt.Errorf("cycle detected in dependency graph")

// TopologicalSort sorts entities so parents come before children using Kahn's algorithm.
//
// This enables handling arbitrary depth parent-child relationships (delta chains of any length).
// Entities with external parents (parent ID not in the current batch) are treated as roots,
// since their parents are assumed to already exist in the database.
//
// Parameters:
//   - entities: slice of entities to sort
//   - getID: function to extract the ID from an entity
//   - getParentID: function to extract the parent ID from an entity (nil if no parent)
//
// Returns:
//   - sorted slice of entities (parents before children)
//   - error if a cycle is detected
func TopologicalSort[T any](
	entities []T,
	getID func(T) idwrap.IDWrap,
	getParentID func(T) *idwrap.IDWrap,
) ([]T, error) {
	if len(entities) == 0 {
		return entities, nil
	}

	// Build index of entities in this batch
	entityIndex := make(map[idwrap.IDWrap]int, len(entities))
	for i, e := range entities {
		entityIndex[getID(e)] = i
	}

	// Calculate in-degree for each entity (number of parents in this batch)
	// Build adjacency list: parent -> children
	inDegree := make([]int, len(entities))
	children := make([][]int, len(entities))

	for i := range children {
		children[i] = make([]int, 0)
	}

	for i, e := range entities {
		parentID := getParentID(e)
		if parentID == nil {
			// No parent - this is a root
			continue
		}

		// Check if parent is in this batch
		if parentIdx, inBatch := entityIndex[*parentID]; inBatch {
			// Parent is in this batch - add edge and increment in-degree
			inDegree[i]++
			children[parentIdx] = append(children[parentIdx], i)
		}
		// If parent is not in batch, treat this entity as a root (parent assumed to exist in DB)
	}

	// Initialize queue with all roots (entities with in-degree 0)
	queue := make([]int, 0, len(entities))
	for i, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, i)
		}
	}

	// BFS: process roots, then their children as they become roots
	result := make([]T, 0, len(entities))
	processed := 0

	for len(queue) > 0 {
		// Dequeue
		idx := queue[0]
		queue = queue[1:]

		result = append(result, entities[idx])
		processed++

		// For each child, decrement in-degree
		for _, childIdx := range children[idx] {
			inDegree[childIdx]--
			if inDegree[childIdx] == 0 {
				queue = append(queue, childIdx)
			}
		}
	}

	// If we didn't process all entities, there's a cycle
	if processed != len(entities) {
		return nil, ErrCycleDetected
	}

	return result, nil
}

// TopologicalSortWithFallback sorts entities topologically, falling back to original order on cycle.
//
// This is useful when cycles shouldn't occur in valid data but we want to be defensive.
// If a cycle is detected:
//  1. A warning is logged (via the provided logger callback)
//  2. The original order is returned (best effort)
//
// Parameters:
//   - entities: slice of entities to sort
//   - getID: function to extract the ID from an entity
//   - getParentID: function to extract the parent ID from an entity (nil if no parent)
//   - onCycle: optional callback invoked when a cycle is detected (can be nil)
//
// Returns:
//   - sorted slice of entities (parents before children), or original order if cycle detected
func TopologicalSortWithFallback[T any](
	entities []T,
	getID func(T) idwrap.IDWrap,
	getParentID func(T) *idwrap.IDWrap,
	onCycle func(entities []T),
) []T {
	sorted, err := TopologicalSort(entities, getID, getParentID)
	if err != nil {
		if onCycle != nil {
			onCycle(entities)
		}
		// Return a copy of the original slice to avoid mutation issues
		result := make([]T, len(entities))
		copy(result, entities)
		return result
	}
	return sorted
}
