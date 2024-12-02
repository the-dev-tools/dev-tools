package node

import "the-dev-tools/backend/pkg/idwrap"

type FlowNode interface {
	GetID() idwrap.IDWrap

	// TODO: will implement streaming in the future
	Run(variableMap map[string]interface{}) (*idwrap.IDWrap, error)
}
