package node_test

import (
	"errors"
	"sync"
	"testing"
	"the-dev-tools/server/pkg/flow/node"
)

func TestAddNodeVar(t *testing.T) {
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		ReadWriteLock: &sync.RWMutex{},
	}

	key := "testKey"
	value := "testValue"
	nodeName := "test-node"

	err := node.WriteNodeVar(req, nodeName, key, value)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	storedValue, err := node.ReadNodeVar(req, nodeName, key)
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

	key := "testKey"
	value := "testValue"
	nodeName := "test-node"
	req.VarMap[nodeName] = map[string]interface{}{key: value}

	storedValue, err := node.ReadNodeVar(req, nodeName, key)
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

	key := "testKey"
	nodeName := "test-node"

	_, err := node.ReadNodeVar(req, nodeName, key)
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

	nodeName := "test-node"
	req.VarMap[nodeName] = map[string]interface{}{}

	key := "testKey"

	_, err := node.ReadNodeVar(req, nodeName, key)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	expectedErr := errors.New("key not found")
	if err.Error() != expectedErr.Error() {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

