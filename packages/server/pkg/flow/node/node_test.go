package node_test

import (
	"errors"
	"sync"
	"testing"

	"the-dev-tools/server/pkg/flow/node"

	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)

	storedValue, err := node.ReadNodeVar(req, nodeName, key)
	require.NoError(t, err)

	require.Equal(t, value, storedValue)
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
	require.NoError(t, err)

	require.Equal(t, value, storedValue)
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
	require.NoError(t, err)

	require.Equal(t, value, storedValue)
}

func TestReadNodeVar_NodeNotFound(t *testing.T) {
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		ReadWriteLock: &sync.RWMutex{},
	}

	key := "testKey"
	nodeName := "test-node"

	_, err := node.ReadNodeVar(req, nodeName, key)
	require.Error(t, err, "expected error")
	require.Equal(t, node.ErrVarNodeNotFound, err)
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
	require.Error(t, err, "expected error")

	expectedErr := errors.New("key not found")
	require.Equal(t, expectedErr.Error(), err.Error())
}
