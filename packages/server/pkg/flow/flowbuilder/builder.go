//nolint:revive // exported
package flowbuilder

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/compress"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nai"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nfor"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nforeach"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nif"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/njs"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nmemory"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/naiprovider"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nrequest"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nstart"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/http/resolver"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/httpclient"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcondition"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/scredential"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/senv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/node_js_executor/v1/node_js_executorv1connect"
)

type Builder struct {
	Node        *sflow.NodeService
	NodeRequest *sflow.NodeRequestService
	NodeFor     *sflow.NodeForService
	NodeForEach *sflow.NodeForEachService
	NodeIf      *sflow.NodeIfService
	NodeJS      *sflow.NodeJsService
	NodeAI      *sflow.NodeAIService
	NodeAiProvider *sflow.NodeAiProviderService
	NodeMemory  *sflow.NodeMemoryService

	Workspace    *sworkspace.WorkspaceService
	Variable     *senv.VariableService
	FlowVariable *sflow.FlowVariableService

	Resolver           resolver.RequestResolver
	Logger             *slog.Logger
	LLMProviderFactory *scredential.LLMProviderFactory
}

func New(
	ns *sflow.NodeService,
	nrs *sflow.NodeRequestService,
	nfs *sflow.NodeForService,
	nfes *sflow.NodeForEachService,
	nifs *sflow.NodeIfService,
	njss *sflow.NodeJsService,
	nais *sflow.NodeAIService,
	naps *sflow.NodeAiProviderService,
	nmems *sflow.NodeMemoryService,
	ws *sworkspace.WorkspaceService,
	vs *senv.VariableService,
	fvs *sflow.FlowVariableService,
	resolver resolver.RequestResolver,
	logger *slog.Logger,
	llmFactory *scredential.LLMProviderFactory,
) *Builder {
	return &Builder{
		Node:               ns,
		NodeRequest:        nrs,
		NodeFor:            nfs,
		NodeForEach:        nfes,
		NodeIf:             nifs,
		NodeJS:             njss,
		NodeAI:             nais,
		NodeAiProvider:     naps,
		NodeMemory:         nmems,
		Workspace:          ws,
		Variable:           vs,
		FlowVariable:       fvs,
		Resolver:           resolver,
		Logger:             logger,
		LLMProviderFactory: llmFactory,
	}
}

func (b *Builder) BuildNodes(
	ctx context.Context,
	flow mflow.Flow,
	nodes []mflow.Node,
	timeout time.Duration,
	httpClient httpclient.HttpClient,
	respChan chan nrequest.NodeRequestSideResp,
	jsClient node_js_executorv1connect.NodeJsExecutorServiceClient,
) (map[idwrap.IDWrap]node.FlowNode, idwrap.IDWrap, error) {
	flowNodeMap := make(map[idwrap.IDWrap]node.FlowNode, len(nodes))
	var startNodeID idwrap.IDWrap

	for _, nodeModel := range nodes {
		switch nodeModel.NodeKind {
		case mflow.NODE_KIND_MANUAL_START:
			flowNodeMap[nodeModel.ID] = nstart.New(nodeModel.ID, nodeModel.Name)
			startNodeID = nodeModel.ID
		case mflow.NODE_KIND_REQUEST:
			requestCfg, err := b.NodeRequest.GetNodeRequest(ctx, nodeModel.ID)
			if err != nil {
				return nil, idwrap.IDWrap{}, err
			}
			if requestCfg == nil || requestCfg.HttpID == nil || isZeroID(*requestCfg.HttpID) {
				return nil, idwrap.IDWrap{}, fmt.Errorf("request node %s missing http configuration", nodeModel.ID.String())
			}

			resolved, err := b.Resolver.Resolve(ctx, *requestCfg.HttpID, requestCfg.DeltaHttpID)
			if err != nil {
				return nil, idwrap.IDWrap{}, fmt.Errorf("resolve http %s: %w", requestCfg.HttpID.String(), err)
			}

			requestNode := nrequest.New(
				nodeModel.ID,
				nodeModel.Name,
				resolved.Resolved,
				resolved.ResolvedHeaders,
				resolved.ResolvedQueries,
				&resolved.ResolvedRawBody,
				resolved.ResolvedFormBody,
				resolved.ResolvedUrlEncodedBody,
				resolved.ResolvedAsserts,
				httpClient,
				respChan,
				b.Logger,
			)
			flowNodeMap[nodeModel.ID] = requestNode

		case mflow.NODE_KIND_FOR:
			forCfg, err := b.NodeFor.GetNodeFor(ctx, nodeModel.ID)
			if err != nil {
				return nil, idwrap.IDWrap{}, err
			}
			if forCfg == nil {
				// Default configuration if missing
				flowNodeMap[nodeModel.ID] = nfor.New(nodeModel.ID, nodeModel.Name, 1, timeout, mflow.ErrorHandling_ERROR_HANDLING_BREAK)
			} else {
				// Use IterCount from config, but default to 1 if not set (0 means unconfigured)
				iterCount := forCfg.IterCount
				if iterCount <= 0 {
					iterCount = 1
				}
				if forCfg.Condition.Comparisons.Expression != "" {
					flowNodeMap[nodeModel.ID] = nfor.NewWithCondition(nodeModel.ID, nodeModel.Name, iterCount, timeout, forCfg.ErrorHandling, forCfg.Condition)
				} else {
					flowNodeMap[nodeModel.ID] = nfor.New(nodeModel.ID, nodeModel.Name, iterCount, timeout, forCfg.ErrorHandling)
				}
			}
		case mflow.NODE_KIND_FOR_EACH:
			forEachCfg, err := b.NodeForEach.GetNodeForEach(ctx, nodeModel.ID)
			if err != nil {
				return nil, idwrap.IDWrap{}, err
			}
			if forEachCfg == nil {
				// Default configuration if missing
				flowNodeMap[nodeModel.ID] = nforeach.New(nodeModel.ID, nodeModel.Name, "", timeout, mcondition.Condition{}, mflow.ErrorHandling_ERROR_HANDLING_BREAK)
			} else {
				flowNodeMap[nodeModel.ID] = nforeach.New(nodeModel.ID, nodeModel.Name, forEachCfg.IterExpression, timeout, forEachCfg.Condition, forEachCfg.ErrorHandling)
			}
		case mflow.NODE_KIND_CONDITION:
			condCfg, err := b.NodeIf.GetNodeIf(ctx, nodeModel.ID)
			if err != nil {
				return nil, idwrap.IDWrap{}, err
			}
			if condCfg == nil {
				// Default to "true" or "false"? Usually better to default to something safe or empty.
				// If empty, it might fail evaluation. Let's use an empty condition.
				flowNodeMap[nodeModel.ID] = nif.New(nodeModel.ID, nodeModel.Name, mcondition.Condition{})
			} else {
				flowNodeMap[nodeModel.ID] = nif.New(nodeModel.ID, nodeModel.Name, condCfg.Condition)
			}
		case mflow.NODE_KIND_JS:
			jsCfg, err := b.NodeJS.GetNodeJS(ctx, nodeModel.ID)
			if err != nil {
				return nil, idwrap.IDWrap{}, err
			}
			if jsCfg == nil {
				// Default empty JS
				flowNodeMap[nodeModel.ID] = njs.New(nodeModel.ID, nodeModel.Name, "", jsClient)
			} else {
				codeBytes := jsCfg.Code
				if jsCfg.CodeCompressType != compress.CompressTypeNone {
					codeBytes, err = compress.Decompress(jsCfg.Code, jsCfg.CodeCompressType)
					if err != nil {
						return nil, idwrap.IDWrap{}, fmt.Errorf("decompress js code: %w", err)
					}
				}
				flowNodeMap[nodeModel.ID] = njs.New(nodeModel.ID, nodeModel.Name, string(codeBytes), jsClient)
			}
		case mflow.NODE_KIND_AI:
			aiCfg, err := b.NodeAI.GetNodeAI(ctx, nodeModel.ID)
			if err != nil {
				return nil, idwrap.IDWrap{}, err
			}
			if aiCfg == nil {
				// Default AI node with empty prompt
				flowNodeMap[nodeModel.ID] = nai.New(
					nodeModel.ID,
					nodeModel.Name,
					"",
					5,
					b.LLMProviderFactory,
				)
			} else {
				flowNodeMap[nodeModel.ID] = nai.New(
					nodeModel.ID,
					nodeModel.Name,
					aiCfg.Prompt,
					aiCfg.MaxIterations,
					b.LLMProviderFactory,
				)
			}
		case mflow.NODE_KIND_AI_PROVIDER:
			providerCfg, err := b.NodeAiProvider.GetNodeAiProvider(ctx, nodeModel.ID)
			if err != nil && !errors.Is(err, sflow.ErrNoNodeAiProviderFound) {
				return nil, idwrap.IDWrap{}, err
			}
			if providerCfg == nil {
				// Default AI Provider node (no credential set)
				flowNodeMap[nodeModel.ID] = naiprovider.New(
					nodeModel.ID,
					nodeModel.Name,
					nil, // No credential set yet
					mflow.AiModelGpt52,
					"",
					nil,
					nil,
				)
			} else {
				flowNodeMap[nodeModel.ID] = naiprovider.New(
					nodeModel.ID,
					nodeModel.Name,
					providerCfg.CredentialID, // Already a *idwrap.IDWrap
					providerCfg.Model,
					"", // TODO(persistent-kv): CustomModel will be stored when persistent key-value store is implemented
					providerCfg.Temperature,
					providerCfg.MaxTokens,
				)
			}
		case mflow.NODE_KIND_AI_MEMORY:
			memoryCfg, err := b.NodeMemory.GetNodeMemory(ctx, nodeModel.ID)
			if err != nil && !errors.Is(err, sflow.ErrNoNodeMemoryFound) {
				return nil, idwrap.IDWrap{}, err
			}
			if memoryCfg == nil {
				// Default Memory node with window buffer of 10 messages
				flowNodeMap[nodeModel.ID] = nmemory.New(
					nodeModel.ID,
					nodeModel.Name,
					mflow.AiMemoryTypeWindowBuffer,
					10,
				)
			} else {
				flowNodeMap[nodeModel.ID] = nmemory.New(
					nodeModel.ID,
					nodeModel.Name,
					memoryCfg.MemoryType,
					memoryCfg.WindowSize,
				)
			}
		default:
			return nil, idwrap.IDWrap{}, fmt.Errorf("node kind %d not supported", nodeModel.NodeKind)
		}
	}

	if startNodeID == (idwrap.IDWrap{}) {
		return nil, idwrap.IDWrap{}, errors.New("flow missing start node")
	}

	return flowNodeMap, startNodeID, nil
}

// EnvNamespace is the namespace key for environment variables in VarMap.
// Access environment variables using {{ env.varName }} syntax.
const EnvNamespace = "env"

func (b *Builder) BuildVariables(
	ctx context.Context,
	workspaceID idwrap.IDWrap,
	flowVars []mflow.FlowVariable,
) (map[string]any, error) {
	baseVars := make(map[string]any)
	envVars := make(map[string]any)

	// Get workspace to find GlobalEnv and ActiveEnv
	workspace, err := b.Workspace.Get(ctx, workspaceID)
	if err != nil {
		// If workspace not found, just use flow vars
		b.Logger.Warn("failed to get workspace for environment variables", "workspace_id", workspaceID.String(), "error", err)
	} else {
		// 1. Add global environment variables to env namespace
		if workspace.GlobalEnv != (idwrap.IDWrap{}) {
			globalVars, err := b.Variable.GetVariableByEnvID(ctx, workspace.GlobalEnv)
			if err != nil && !errors.Is(err, senv.ErrNoVarFound) {
				b.Logger.Warn("failed to get global environment variables", "env_id", workspace.GlobalEnv.String(), "error", err)
			} else {
				for _, v := range globalVars {
					if v.Enabled {
						envVars[v.VarKey] = v.Value
					}
				}
			}
		}

		// 2. Add active environment variables (override global) to env namespace
		// Only if ActiveEnv is different from GlobalEnv
		if workspace.ActiveEnv != (idwrap.IDWrap{}) && workspace.ActiveEnv != workspace.GlobalEnv {
			activeVars, err := b.Variable.GetVariableByEnvID(ctx, workspace.ActiveEnv)
			if err != nil && !errors.Is(err, senv.ErrNoVarFound) {
				b.Logger.Warn("failed to get active environment variables", "env_id", workspace.ActiveEnv.String(), "error", err)
			} else {
				for _, v := range activeVars {
					if v.Enabled {
						envVars[v.VarKey] = v.Value
					}
				}
			}
		}
	}

	// 3. Add flow-level variables to env namespace (override environment variables)
	for _, variable := range flowVars {
		if variable.Enabled {
			envVars[variable.Name] = variable.Value
		}
	}

	// Store all environment/flow variables under the "env" namespace
	// Access via {{ env.apiKey }} or {{ env["key.with.dots"] }}
	baseVars[EnvNamespace] = envVars

	return baseVars, nil
}

func isZeroID(id idwrap.IDWrap) bool {
	return id == idwrap.IDWrap{}
}
