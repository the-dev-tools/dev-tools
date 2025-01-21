package node_test

import (
	"errors"
	"sync"
	"testing"
	"the-dev-tools/backend/pkg/flow/node"
	"the-dev-tools/backend/pkg/idwrap"
)

func TestAddNodeVar(t *testing.T) {
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		ReadWriteLock: &sync.RWMutex{},
	}

	nodeID := idwrap.NewNow()
	key := "testKey"
	value := "testValue"

	err := node.AddNodeVar(req, value, nodeID, key)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	storedValue, err := node.ReadNodeVar(req, nodeID, key)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if storedValue != value {
		t.Fatalf("expected %v, got %v", value, storedValue)
	}
}

func TestReadVarRaw(t *testing.T) {
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		ReadWriteLock: &sync.RWMutex{},
	}

	key := "testKey"
	value := "testValue"
	req.VarMap[key] = value

	storedValue, err := node.ReadVarRaw(req, key)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if storedValue != value {
		t.Fatalf("expected %v, got %v", value, storedValue)
	}
}

func TestReadNodeVar(t *testing.T) {
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		ReadWriteLock: &sync.RWMutex{},
	}

	nodeID := idwrap.NewNow()
	key := "testKey"
	value := "testValue"
	req.VarMap[nodeID.String()] = map[string]interface{}{key: value}

	storedValue, err := node.ReadNodeVar(req, nodeID, key)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if storedValue != value {
		t.Fatalf("expected %v, got %v", value, storedValue)
	}
}

func TestReadNodeVar_NodeNotFound(t *testing.T) {
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		ReadWriteLock: &sync.RWMutex{},
	}

	nodeID := idwrap.NewNow()
	key := "testKey"

	_, err := node.ReadNodeVar(req, nodeID, key)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if err != node.ErrVarNodeNotFound {
		t.Fatalf("expected %v, got %v", node.ErrNodeNotFound, err)
	}
}

func TestReadNodeVar_KeyNotFound(t *testing.T) {
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		ReadWriteLock: &sync.RWMutex{},
	}

	nodeID := idwrap.NewNow()
	req.VarMap[nodeID.String()] = map[string]interface{}{}

	key := "testKey"

	_, err := node.ReadNodeVar(req, nodeID, key)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	expectedErr := errors.New("key not found")
	if err.Error() != expectedErr.Error() {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}
