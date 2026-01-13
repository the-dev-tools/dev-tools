package sflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func TestNodeAiMapper(t *testing.T) {
	nodeID := idwrap.NewNow()
	credID := idwrap.NewNow()

	mn := mflow.NodeAI{
		FlowNodeID:    nodeID,
		Model:         mflow.AiModelGpt52Thinking,
		CredentialID:  credID,
		Prompt:        "test prompt",
		MaxIterations: 42,
	}

	dbn := ConvertNodeAiToDB(mn)
	assert.Equal(t, nodeID, dbn.FlowNodeID)
	assert.Equal(t, int8(mflow.AiModelGpt52Thinking), dbn.Model)
	assert.Equal(t, credID.Bytes(), dbn.CredentialID)
	assert.Equal(t, "test prompt", dbn.Prompt)
	assert.Equal(t, int32(42), dbn.MaxIterations)

	mn2 := ConvertDBToNodeAi(dbn)
	assert.Equal(t, mn.FlowNodeID, mn2.FlowNodeID)
	assert.Equal(t, mn.Model, mn2.Model)
	assert.Equal(t, mn.CredentialID, mn2.CredentialID)
	assert.Equal(t, mn.Prompt, mn2.Prompt)
	assert.Equal(t, mn.MaxIterations, mn2.MaxIterations)
}
