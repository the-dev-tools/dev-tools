//nolint:revive // exported
package rreference

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"connectrpc.com/connect"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/api"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/reference"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/referencecompletion"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/senv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sgraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	referencev1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/reference/v1"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/reference/v1/referencev1connect"
)

// --------------------------------------------------------------------------
// Service struct & constructor
// --------------------------------------------------------------------------

type ReferenceServiceRPC struct {
	DB *sql.DB

	userReader      *sworkspace.UserReader
	workspaceReader *sworkspace.WorkspaceReader

	// env
	envReader *senv.EnvReader
	varReader *senv.VariableReader

	// flow
	flowReader          *sflow.FlowReader
	nodeReader          *sflow.NodeReader
	nodeRequestReader   *sflow.NodeRequestReader
	flowVariableReader  *sflow.FlowVariableReader
	flowEdgeReader      *sflow.EdgeReader
	nodeExecutionReader *sflow.NodeExecutionReader

	// http
	httpResponseReader *shttp.HttpResponseReader

	// graphql
	graphqlResponseReader *sgraphql.GraphQLResponseService

	// sub-flow
	nodeSubFlowTriggerService *sflow.NodeSubFlowTriggerService
}

type ReferenceServiceRPCReaders struct {
	User          *sworkspace.UserReader
	Workspace     *sworkspace.WorkspaceReader
	Env           *senv.EnvReader
	Variable      *senv.VariableReader
	Flow          *sflow.FlowReader
	Node          *sflow.NodeReader
	NodeRequest   *sflow.NodeRequestReader
	FlowVariable  *sflow.FlowVariableReader
	FlowEdge      *sflow.EdgeReader
	NodeExecution     *sflow.NodeExecutionReader
	HttpResponse      *shttp.HttpResponseReader
	GraphQLResponse         *sgraphql.GraphQLResponseService
	NodeSubFlowTrigger      *sflow.NodeSubFlowTriggerService
}

func (r *ReferenceServiceRPCReaders) Validate() error {
	if r.User == nil {
		return fmt.Errorf("user reader is required")
	}
	if r.Workspace == nil {
		return fmt.Errorf("workspace reader is required")
	}
	if r.Env == nil {
		return fmt.Errorf("env reader is required")
	}
	if r.Variable == nil {
		return fmt.Errorf("variable reader is required")
	}
	if r.Flow == nil {
		return fmt.Errorf("flow reader is required")
	}
	if r.Node == nil {
		return fmt.Errorf("node reader is required")
	}
	if r.NodeRequest == nil {
		return fmt.Errorf("node request reader is required")
	}
	if r.FlowVariable == nil {
		return fmt.Errorf("flow variable reader is required")
	}
	if r.FlowEdge == nil {
		return fmt.Errorf("flow edge reader is required")
	}
	if r.NodeExecution == nil {
		return fmt.Errorf("node execution reader is required")
	}
	if r.HttpResponse == nil {
		return fmt.Errorf("http response reader is required")
	}
	if r.GraphQLResponse == nil {
		return fmt.Errorf("graphql response reader is required")
	}
	return nil
}

type ReferenceServiceRPCDeps struct {
	DB      *sql.DB
	Readers ReferenceServiceRPCReaders
}

func (d *ReferenceServiceRPCDeps) Validate() error {
	if d.DB == nil {
		return fmt.Errorf("db is required")
	}
	if err := d.Readers.Validate(); err != nil {
		return err
	}
	return nil
}

func NewReferenceServiceRPC(deps ReferenceServiceRPCDeps) *ReferenceServiceRPC {
	if err := deps.Validate(); err != nil {
		panic(fmt.Sprintf("ReferenceServiceRPC Deps validation failed: %v", err))
	}

	return &ReferenceServiceRPC{
		DB: deps.DB,

		userReader:      deps.Readers.User,
		workspaceReader: deps.Readers.Workspace,

		envReader: deps.Readers.Env,
		varReader: deps.Readers.Variable,

		flowReader:          deps.Readers.Flow,
		nodeReader:          deps.Readers.Node,
		nodeRequestReader:   deps.Readers.NodeRequest,
		flowVariableReader:  deps.Readers.FlowVariable,
		flowEdgeReader:      deps.Readers.FlowEdge,
		nodeExecutionReader: deps.Readers.NodeExecution,
		httpResponseReader:  deps.Readers.HttpResponse,
		graphqlResponseReader:     deps.Readers.GraphQLResponse,
		nodeSubFlowTriggerService: deps.Readers.NodeSubFlowTrigger,
	}
}

func CreateService(srv *ReferenceServiceRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := referencev1connect.NewReferenceServiceHandler(srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

// --------------------------------------------------------------------------
// Errors & helpers
// --------------------------------------------------------------------------

var (
	ErrExampleNotFound   = errors.New("example not found")
	ErrNodeNotFound      = errors.New("node not found")
	ErrWorkspaceNotFound = errors.New("workspace not found")
	ErrEnvNotFound       = errors.New("env not found")
)

// subFlowTriggerParamMap builds a variable map from sub-flow trigger params.
func (s *ReferenceServiceRPC) subFlowTriggerParamMap(ctx context.Context, nodeID idwrap.IDWrap) map[string]any {
	if s.nodeSubFlowTriggerService == nil {
		return map[string]any{}
	}
	trigger, err := s.nodeSubFlowTriggerService.GetNodeSubFlowTrigger(ctx, nodeID)
	if err != nil || trigger == nil {
		return map[string]any{}
	}
	m := make(map[string]any, len(trigger.Params))
	for _, p := range trigger.Params {
		switch p.Type {
		case "number":
			m[p.Name] = 0
		case "boolean":
			m[p.Name] = false
		case "json":
			m[p.Name] = map[string]any{}
		default:
			m[p.Name] = ""
		}
	}
	return m
}

// --------------------------------------------------------------------------
// Proto conversion
// --------------------------------------------------------------------------

const referenceKindProtoFallback = referencev1.ReferenceKind_REFERENCE_KIND_UNSPECIFIED

func referenceKindToProto(kind reference.ReferenceKind) (referencev1.ReferenceKind, error) {
	switch kind {
	case reference.ReferenceKind_REFERENCE_KIND_UNSPECIFIED:
		return referencev1.ReferenceKind_REFERENCE_KIND_UNSPECIFIED, nil
	case reference.ReferenceKind_REFERENCE_KIND_MAP:
		return referencev1.ReferenceKind_REFERENCE_KIND_MAP, nil
	case reference.ReferenceKind_REFERENCE_KIND_ARRAY:
		return referencev1.ReferenceKind_REFERENCE_KIND_ARRAY, nil
	case reference.ReferenceKind_REFERENCE_KIND_VALUE:
		return referencev1.ReferenceKind_REFERENCE_KIND_VALUE, nil
	case reference.ReferenceKind_REFERENCE_KIND_VARIABLE:
		return referencev1.ReferenceKind_REFERENCE_KIND_VARIABLE, nil
	default:
		return referenceKindProtoFallback, fmt.Errorf("unknown reference kind: %d", kind)
	}
}

var convertReferenceCompletionItemsFn = convertReferenceCompletionItems

func convertReferenceCompletionItems(items []referencecompletion.ReferenceCompletionItem) ([]*referencev1.ReferenceCompletion, error) {
	if len(items) == 0 {
		return nil, nil
	}
	converted := make([]*referencev1.ReferenceCompletion, 0, len(items))
	for _, item := range items {
		kind, err := referenceKindToProto(item.Kind)
		if err != nil {
			return nil, fmt.Errorf("reference kind to proto: %w", err)
		}
		converted = append(converted, &referencev1.ReferenceCompletion{
			Kind:         kind,
			EndToken:     item.EndToken,
			EndIndex:     item.EndIndex,
			ItemCount:    item.ItemCount,
			Environments: item.Environments,
		})
	}
	return converted, nil
}

// --------------------------------------------------------------------------
// RPC handlers
// --------------------------------------------------------------------------

func (c *ReferenceServiceRPC) ReferenceTree(ctx context.Context, req *connect.Request[referencev1.ReferenceTreeRequest]) (*connect.Response[referencev1.ReferenceTreeResponse], error) {
	params, err := parseReferenceContext(referenceContextMsg{
		WorkspaceID: req.Msg.WorkspaceId,
		HttpID:      req.Msg.HttpId,
		FlowNodeID:  req.Msg.FlowNodeId,
	})
	if err != nil {
		return nil, err
	}

	items, err := c.resolveTree(ctx, params)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&referencev1.ReferenceTreeResponse{Items: items}), nil
}

func (c *ReferenceServiceRPC) ReferenceCompletion(ctx context.Context, req *connect.Request[referencev1.ReferenceCompletionRequest]) (*connect.Response[referencev1.ReferenceCompletionResponse], error) {
	params, err := parseReferenceContext(referenceContextMsg{
		WorkspaceID: req.Msg.WorkspaceId,
		HttpID:      req.Msg.HttpId,
		GraphqlID:   req.Msg.GraphqlId,
		FlowNodeID:  req.Msg.FlowNodeId,
	})
	if err != nil {
		return nil, err
	}

	varMap, err := c.resolveVarMap(ctx, params)
	if err != nil {
		return nil, err
	}

	creator := referencecompletion.NewReferenceCompletionCreator()
	for k, v := range varMap {
		creator.AddWithKey(k, v)
	}

	items := creator.FindNextLevelCompletionData(req.Msg.Start)
	completions, err := convertReferenceCompletionItemsFn(items)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("convert reference completion items: %w", err))
	}

	return connect.NewResponse(&referencev1.ReferenceCompletionResponse{Items: completions}), nil
}

func (c *ReferenceServiceRPC) ReferenceValue(ctx context.Context, req *connect.Request[referencev1.ReferenceValueRequest]) (*connect.Response[referencev1.ReferenceValueResponse], error) {
	params, err := parseReferenceContext(referenceContextMsg{
		WorkspaceID: req.Msg.WorkspaceId,
		HttpID:      req.Msg.HttpId,
		GraphqlID:   req.Msg.GraphqlId,
		FlowNodeID:  req.Msg.FlowNodeId,
	})
	if err != nil {
		return nil, err
	}

	varMap, err := c.resolveVarMap(ctx, params)
	if err != nil {
		return nil, err
	}

	lookup := referencecompletion.NewReferenceCompletionLookup()
	for k, v := range varMap {
		lookup.AddWithKey(k, v)
	}

	value, err := lookup.GetValue(req.Msg.Path)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&referencev1.ReferenceValueResponse{Value: value}), nil
}

