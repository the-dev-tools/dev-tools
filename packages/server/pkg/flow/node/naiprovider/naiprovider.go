// Package naiprovider provides the AI Provider node implementation for flow execution.
// AI Provider nodes are passive configuration providers that supply LLM settings
// to connected AI Agent nodes via HandleAiProvider edges.
package naiprovider

import (
	"context"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

// NodeAiProvider represents an AI Provider node that provides LLM configuration to AI Agent nodes.
// It is a passive node - it does not execute but provides configuration when
// discovered by AI nodes via HandleAiProvider edges.
type NodeAiProvider struct {
	FlowNodeID   idwrap.IDWrap
	Name         string
	CredentialID idwrap.IDWrap
	Model        mflow.AiModel
	CustomModel  string   // Used when Model == AiModelCustom
	Temperature  *float32 // Optional: nil means use provider default
	MaxTokens    *int32   // Optional: nil means use provider default
}

// New creates a new NodeAiProvider with the given configuration.
func New(
	id idwrap.IDWrap,
	name string,
	credentialID idwrap.IDWrap,
	model mflow.AiModel,
	customModel string,
	temperature *float32,
	maxTokens *int32,
) *NodeAiProvider {
	return &NodeAiProvider{
		FlowNodeID:   id,
		Name:         name,
		CredentialID: credentialID,
		Model:        model,
		CustomModel:  customModel,
		Temperature:  temperature,
		MaxTokens:    maxTokens,
	}
}

// GetID returns the node's unique identifier.
func (n *NodeAiProvider) GetID() idwrap.IDWrap { return n.FlowNodeID }

// GetName returns the node's display name.
func (n *NodeAiProvider) GetName() string { return n.Name }

// RunSync is a no-op for AI Provider nodes. AI Provider nodes are passive configuration
// providers and do not execute directly. They are discovered by AI Agent nodes
// via HandleAiProvider edges.
func (n *NodeAiProvider) RunSync(_ context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	// AI Provider nodes are passive - they don't produce output or trigger next nodes.
	// They are read by AI Agent nodes via edge connections.
	next := mflow.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, mflow.HandleUnspecified)
	return node.FlowNodeResult{
		NextNodeID: next,
		Err:        nil,
	}
}

// RunAsync runs the node asynchronously by calling RunSync and sending the result.
func (n *NodeAiProvider) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	resultChan <- n.RunSync(ctx, req)
}
