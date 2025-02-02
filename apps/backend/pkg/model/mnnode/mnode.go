package mnnode

import (
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mnnode/mnfor"
	"the-dev-tools/backend/pkg/model/mnnode/mnif"
	"the-dev-tools/backend/pkg/model/mnnode/mnrequest"
)

type NodeKind int32

/* RPC
NODE_KIND_UNSPECIFIED: 0,
NODE_KIND_NO_OP: 1,
NODE_KIND_REQUEST: 2,
NODE_KIND_CONDITION: 3,
NODE_KIND_FOR: 4,
NODE_KIND_FOR_EACH: 5,
*/

const (
	NODE_KIND_UNSPECIFIED NodeKind = 0
	NODE_KIND_NO_OP       NodeKind = 1
	NODE_KIND_REQUEST     NodeKind = 2
	NODE_KIND_CONDITION   NodeKind = 3
	NODE_KIND_FOR         NodeKind = 4
	NODE_KIND_FOR_EACH    NodeKind = 5
)

type MNode struct {
	ID        idwrap.IDWrap
	FlowID    idwrap.IDWrap
	NodeKind  NodeKind
	PositionX float64
	PositionY float64
}

type NodeInterface interface {
	mnfor.MNFor | mnif.MNIF | mnrequest.MNRequest
}
