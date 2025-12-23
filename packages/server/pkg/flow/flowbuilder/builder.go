//nolint:revive // exported
package flowbuilder

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/nfor"
	"the-dev-tools/server/pkg/flow/node/nforeach"
	"the-dev-tools/server/pkg/flow/node/nif"
	"the-dev-tools/server/pkg/flow/node/njs"
	"the-dev-tools/server/pkg/flow/node/nrequest"
	"the-dev-tools/server/pkg/flow/node/nstart"
	"the-dev-tools/server/pkg/http/resolver"
	"the-dev-tools/server/pkg/httpclient"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/spec/dist/buf/go/api/node_js_executor/v1/node_js_executorv1connect"
)

type Builder struct {
	Node        *sflow.NodeService
	NodeRequest *sflow.NodeRequestService
	NodeFor     *sflow.NodeForService
	NodeForEach *sflow.NodeForEachService
	NodeIf      *sflow.NodeIfService
	NodeJS      *sflow.NodeJsService

	Workspace    *sworkspace.WorkspaceService
	Variable     *senv.VariableService
	FlowVariable *sflow.FlowVariableService

	Resolver resolver.RequestResolver
	Logger   *slog.Logger
}

func New(
	ns *sflow.NodeService,
	nrs *sflow.NodeRequestService,
	nfs *sflow.NodeForService,
	nfes *sflow.NodeForEachService,
	nifs *sflow.NodeIfService,
	njss *sflow.NodeJsService,
	ws *sworkspace.WorkspaceService,
	vs *senv.VariableService,
	fvs *sflow.FlowVariableService,
	resolver resolver.RequestResolver,
	logger *slog.Logger,
) *Builder {
	return &Builder{
		Node:         ns,
		NodeRequest:  nrs,
		NodeFor:      nfs,
		NodeForEach:  nfes,
		NodeIf:       nifs,
		NodeJS:       njss,
		Workspace:    ws,
		Variable:     vs,
		FlowVariable: fvs,
		Resolver:     resolver,
		Logger:       logger,
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
		default:
			return nil, idwrap.IDWrap{}, fmt.Errorf("node kind %d not supported", nodeModel.NodeKind)
		}
	}

	if startNodeID == (idwrap.IDWrap{}) {
		return nil, idwrap.IDWrap{}, errors.New("flow missing start node")
	}

	return flowNodeMap, startNodeID, nil
}

func (b *Builder) BuildVariables(
	ctx context.Context,
	workspaceID idwrap.IDWrap,
	flowVars []mflow.FlowVariable,
) (map[string]any, error) {
	baseVars := make(map[string]any)

	// Get workspace to find GlobalEnv and ActiveEnv
	workspace, err := b.Workspace.Get(ctx, workspaceID)
	if err != nil {
		// If workspace not found, just use flow vars
		b.Logger.Warn("failed to get workspace for environment variables", "workspace_id", workspaceID.String(), "error", err)
	} else {
		// 1. Add global environment variables
		if workspace.GlobalEnv != (idwrap.IDWrap{}) {
			globalVars, err := b.Variable.GetVariableByEnvID(ctx, workspace.GlobalEnv)
			if err != nil && !errors.Is(err, senv.ErrNoVarFound) {
				b.Logger.Warn("failed to get global environment variables", "env_id", workspace.GlobalEnv.String(), "error", err)
			} else {
				for _, v := range globalVars {
					if v.Enabled {
						baseVars[v.VarKey] = v.Value
					}
				}
			}
		}

		// 2. Add active environment variables (override global)
		// Only if ActiveEnv is different from GlobalEnv
		if workspace.ActiveEnv != (idwrap.IDWrap{}) && workspace.ActiveEnv != workspace.GlobalEnv {
			activeVars, err := b.Variable.GetVariableByEnvID(ctx, workspace.ActiveEnv)
			if err != nil && !errors.Is(err, senv.ErrNoVarFound) {
				b.Logger.Warn("failed to get active environment variables", "env_id", workspace.ActiveEnv.String(), "error", err)
			} else {
				for _, v := range activeVars {
					if v.Enabled {
						baseVars[v.VarKey] = v.Value
					}
				}
			}
		}
	}

	// 3. Add flow-level variables (override environment variables)
	for _, variable := range flowVars {
		if variable.Enabled {
			baseVars[variable.Name] = variable.Value
		}
	}

	return baseVars, nil
}

func isZeroID(id idwrap.IDWrap) bool {
	return id == idwrap.IDWrap{}
}
