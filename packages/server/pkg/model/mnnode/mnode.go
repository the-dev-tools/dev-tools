package mnnode

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
)

type NodeKind = int32

const (
	NODE_KIND_UNSPECIFIED NodeKind = 0
	NODE_KIND_NO_OP       NodeKind = 1
	NODE_KIND_REQUEST     NodeKind = 2
	NODE_KIND_CONDITION   NodeKind = 3
	NODE_KIND_FOR         NodeKind = 4
	NODE_KIND_FOR_EACH    NodeKind = 5
	NODE_KIND_JS          NodeKind = 6
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
	return [...]string{"Unspecified", "Running", "Success", "Failure", "Canceled"}[a]
}

func StringNodeStateWithIcons(a NodeState) string {
	return [...]string{"🔄 Starting", "⏳ Running", "✅ Success", "❌ Failed", "Canceled"}[a]
}

type MNode struct {
	ID        idwrap.IDWrap
	FlowID    idwrap.IDWrap
	Name      string
	NodeKind  NodeKind
	PositionX float64
	PositionY float64
}

type NodeInterface interface {
	mnfor.MNFor | mnif.MNIF | mnrequest.MNRequest
}
