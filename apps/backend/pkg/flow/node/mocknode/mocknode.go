package mocknode

import (
	"the-dev-tools/backend/pkg/idwrap"
)

type MockNode struct {
	ID    idwrap.IDWrap
	Next  *idwrap.IDWrap
	OnRun func()
}

func NewMockNode(id idwrap.IDWrap, next *idwrap.IDWrap, onRun func()) *MockNode {
	return &MockNode{
		ID:    id,
		Next:  next,
		OnRun: onRun,
	}
}

func (mn *MockNode) GetID() idwrap.IDWrap {
	return mn.ID
}

func (mn *MockNode) SetID(id idwrap.IDWrap) {
	mn.ID = id
}

func (mn *MockNode) Run(variableMap map[string]interface{}) (*idwrap.IDWrap, error) {
	mn.OnRun()
	return mn.Next, nil
}
