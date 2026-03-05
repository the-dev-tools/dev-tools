//nolint:revive // exported
package mflow

import (
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

type NodeKind = int32

const (
	NODE_KIND_UNSPECIFIED  NodeKind = 0
	NODE_KIND_MANUAL_START NodeKind = 1
	NODE_KIND_REQUEST      NodeKind = 2
	NODE_KIND_CONDITION    NodeKind = 3
	NODE_KIND_FOR          NodeKind = 4
	NODE_KIND_FOR_EACH     NodeKind = 5
	NODE_KIND_JS           NodeKind = 6
	NODE_KIND_AI           NodeKind = 7
	NODE_KIND_AI_PROVIDER  NodeKind = 8
	NODE_KIND_AI_MEMORY    NodeKind = 9
	NODE_KIND_GRAPHQL       NodeKind = 10
	NODE_KIND_WS_CONNECTION NodeKind = 11
	NODE_KIND_WS_SEND       NodeKind = 12
	NODE_KIND_WAIT          NodeKind = 13
	// NODE_KIND_WEBHOOK NodeKind = 14 // reserved for future trigger entry
)

type NodeState = int8

const (
	NODE_STATE_UNSPECIFIED NodeState = 0
	NODE_STATE_RUNNING     NodeState = 1
	NODE_STATE_SUCCESS     NodeState = 2
	NODE_STATE_FAILURE     NodeState = 3
	NODE_STATE_CANCELED    NodeState = 4
)

func StringNodeState(a NodeState) string {
	switch a {
	case NODE_STATE_UNSPECIFIED:
		return "Unspecified"
	case NODE_STATE_RUNNING:
		return "Running"
	case NODE_STATE_SUCCESS:
		return "Success"
	case NODE_STATE_FAILURE:
		return "Failure"
	case NODE_STATE_CANCELED:
		return "Canceled"
	default:
		return "Unknown"
	}
}

func StringNodeStateWithIcons(a NodeState) string {
	switch a {
	case NODE_STATE_UNSPECIFIED:
		return "🔄 Starting"
	case NODE_STATE_RUNNING:
		return "⏳ Running"
	case NODE_STATE_SUCCESS:
		return "✅ Success"
	case NODE_STATE_FAILURE:
		return "❌ Failed"
	case NODE_STATE_CANCELED:
		return "Canceled"
	default:
		return "❓ Unknown"
	}
}

type Node struct {
	ID        idwrap.IDWrap
	FlowID    idwrap.IDWrap
	Name      string
	NodeKind  NodeKind
	PositionX float64
	PositionY float64
	State     NodeState
}
