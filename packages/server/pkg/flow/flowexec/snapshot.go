package flowexec

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcondition"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
)

// NodeConfigSnapshot reads and writes a single node kind's type-specific
// configuration during flow version snapshots.
type NodeConfigSnapshot interface {
	Kind() mflow.NodeKind
	// Read fetches the type-specific config for a node. Returns (nil, nil) if none exists.
	Read(ctx context.Context, nodeID idwrap.IDWrap) (any, error)
	// WriteTx creates a copy of the config with the new node ID inside the given transaction.
	// Returns the created model (for event publishing) or (nil, nil) if skipped.
	WriteTx(ctx context.Context, tx *sql.Tx, newNodeID idwrap.IDWrap, config any) (any, error)
}

// SnapshotRegistry maps node kinds to their snapshot handlers.
type SnapshotRegistry struct {
	handlers map[mflow.NodeKind]NodeConfigSnapshot
}

// NewSnapshotRegistry creates an empty registry.
func NewSnapshotRegistry() *SnapshotRegistry {
	return &SnapshotRegistry{handlers: make(map[mflow.NodeKind]NodeConfigSnapshot)}
}

// Register adds a snapshot handler for a node kind.
func (r *SnapshotRegistry) Register(s NodeConfigSnapshot) {
	r.handlers[s.Kind()] = s
}

// Get returns the snapshot handler for a node kind, if registered.
// Returns (nil, false) if the registry is nil or the kind is not registered.
func (r *SnapshotRegistry) Get(kind mflow.NodeKind) (NodeConfigSnapshot, bool) {
	if r == nil {
		return nil, false
	}
	s, ok := r.handlers[kind]
	return s, ok
}

// ReadAll reads type-specific configurations for all nodes that have registered handlers.
// Returns a map from node ID to the config value. Errors are logged and the node is
// skipped (resulting in default config when written).
func (r *SnapshotRegistry) ReadAll(ctx context.Context, nodes []mflow.Node, logger interface{ Warn(msg string, args ...any) }) map[idwrap.IDWrap]any {
	configs := make(map[idwrap.IDWrap]any, len(nodes))
	for _, node := range nodes {
		handler, ok := r.Get(node.NodeKind)
		if !ok {
			continue
		}
		config, err := handler.Read(ctx, node.ID)
		if err != nil {
			logger.Warn("failed to read node config, using defaults",
				"node_id", node.ID.String(), "kind", node.NodeKind, "error", err)
			continue
		}
		configs[node.ID] = config
	}
	return configs
}

// NodeConfigResult holds the result of writing a single node's type-specific config.
type NodeConfigResult struct {
	NodeKind mflow.NodeKind
	Config   any // The created model (e.g., mflow.NodeFor, mflow.NodeJS)
}

// WriteAllTx writes type-specific configurations for all nodes within a transaction.
// Uses configs from ReadAll. Returns the created configs for event publishing.
func (r *SnapshotRegistry) WriteAllTx(
	ctx context.Context,
	tx *sql.Tx,
	sourceNodes []mflow.Node,
	nodeIDMapping map[string]idwrap.IDWrap,
	configs map[idwrap.IDWrap]any,
) ([]NodeConfigResult, error) {
	var results []NodeConfigResult
	for _, sourceNode := range sourceNodes {
		handler, ok := r.Get(sourceNode.NodeKind)
		if !ok {
			continue
		}
		newNodeID, ok := nodeIDMapping[sourceNode.ID.String()]
		if !ok {
			continue
		}
		config := configs[sourceNode.ID]
		created, err := handler.WriteTx(ctx, tx, newNodeID, config)
		if err != nil {
			return nil, fmt.Errorf("create %s node config: %w", sourceNode.Name, err)
		}
		if created != nil {
			results = append(results, NodeConfigResult{
				NodeKind: sourceNode.NodeKind,
				Config:   created,
			})
		}
	}
	return results, nil
}

// --- Request ---

type RequestSnapshot struct{ Service *sflow.NodeRequestService }

func (s *RequestSnapshot) Kind() mflow.NodeKind { return mflow.NODE_KIND_REQUEST }

func (s *RequestSnapshot) Read(ctx context.Context, nodeID idwrap.IDWrap) (any, error) {
	return s.Service.GetNodeRequest(ctx, nodeID)
}

func (s *RequestSnapshot) WriteTx(ctx context.Context, tx *sql.Tx, newNodeID idwrap.IDWrap, config any) (any, error) {
	src, _ := config.(*mflow.NodeRequest)
	if src == nil {
		return nil, nil
	}
	newData := mflow.NodeRequest{
		FlowNodeID:       newNodeID,
		HttpID:           src.HttpID,
		DeltaHttpID:      src.DeltaHttpID,
		HasRequestConfig: src.HasRequestConfig,
	}
	writer := s.Service.TX(tx)
	return newData, writer.CreateNodeRequest(ctx, newData)
}

// --- For ---

type ForSnapshot struct{ Service *sflow.NodeForService }

func (s *ForSnapshot) Kind() mflow.NodeKind { return mflow.NODE_KIND_FOR }

func (s *ForSnapshot) Read(ctx context.Context, nodeID idwrap.IDWrap) (any, error) {
	return s.Service.GetNodeFor(ctx, nodeID)
}

func (s *ForSnapshot) WriteTx(ctx context.Context, tx *sql.Tx, newNodeID idwrap.IDWrap, config any) (any, error) {
	newData := mflow.NodeFor{
		FlowNodeID:    newNodeID,
		IterCount:     1,
		Condition:     mcondition.Condition{},
		ErrorHandling: mflow.ErrorHandling_ERROR_HANDLING_BREAK,
	}
	if src, ok := config.(*mflow.NodeFor); ok && src != nil {
		if src.IterCount > 0 {
			newData.IterCount = src.IterCount
		}
		newData.Condition = src.Condition
		newData.ErrorHandling = src.ErrorHandling
	}
	writer := s.Service.TX(tx)
	return newData, writer.CreateNodeFor(ctx, newData)
}

// --- ForEach ---

type ForEachSnapshot struct{ Service *sflow.NodeForEachService }

func (s *ForEachSnapshot) Kind() mflow.NodeKind { return mflow.NODE_KIND_FOR_EACH }

func (s *ForEachSnapshot) Read(ctx context.Context, nodeID idwrap.IDWrap) (any, error) {
	return s.Service.GetNodeForEach(ctx, nodeID)
}

func (s *ForEachSnapshot) WriteTx(ctx context.Context, tx *sql.Tx, newNodeID idwrap.IDWrap, config any) (any, error) {
	newData := mflow.NodeForEach{
		FlowNodeID:     newNodeID,
		IterExpression: "",
		Condition:      mcondition.Condition{},
		ErrorHandling:  mflow.ErrorHandling_ERROR_HANDLING_BREAK,
	}
	if src, ok := config.(*mflow.NodeForEach); ok && src != nil {
		newData.IterExpression = src.IterExpression
		newData.Condition = src.Condition
		newData.ErrorHandling = src.ErrorHandling
	}
	writer := s.Service.TX(tx)
	return newData, writer.CreateNodeForEach(ctx, newData)
}

// --- Condition (If) ---

type ConditionSnapshot struct{ Service *sflow.NodeIfService }

func (s *ConditionSnapshot) Kind() mflow.NodeKind { return mflow.NODE_KIND_CONDITION }

func (s *ConditionSnapshot) Read(ctx context.Context, nodeID idwrap.IDWrap) (any, error) {
	return s.Service.GetNodeIf(ctx, nodeID)
}

func (s *ConditionSnapshot) WriteTx(ctx context.Context, tx *sql.Tx, newNodeID idwrap.IDWrap, config any) (any, error) {
	newData := mflow.NodeIf{
		FlowNodeID: newNodeID,
		Condition:  mcondition.Condition{},
	}
	if src, ok := config.(*mflow.NodeIf); ok && src != nil {
		newData.Condition = src.Condition
	}
	writer := s.Service.TX(tx)
	return newData, writer.CreateNodeIf(ctx, newData)
}

// --- JS ---

type JSSnapshot struct{ Service *sflow.NodeJsService }

func (s *JSSnapshot) Kind() mflow.NodeKind { return mflow.NODE_KIND_JS }

func (s *JSSnapshot) Read(ctx context.Context, nodeID idwrap.IDWrap) (any, error) {
	return s.Service.GetNodeJS(ctx, nodeID)
}

func (s *JSSnapshot) WriteTx(ctx context.Context, tx *sql.Tx, newNodeID idwrap.IDWrap, config any) (any, error) {
	newData := mflow.NodeJS{
		FlowNodeID:       newNodeID,
		Code:             nil,
		CodeCompressType: 0,
	}
	if src, ok := config.(*mflow.NodeJS); ok && src != nil {
		newData.Code = src.Code
		newData.CodeCompressType = src.CodeCompressType
	}
	writer := s.Service.TX(tx)
	return newData, writer.CreateNodeJS(ctx, newData)
}

// --- AI ---

type AISnapshot struct{ Service *sflow.NodeAIService }

func (s *AISnapshot) Kind() mflow.NodeKind { return mflow.NODE_KIND_AI }

func (s *AISnapshot) Read(ctx context.Context, nodeID idwrap.IDWrap) (any, error) {
	return s.Service.GetNodeAI(ctx, nodeID)
}

func (s *AISnapshot) WriteTx(ctx context.Context, tx *sql.Tx, newNodeID idwrap.IDWrap, config any) (any, error) {
	newData := mflow.NodeAI{
		FlowNodeID:    newNodeID,
		Prompt:        "",
		MaxIterations: 5,
	}
	if src, ok := config.(*mflow.NodeAI); ok && src != nil {
		newData.Prompt = src.Prompt
		newData.MaxIterations = src.MaxIterations
	}
	writer := s.Service.TX(tx)
	return newData, writer.CreateNodeAI(ctx, newData)
}

// --- AI Provider ---

type AIProviderSnapshot struct{ Service *sflow.NodeAiProviderService }

func (s *AIProviderSnapshot) Kind() mflow.NodeKind { return mflow.NODE_KIND_AI_PROVIDER }

func (s *AIProviderSnapshot) Read(ctx context.Context, nodeID idwrap.IDWrap) (any, error) {
	return s.Service.GetNodeAiProvider(ctx, nodeID)
}

func (s *AIProviderSnapshot) WriteTx(ctx context.Context, tx *sql.Tx, newNodeID idwrap.IDWrap, config any) (any, error) {
	newData := mflow.NodeAiProvider{
		FlowNodeID:   newNodeID,
		CredentialID: nil,
		Model:        mflow.AiModelUnspecified,
		Temperature:  nil,
		MaxTokens:    nil,
	}
	if src, ok := config.(*mflow.NodeAiProvider); ok && src != nil {
		newData.CredentialID = src.CredentialID
		newData.Model = src.Model
		newData.Temperature = src.Temperature
		newData.MaxTokens = src.MaxTokens
	}
	writer := s.Service.TX(tx)
	return newData, writer.CreateNodeAiProvider(ctx, newData)
}

// --- Memory ---

type MemorySnapshot struct{ Service *sflow.NodeMemoryService }

func (s *MemorySnapshot) Kind() mflow.NodeKind { return mflow.NODE_KIND_AI_MEMORY }

func (s *MemorySnapshot) Read(ctx context.Context, nodeID idwrap.IDWrap) (any, error) {
	return s.Service.GetNodeMemory(ctx, nodeID)
}

func (s *MemorySnapshot) WriteTx(ctx context.Context, tx *sql.Tx, newNodeID idwrap.IDWrap, config any) (any, error) {
	newData := mflow.NodeMemory{
		FlowNodeID: newNodeID,
		MemoryType: mflow.AiMemoryTypeWindowBuffer,
		WindowSize: 10,
	}
	if src, ok := config.(*mflow.NodeMemory); ok && src != nil {
		newData.MemoryType = src.MemoryType
		newData.WindowSize = src.WindowSize
	}
	writer := s.Service.TX(tx)
	return newData, writer.CreateNodeMemory(ctx, newData)
}

// --- GraphQL ---

type GraphQLSnapshot struct{ Service *sflow.NodeGraphQLService }

func (s *GraphQLSnapshot) Kind() mflow.NodeKind { return mflow.NODE_KIND_GRAPHQL }

func (s *GraphQLSnapshot) Read(ctx context.Context, nodeID idwrap.IDWrap) (any, error) {
	return s.Service.GetNodeGraphQL(ctx, nodeID)
}

func (s *GraphQLSnapshot) WriteTx(ctx context.Context, tx *sql.Tx, newNodeID idwrap.IDWrap, config any) (any, error) {
	src, _ := config.(*mflow.NodeGraphQL)
	if src == nil {
		return nil, nil
	}
	newData := mflow.NodeGraphQL{
		FlowNodeID: newNodeID,
		GraphQLID:  src.GraphQLID,
	}
	writer := s.Service.TX(tx)
	return newData, writer.CreateNodeGraphQL(ctx, newData)
}

// --- WebSocket Connection ---

type WsConnectionSnapshot struct{ Service *sflow.NodeWsConnectionService }

func (s *WsConnectionSnapshot) Kind() mflow.NodeKind { return mflow.NODE_KIND_WS_CONNECTION }

func (s *WsConnectionSnapshot) Read(ctx context.Context, nodeID idwrap.IDWrap) (any, error) {
	return s.Service.GetNodeWsConnection(ctx, nodeID)
}

func (s *WsConnectionSnapshot) WriteTx(ctx context.Context, tx *sql.Tx, newNodeID idwrap.IDWrap, config any) (any, error) {
	src, _ := config.(*mflow.NodeWsConnection)
	if src == nil {
		return nil, nil
	}
	newData := *src
	newData.FlowNodeID = newNodeID
	writer := s.Service.TX(tx)
	return newData, writer.CreateNodeWsConnection(ctx, newData)
}

// --- WebSocket Send ---

type WsSendSnapshot struct{ Service *sflow.NodeWsSendService }

func (s *WsSendSnapshot) Kind() mflow.NodeKind { return mflow.NODE_KIND_WS_SEND }

func (s *WsSendSnapshot) Read(ctx context.Context, nodeID idwrap.IDWrap) (any, error) {
	return s.Service.GetNodeWsSend(ctx, nodeID)
}

func (s *WsSendSnapshot) WriteTx(ctx context.Context, tx *sql.Tx, newNodeID idwrap.IDWrap, config any) (any, error) {
	src, _ := config.(*mflow.NodeWsSend)
	if src == nil {
		return nil, nil
	}
	newData := *src
	newData.FlowNodeID = newNodeID
	writer := s.Service.TX(tx)
	return newData, writer.CreateNodeWsSend(ctx, newData)
}

// --- Wait ---

type WaitSnapshot struct{ Service *sflow.NodeWaitService }

func (s *WaitSnapshot) Kind() mflow.NodeKind { return mflow.NODE_KIND_WAIT }

func (s *WaitSnapshot) Read(ctx context.Context, nodeID idwrap.IDWrap) (any, error) {
	return s.Service.GetNodeWait(ctx, nodeID)
}

func (s *WaitSnapshot) WriteTx(ctx context.Context, tx *sql.Tx, newNodeID idwrap.IDWrap, config any) (any, error) {
	src, _ := config.(*mflow.NodeWait)
	if src == nil {
		return nil, nil
	}
	newData := *src
	newData.FlowNodeID = newNodeID
	writer := s.Service.TX(tx)
	return newData, writer.CreateNodeWait(ctx, newData)
}
