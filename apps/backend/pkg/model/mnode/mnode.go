package mnode

import (
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mnode/mnfor"
	"the-dev-tools/backend/pkg/model/mnode/mnif"
	"the-dev-tools/backend/pkg/model/mnode/mnrequest"
)

/* DB
id BLOB NOT NULL PRIMARY KEY,
flow_id BLOB NOT NULL,
node_type INT8 NOT NULL,
position_x REAL NOT NULL,
position_y REAL NOT NULL,
*/

type NodeKind int32

/* RPC
NODE_KIND_UNSPECIFIED: 0,
NODE_KIND_START: 1,
NODE_KIND_REQUEST: 2,
NODE_KIND_FOR: 3,
NODE_KIND_CONDITION: 4,
*/

const (
	NODE_KIND_UNSPECIFIED NodeKind = iota
	NODE_KIND_NOOP
	NODE_KIND_REQUEST
	NODE_KIND_FOR
	NODE_KIND_CONDITION
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
