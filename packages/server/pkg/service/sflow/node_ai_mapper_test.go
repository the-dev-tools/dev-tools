package sflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func TestNodeAiMapper(t *testing.T) {
	nodeID := idwrap.NewNow()

	mn := mflow.NodeAI{
		FlowNodeID:    nodeID,
		Prompt:        "test prompt",
		MaxIterations: 42,
	}

	dbn := ConvertNodeAiToDB(mn)
	assert.Equal(t, nodeID, dbn.FlowNodeID)
	assert.Equal(t, "test prompt", dbn.Prompt)
	assert.Equal(t, int32(42), dbn.MaxIterations)

	mn2 := ConvertDBToNodeAi(dbn)
	assert.Equal(t, mn.FlowNodeID, mn2.FlowNodeID)
	assert.Equal(t, mn.Prompt, mn2.Prompt)
	assert.Equal(t, mn.MaxIterations, mn2.MaxIterations)
}
