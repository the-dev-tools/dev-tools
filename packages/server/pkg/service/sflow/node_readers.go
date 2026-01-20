package sflow

import (
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
)

// NodeReaders bundles all node implementation readers for convenient access.
// Use this when you need to read multiple node types (e.g., in import/export,
// flow execution, or tests).
type NodeReaders struct {
	JS      *NodeJsReader
	If      *NodeIfReader
	For     *NodeForReader
	ForEach *NodeForEachReader
	AI      *NodeAIReader
}

// NewNodeReaders creates all node readers from a queries instance.
func NewNodeReaders(q *gen.Queries) NodeReaders {
	return NodeReaders{
		JS:      NewNodeJsReaderFromQueries(q),
		If:      NewNodeIfReaderFromQueries(q),
		For:     NewNodeForReaderFromQueries(q),
		ForEach: NewNodeForEachReaderFromQueries(q),
		AI:      NewNodeAIReaderFromQueries(q),
	}
}
