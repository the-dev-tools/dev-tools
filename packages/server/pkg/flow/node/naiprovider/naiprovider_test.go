package naiprovider

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func TestNewNodeAiProvider(t *testing.T) {
	id := idwrap.NewNow()
	credID := idwrap.NewNow()
	temp := float32(0.7)
	maxTokens := int32(4096)

	n := New(id, "TestAiProvider", &credID, mflow.AiModelGpt52Instant, "", &temp, &maxTokens)

	assert.Equal(t, id, n.GetID())
	assert.Equal(t, "TestAiProvider", n.GetName())
	require.NotNil(t, n.CredentialID)
	assert.Equal(t, credID, *n.CredentialID)
	assert.Equal(t, mflow.AiModelGpt52Instant, n.Model)
	assert.Equal(t, "", n.CustomModel)
	require.NotNil(t, n.Temperature)
	assert.Equal(t, float32(0.7), *n.Temperature)
	require.NotNil(t, n.MaxTokens)
	assert.Equal(t, int32(4096), *n.MaxTokens)
}

func TestNewNodeAiProvider_NilOptionalFields(t *testing.T) {
	id := idwrap.NewNow()
	credID := idwrap.NewNow()

	n := New(id, "TestAiProvider", &credID, mflow.AiModelClaudeSonnet45, "custom-model", nil, nil)

	assert.Equal(t, id, n.GetID())
	assert.Equal(t, "TestAiProvider", n.GetName())
	assert.Equal(t, mflow.AiModelClaudeSonnet45, n.Model)
	assert.Equal(t, "custom-model", n.CustomModel)
	assert.Nil(t, n.Temperature)
	assert.Nil(t, n.MaxTokens)
}

func TestNewNodeAiProvider_NilCredentialID(t *testing.T) {
	id := idwrap.NewNow()

	n := New(id, "TestAiProvider", nil, mflow.AiModelClaudeSonnet45, "", nil, nil)

	assert.Equal(t, id, n.GetID())
	assert.Nil(t, n.CredentialID)
}

func TestNodeAiProvider_RunSync_PassesThrough(t *testing.T) {
	nodeID := idwrap.NewNow()
	nextID := idwrap.NewNow()
	credID := idwrap.NewNow()

	n := New(nodeID, "AiProvider", &credID, mflow.AiModelGemini3Flash, "", nil, nil)

	// Setup edge map for pass-through
	edgeMap := mflow.EdgesMap{
		nodeID: {
			mflow.HandleUnspecified: []idwrap.IDWrap{nextID},
		},
	}

	req := &node.FlowNodeRequest{
		EdgeSourceMap: edgeMap,
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
	}

	result := n.RunSync(context.Background(), req)

	assert.NoError(t, result.Err)
	require.Len(t, result.NextNodeID, 1)
	assert.Equal(t, nextID, result.NextNodeID[0])
}

func TestNodeAiProvider_RunSync_NoNextNode(t *testing.T) {
	nodeID := idwrap.NewNow()
	credID := idwrap.NewNow()

	n := New(nodeID, "AiProvider", &credID, mflow.AiModelO3, "", nil, nil)

	req := &node.FlowNodeRequest{
		EdgeSourceMap: mflow.EdgesMap{},
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
	}

	result := n.RunSync(context.Background(), req)

	assert.NoError(t, result.Err)
	assert.Empty(t, result.NextNodeID)
}

func TestNodeAiProvider_RunAsync(t *testing.T) {
	nodeID := idwrap.NewNow()
	credID := idwrap.NewNow()

	n := New(nodeID, "AiProvider", &credID, mflow.AiModelClaudeOpus45, "", nil, nil)

	req := &node.FlowNodeRequest{
		EdgeSourceMap: mflow.EdgesMap{},
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
	}

	resultChan := make(chan node.FlowNodeResult, 1)
	n.RunAsync(context.Background(), req, resultChan)

	result := <-resultChan
	assert.NoError(t, result.Err)
}

func TestNodeAiProvider_AllModelTypes(t *testing.T) {
	models := []mflow.AiModel{
		mflow.AiModelGpt52Instant,
		mflow.AiModelGpt52Pro,
		mflow.AiModelClaudeSonnet45,
		mflow.AiModelClaudeOpus45,
		mflow.AiModelGemini3Flash,
		mflow.AiModelO3,
	}

	for _, model := range models {
		t.Run(fmt.Sprintf("model_%d", model), func(t *testing.T) {
			id := idwrap.NewNow()
			credID := idwrap.NewNow()

			n := New(id, "AiProvider", &credID, model, "", nil, nil)
			assert.Equal(t, model, n.Model)
		})
	}
}
