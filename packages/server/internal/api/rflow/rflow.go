package rflow

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	devtoolsdb "the-dev-tools/db"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/ritemapiexample"
	"the-dev-tools/server/internal/api/rtag"
	"the-dev-tools/server/internal/api/rworkspace"
	"the-dev-tools/server/pkg/cachettl"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/nfor"
	"the-dev-tools/server/pkg/flow/node/nforeach"
	"the-dev-tools/server/pkg/flow/node/nif"
	"the-dev-tools/server/pkg/flow/node/njs"
	"the-dev-tools/server/pkg/flow/node/nnoop"
	"the-dev-tools/server/pkg/flow/node/nrequest"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/server/pkg/httpclient"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logconsole"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/massertres"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/model/mexamplerespheader"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/mnodeexecution"
	"the-dev-tools/server/pkg/model/mvar"
	"the-dev-tools/server/pkg/overlay/merge"
	"the-dev-tools/server/pkg/overlay/resolve"
	"the-dev-tools/server/pkg/permcheck"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/sassert"
	"the-dev-tools/server/pkg/service/sassertres"
	"the-dev-tools/server/pkg/service/sbodyform"
	"the-dev-tools/server/pkg/service/sbodyraw"
	"the-dev-tools/server/pkg/service/sbodyurl"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sexampleheader"
	"the-dev-tools/server/pkg/service/sexamplequery"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sexamplerespheader"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sflowtag"
	"the-dev-tools/server/pkg/service/sflowvariable"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodeexecution"
	"the-dev-tools/server/pkg/service/snodefor"
	"the-dev-tools/server/pkg/service/snodeforeach"
	"the-dev-tools/server/pkg/service/snodeif"
	"the-dev-tools/server/pkg/service/snodejs"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/soverlayheader"
	"the-dev-tools/server/pkg/service/soverlayquery"
	"the-dev-tools/server/pkg/service/stag"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/translate/tflow"
	"the-dev-tools/server/pkg/translate/tflowversion"
	"the-dev-tools/server/pkg/translate/tgeneric"
	nodev1 "the-dev-tools/spec/dist/buf/go/flow/node/v1"
	flowv1 "the-dev-tools/spec/dist/buf/go/flow/v1"
	"the-dev-tools/spec/dist/buf/go/flow/v1/flowv1connect"
	"the-dev-tools/spec/dist/buf/go/nodejs_executor/v1/nodejs_executorv1connect"
	"time"

	"connectrpc.com/connect"
)

var logOrphanedResponses = false

// formatErrForUser moved to logging.go

// normalizeForLog converts OutputData values into log-friendly forms:
// - []byte -> if JSON, unmarshal to any; else convert to string
// - map[string]any / []any -> recurse
// normalizeForLog moved to logging.go

// upsertWithRetry attempts to upsert a node execution with small retries for transient DB lock/busy errors.
// upsertWithRetry moved to retry.go

// preRegisteredRequestNode wraps a REQUEST node to handle pre-registration of ExecutionIDs
// This fixes the race condition where responses arrive before ExecutionID is added to pendingNodeExecutions
type preRegisteredRequestNode struct {
	nodeRequest             node.FlowNode
	preRegisteredExecutions map[idwrap.IDWrap]struct{}
	preRegisteredMutex      *sync.RWMutex
}

// Implement node.FlowNode interface
func (p *preRegisteredRequestNode) GetID() idwrap.IDWrap {
	return p.nodeRequest.GetID()
}

func (p *preRegisteredRequestNode) GetName() string {
	return p.nodeRequest.GetName()
}

func (p *preRegisteredRequestNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	// Pre-register the ExecutionID before running the request
	if req.ExecutionID != (idwrap.IDWrap{}) {
		p.preRegisteredMutex.Lock()
		p.preRegisteredExecutions[req.ExecutionID] = struct{}{}
		p.preRegisteredMutex.Unlock()
		// Pre-registration is internal bookkeeping; no user-facing log required
	}

	// Run the actual request node
	return p.nodeRequest.RunSync(ctx, req)
}

func (p *preRegisteredRequestNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	// Pre-register the ExecutionID before running the request
	if req.ExecutionID != (idwrap.IDWrap{}) {
		p.preRegisteredMutex.Lock()
		p.preRegisteredExecutions[req.ExecutionID] = struct{}{}
		p.preRegisteredMutex.Unlock()
		// Pre-registration is internal bookkeeping; no user-facing log required
	}

	// Run the actual request node
	p.nodeRequest.RunAsync(ctx, req, resultChan)
}

// buildLogPayload constructs structured log payloads for a node state change.
// Error-first behavior:
//   - If nodeError != nil, prefer an error payload with minimal node info and
//     error { message, kind } and optional failure context keys from outputData.
//   - Else, if outputData is a map, normalize and render it as-is.
//   - Else, fall back to a small metadata struct.
// buildLogPayload moved to logging.go

// formatIterationContext renders iteration-aware execution names using label segments.
func formatIterationContext(ctx *runner.IterationContext, nodeNameMap map[idwrap.IDWrap]string, nodeID idwrap.IDWrap, fallbackName string, isLoopNode bool, executionCount int) string {
	if ctx == nil || len(ctx.Labels) == 0 {
		name := fallbackName
		if name == "" {
			name = nodeNameMap[nodeID]
		}
		if name == "" {
			name = nodeID.String()
		}

		iteration := executionCount
		if iteration <= 0 {
			if ctx != nil {
				iteration = ctx.ExecutionIndex + 1
			}
			if iteration <= 0 {
				iteration = 1
			}
		}

		return fmt.Sprintf("%s Iteration %d", name, iteration)
	}

	segments := make([]string, 0, len(ctx.Labels)+1)
	for _, label := range ctx.Labels {
		displayName := label.Name
		if displayName == "" {
			displayName = nodeNameMap[label.NodeID]
		}
		if displayName == "" {
			displayName = label.NodeID.String()
		}
		segments = append(segments, fmt.Sprintf("%s Iteration %d", displayName, label.Iteration))
	}

	lastLabel := ctx.Labels[len(ctx.Labels)-1]
	if !isLoopNode || lastLabel.NodeID != nodeID {
		childName := fallbackName
		if childName == "" {
			childName = nodeNameMap[nodeID]
		}
		if childName != "" {
			segments = append(segments, childName)
		}
	}

	return strings.Join(segments, " | ")
}

const (
	cacheDefaultTTL      = 3 * time.Second
	cacheCleanupInterval = time.Minute
)

type FlowServiceRPC struct {
	DB *sql.DB
	ws sworkspace.WorkspaceService
	us suser.UserService
	ts stag.TagService

	// flow
	fs   sflow.FlowService
	fts  sflowtag.FlowTagService
	fes  sedge.EdgeService
	fvs  sflowvariable.FlowVariableService
	envs senv.EnvService
	vars svar.VarService

	// request
	ias        sitemapi.ItemApiService
	es         sitemapiexample.ItemApiExampleService
	qs         sexamplequery.ExampleQueryService
	hs         sexampleheader.HeaderService
	overlayMgr *merge.Manager

	// body
	brs  sbodyraw.BodyRawService
	bfs  sbodyform.BodyFormService
	bues sbodyurl.BodyURLEncodedService

	// response
	ers  sexampleresp.ExampleRespService
	erhs sexamplerespheader.ExampleRespHeaderService
	as   sassert.AssertService
	ars  sassertres.AssertResultService

	// sub nodes
	ns   snode.NodeService
	rns  snoderequest.NodeRequestService
	fns  snodefor.NodeForService
	fens snodeforeach.NodeForEachService
	sns  snodenoop.NodeNoopService
	ins  snodeif.NodeIfService
	jsns snodejs.NodeJSService

	// node execution
	nes snodeexecution.NodeExecutionService

	logChanMap logconsole.LogChanMap
	logger     *slog.Logger

	// caches to avoid hot-path DB fetches on repeated requests
	itemAPICache        *cachettl.Cache[idwrap.IDWrap, *mitemapi.ItemApi]
	itemApiExampleCache *cachettl.Cache[idwrap.IDWrap, *mitemapiexample.ItemApiExample]
	headerCache         *cachettl.Cache[idwrap.IDWrap, []mexampleheader.Header]
	queryCache          *cachettl.Cache[idwrap.IDWrap, []mexamplequery.Query]
	bodyRawCache        *cachettl.Cache[idwrap.IDWrap, *mbodyraw.ExampleBodyRaw]
	bodyFormCache       *cachettl.Cache[idwrap.IDWrap, []mbodyform.BodyForm]
	bodyURLCache        *cachettl.Cache[idwrap.IDWrap, []mbodyurl.BodyURLEncoded]
	assertCache         *cachettl.Cache[idwrap.IDWrap, []massert.Assert]
	exampleRespCache    *cachettl.Cache[idwrap.IDWrap, *mexampleresp.ExampleResp]
	respHeaderCache     *cachettl.Cache[idwrap.IDWrap, []mexamplerespheader.ExampleRespHeader]
}

func New(db *sql.DB, ws sworkspace.WorkspaceService, us suser.UserService, ts stag.TagService,
	// flow
	fs sflow.FlowService, fts sflowtag.FlowTagService, fes sedge.EdgeService, fvs sflowvariable.FlowVariableService, envs senv.EnvService, vars svar.VarService,
	// req
	ias sitemapi.ItemApiService, es sitemapiexample.ItemApiExampleService, qs sexamplequery.ExampleQueryService, hs sexampleheader.HeaderService,
	// body
	brs sbodyraw.BodyRawService, bfs sbodyform.BodyFormService, bues sbodyurl.BodyURLEncodedService,
	// resp
	ers sexampleresp.ExampleRespService, erhs sexamplerespheader.ExampleRespHeaderService, as sassert.AssertService, ars sassertres.AssertResultService,
	// sub nodes
	ns snode.NodeService, rns snoderequest.NodeRequestService, flns snodefor.NodeForService, fens snodeforeach.NodeForEachService,
	sns snodenoop.NodeNoopService, ins snodeif.NodeIfService, jsns snodejs.NodeJSService,
	// node execution
	nes snodeexecution.NodeExecutionService,
	logChanMap logconsole.LogChanMap,
	logger *slog.Logger,
) FlowServiceRPC {
	headerOverlay, _ := soverlayheader.New(db)
	queryOverlay, _ := soverlayquery.New(db)
	overlayMgr := merge.New(headerOverlay, queryOverlay)
	return FlowServiceRPC{
		DB: db,
		ws: ws,
		us: us,
		ts: ts,

		// flow
		fs:   fs,
		fes:  fes,
		fts:  fts,
		fvs:  fvs,
		envs: envs,
		vars: vars,

		// request
		ias:        ias,
		es:         es,
		qs:         qs,
		hs:         hs,
		overlayMgr: overlayMgr,

		// body
		brs:  brs,
		bfs:  bfs,
		bues: bues,

		// resp
		ers:  ers,
		erhs: erhs,
		as:   as,
		ars:  ars,

		// sub nodes
		ns:   ns,
		rns:  rns,
		fns:  flns,
		fens: fens,
		sns:  sns,
		ins:  ins,
		jsns: jsns,

		// node execution
		nes: nes,

		logChanMap: logChanMap,
		logger:     logger,

		itemAPICache:        cachettl.New[idwrap.IDWrap, *mitemapi.ItemApi](cacheDefaultTTL, cacheCleanupInterval),
		itemApiExampleCache: cachettl.New[idwrap.IDWrap, *mitemapiexample.ItemApiExample](cacheDefaultTTL, cacheCleanupInterval),
		headerCache:         cachettl.New[idwrap.IDWrap, []mexampleheader.Header](cacheDefaultTTL, cacheCleanupInterval),
		queryCache:          cachettl.New[idwrap.IDWrap, []mexamplequery.Query](cacheDefaultTTL, cacheCleanupInterval),
		bodyRawCache:        cachettl.New[idwrap.IDWrap, *mbodyraw.ExampleBodyRaw](cacheDefaultTTL, cacheCleanupInterval),
		bodyFormCache:       cachettl.New[idwrap.IDWrap, []mbodyform.BodyForm](cacheDefaultTTL, cacheCleanupInterval),
		bodyURLCache:        cachettl.New[idwrap.IDWrap, []mbodyurl.BodyURLEncoded](cacheDefaultTTL, cacheCleanupInterval),
		assertCache:         cachettl.New[idwrap.IDWrap, []massert.Assert](cacheDefaultTTL, cacheCleanupInterval),
		exampleRespCache:    cachettl.New[idwrap.IDWrap, *mexampleresp.ExampleResp](cacheDefaultTTL, cacheCleanupInterval),
		respHeaderCache:     cachettl.New[idwrap.IDWrap, []mexamplerespheader.ExampleRespHeader](cacheDefaultTTL, cacheCleanupInterval),
	}
}

func cloneItemAPI(src *mitemapi.ItemApi) *mitemapi.ItemApi {
	if src == nil {
		return nil
	}
	copyVal := *src
	return &copyVal
}

func cloneExample(src *mitemapiexample.ItemApiExample) *mitemapiexample.ItemApiExample {
	if src == nil {
		return nil
	}
	copyVal := *src
	return &copyVal
}

func cloneHeaders(src []mexampleheader.Header) []mexampleheader.Header {
	return append([]mexampleheader.Header(nil), src...)
}

func cloneQueries(src []mexamplequery.Query) []mexamplequery.Query {
	return append([]mexamplequery.Query(nil), src...)
}

func cloneFormBody(src []mbodyform.BodyForm) []mbodyform.BodyForm {
	return append([]mbodyform.BodyForm(nil), src...)
}

func cloneURLBody(src []mbodyurl.BodyURLEncoded) []mbodyurl.BodyURLEncoded {
	return append([]mbodyurl.BodyURLEncoded(nil), src...)
}

func cloneRawBody(src *mbodyraw.ExampleBodyRaw) *mbodyraw.ExampleBodyRaw {
	if src == nil {
		return nil
	}
	copyVal := *src
	copyVal.Data = append([]byte(nil), src.Data...)
	return &copyVal
}

func cloneExampleResp(src *mexampleresp.ExampleResp) *mexampleresp.ExampleResp {
	if src == nil {
		return nil
	}
	copyVal := *src
	copyVal.Body = append([]byte(nil), src.Body...)
	return &copyVal
}

func cloneRespHeaders(src []mexamplerespheader.ExampleRespHeader) []mexamplerespheader.ExampleRespHeader {
	return append([]mexamplerespheader.ExampleRespHeader(nil), src...)
}

func cloneAsserts(src []massert.Assert) []massert.Assert {
	return append([]massert.Assert(nil), src...)
}

func (c *FlowServiceRPC) loadItemApi(ctx context.Context, id idwrap.IDWrap) (*mitemapi.ItemApi, error) {
	if val, ok := c.itemAPICache.Get(id); ok {
		return val, nil
	}

	endpoint, err := c.ias.GetItemApi(ctx, id)
	if err != nil {
		return nil, err
	}

	c.itemAPICache.Set(id, endpoint)
	return endpoint, nil
}

func (c *FlowServiceRPC) loadItemApiExample(ctx context.Context, id idwrap.IDWrap) (*mitemapiexample.ItemApiExample, error) {
	if val, ok := c.itemApiExampleCache.Get(id); ok {
		return val, nil
	}

	example, err := c.es.GetApiExample(ctx, id)
	if err != nil {
		return nil, err
	}

	c.itemApiExampleCache.Set(id, example)
	return example, nil
}

func (c *FlowServiceRPC) loadHeaders(ctx context.Context, id idwrap.IDWrap) ([]mexampleheader.Header, error) {
	if val, ok := c.headerCache.Get(id); ok {
		return val, nil
	}

	headers, err := c.hs.GetHeaderByExampleID(ctx, id)
	if err != nil {
		return nil, err
	}

	c.headerCache.Set(id, headers)
	return headers, nil
}

func (c *FlowServiceRPC) loadQueries(ctx context.Context, id idwrap.IDWrap) ([]mexamplequery.Query, error) {
	if val, ok := c.queryCache.Get(id); ok {
		return val, nil
	}

	queries, err := c.qs.GetExampleQueriesByExampleID(ctx, id)
	if err != nil {
		if errors.Is(err, sexamplequery.ErrNoQueryFound) {
			c.queryCache.Set(id, []mexamplequery.Query{})
			return []mexamplequery.Query{}, nil
		}
		return nil, err
	}

	c.queryCache.Set(id, queries)
	return queries, nil
}

func (c *FlowServiceRPC) loadBodyRaw(ctx context.Context, id idwrap.IDWrap) (*mbodyraw.ExampleBodyRaw, error) {
	if val, ok := c.bodyRawCache.Get(id); ok {
		return val, nil
	}

	body, err := c.brs.GetBodyRawByExampleID(ctx, id)
	if err != nil {
		return nil, err
	}

	c.bodyRawCache.Set(id, body)
	return body, nil
}

func (c *FlowServiceRPC) loadBodyForms(ctx context.Context, id idwrap.IDWrap) ([]mbodyform.BodyForm, error) {
	if val, ok := c.bodyFormCache.Get(id); ok {
		return val, nil
	}

	forms, err := c.bfs.GetBodyFormsByExampleID(ctx, id)
	if err != nil {
		if errors.Is(err, sbodyform.ErrNoBodyFormFound) {
			c.bodyFormCache.Set(id, []mbodyform.BodyForm{})
			return []mbodyform.BodyForm{}, nil
		}
		return nil, err
	}

	c.bodyFormCache.Set(id, forms)
	return forms, nil
}

func (c *FlowServiceRPC) loadBodyURL(ctx context.Context, id idwrap.IDWrap) ([]mbodyurl.BodyURLEncoded, error) {
	if val, ok := c.bodyURLCache.Get(id); ok {
		return val, nil
	}

	values, err := c.bues.GetBodyURLEncodedByExampleID(ctx, id)
	if err != nil {
		return nil, err
	}

	c.bodyURLCache.Set(id, values)
	return values, nil
}

func (c *FlowServiceRPC) loadAsserts(ctx context.Context, id idwrap.IDWrap) ([]massert.Assert, error) {
	if val, ok := c.assertCache.Get(id); ok {
		return val, nil
	}

	asserts, err := c.as.GetAssertByExampleID(ctx, id)
	if err != nil {
		if errors.Is(err, sassert.ErrNoAssertFound) {
			c.assertCache.Set(id, []massert.Assert{})
			return []massert.Assert{}, nil
		}
		return nil, err
	}

	c.assertCache.Set(id, asserts)
	return asserts, nil
}

func (c *FlowServiceRPC) loadExampleResp(ctx context.Context, id idwrap.IDWrap) (*mexampleresp.ExampleResp, error) {
	if val, ok := c.exampleRespCache.Get(id); ok {
		return val, nil
	}

	resp, err := c.ers.GetExampleRespByExampleIDLatest(ctx, id)
	if err != nil {
		if errors.Is(err, sexampleresp.ErrNoRespFound) {
			c.exampleRespCache.Set(id, nil)
			return nil, nil
		}
		return nil, err
	}

	c.exampleRespCache.Set(id, resp)
	return resp, nil
}

func (c *FlowServiceRPC) loadRespHeaders(ctx context.Context, id idwrap.IDWrap) ([]mexamplerespheader.ExampleRespHeader, error) {
	if val, ok := c.respHeaderCache.Get(id); ok {
		return val, nil
	}

	headers, err := c.erhs.GetHeaderByRespID(ctx, id)
	if err != nil {
		return nil, err
	}

	c.respHeaderCache.Set(id, headers)
	return headers, nil
}
func CreateService(srv FlowServiceRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := flowv1connect.NewFlowServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (c *FlowServiceRPC) FlowList(ctx context.Context, req *connect.Request[flowv1.FlowListRequest]) (*connect.Response[flowv1.FlowListResponse], error) {
	workspaceID, err := idwrap.NewFromBytes(req.Msg.WorkspaceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	var tagIDPtr *idwrap.IDWrap
	if len(req.Msg.TagId) > 0 {
		tagID, err := idwrap.NewFromBytes(req.Msg.TagId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		tagIDPtr = &tagID
	}

	rpcErr := permcheck.CheckPerm(rworkspace.CheckOwnerWorkspace(ctx, c.us, workspaceID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	var rpcFlows []*flowv1.FlowListItem

	if tagIDPtr == nil {
		flow, err := c.fs.GetFlowsByWorkspaceID(ctx, workspaceID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		rpcFlows = append(rpcFlows, tgeneric.MassConvert(flow, tflow.SeralizeModelToRPCItem)...)

	} else {
		rpcErr := permcheck.CheckPerm(rtag.CheckOwnerTag(ctx, c.ts, c.us, *tagIDPtr))
		if rpcErr != nil {
			return nil, rpcErr
		}
		tagFlows, err := c.fts.GetFlowTagsByTagID(ctx, *tagIDPtr)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// TODO: make this one query
		for _, tagFlow := range tagFlows {
			latestFlow, err := c.fs.GetFlow(ctx, tagFlow.FlowID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			rpcFlow := tflow.SeralizeModelToRPCItem(latestFlow)
			rpcFlows = append(rpcFlows, rpcFlow)
		}
	}

	rpcResp := &flowv1.FlowListResponse{
		WorkspaceId: req.Msg.WorkspaceId,
		TagId:       req.Msg.TagId,
		Items:       rpcFlows,
	}
	return connect.NewResponse(rpcResp), nil
}

func (c *FlowServiceRPC) FlowGet(ctx context.Context, req *connect.Request[flowv1.FlowGetRequest]) (*connect.Response[flowv1.FlowGetResponse], error) {
	if len(req.Msg.FlowId) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow id is required"))
	}
	flowID, err := idwrap.NewFromBytes(req.Msg.FlowId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerFlow(ctx, c.fs, c.us, flowID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	flow, err := c.fs.GetFlow(ctx, flowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	rpcFlow := tflow.SeralizeModelToRPC(flow)
	rpcResp := &flowv1.FlowGetResponse{
		FlowId:   rpcFlow.FlowId,
		Name:     rpcFlow.Name,
		Duration: rpcFlow.Duration,
	}
	return connect.NewResponse(rpcResp), nil
}

func (c *FlowServiceRPC) FlowCreate(ctx context.Context, req *connect.Request[flowv1.FlowCreateRequest]) (*connect.Response[flowv1.FlowCreateResponse], error) {
	workspaceID, err := idwrap.NewFromBytes(req.Msg.WorkspaceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(rworkspace.CheckOwnerWorkspace(ctx, c.us, workspaceID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	ws, err := c.ws.Get(ctx, workspaceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	name := req.Msg.Name

	flowID := idwrap.NewNow()

	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        name,
	}
	if req.Msg.Duration != nil {
		flow.Duration = *req.Msg.Duration
	}

	nodeNoopID := idwrap.NewNow()

	tx, err := c.DB.Begin()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	txFlow, err := sflow.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txNode, err := snode.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txNoopNode, err := snodenoop.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = txFlow.CreateFlow(ctx, flow)
	if err != nil {
		return nil, err
	}

	err = txNode.CreateNode(ctx, mnnode.MNode{
		ID:        nodeNoopID,
		FlowID:    flowID,
		Name:      "Default Start Node",
		NodeKind:  mnnode.NODE_KIND_NO_OP,
		PositionX: float64(0),
		PositionY: float64(0),
	})
	if err != nil {
		return nil, err
	}
	err = txNoopNode.CreateNodeNoop(ctx, mnnoop.NoopNode{
		FlowNodeID: nodeNoopID,
		Type:       mnnoop.NODE_NO_OP_KIND_START,
	})
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	ws.FlowCount++
	ws.Updated = dbtime.DBNow()
	err = c.ws.Update(ctx, ws)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&flowv1.FlowCreateResponse{
		FlowId: flowID.Bytes(),
	}), nil
}

func (c *FlowServiceRPC) FlowUpdate(ctx context.Context, req *connect.Request[flowv1.FlowUpdateRequest]) (*connect.Response[flowv1.FlowUpdateResponse], error) {
	msg := req.Msg
	if msg.FlowId == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow id is required"))
	}
	if msg.Name == nil && msg.Duration == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("nothing to update"))
	}

	flowID, err := idwrap.NewFromBytes(msg.FlowId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerFlow(ctx, c.fs, c.us, flowID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	flow, err := c.fs.GetFlow(ctx, flowID)
	if err != nil {
		return nil, err
	}

	if msg.Name != nil {
		flow.Name = *msg.Name
	}
	if msg.Duration != nil {
		flow.Duration = *msg.Duration
	}

	err = c.fs.UpdateFlow(ctx, flow)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&flowv1.FlowUpdateResponse{}), nil
}

func (c *FlowServiceRPC) FlowDelete(ctx context.Context, req *connect.Request[flowv1.FlowDeleteRequest]) (*connect.Response[flowv1.FlowDeleteResponse], error) {
	flowID, err := idwrap.NewFromBytes(req.Msg.FlowId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerFlow(ctx, c.fs, c.us, flowID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	flow, err := c.fs.GetFlow(ctx, flowID)
	if err != nil {
		return nil, err
	}

	err = c.fs.DeleteFlow(ctx, flowID)
	if err != nil {
		return nil, err
	}

	ws, err := c.ws.Get(ctx, flow.WorkspaceID)
	if err != nil {
		return nil, err
	}

	ws.FlowCount--
	ws.Updated = dbtime.DBNow()
	err = c.ws.Update(ctx, ws)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&flowv1.FlowDeleteResponse{}), nil
}

func (c *FlowServiceRPC) cleanupNodeExecutions(ctx context.Context, flowID idwrap.IDWrap) error {
	// Get all nodes for this flow
	nodes, err := c.ns.GetNodesByFlowID(ctx, flowID)
	if err != nil {
		return fmt.Errorf("get nodes: %w", err)
	}

	// Collect node IDs for batch delete
	nodeIDs := make([]idwrap.IDWrap, len(nodes))
	for i, node := range nodes {
		nodeIDs[i] = node.ID
	}

	// Single batch delete - fast and simple
	return c.nes.DeleteNodeExecutionsByNodeIDs(ctx, nodeIDs)
}

func (c *FlowServiceRPC) FlowRun(ctx context.Context, req *connect.Request[flowv1.FlowRunRequest], stream *connect.ServerStream[flowv1.FlowRunResponse]) error {
	return c.FlowRunAdHoc(ctx, req, stream)
}

func buildLoopNodeExecutionFromStatus(flowNodeStatus runner.FlowNodeStatus, executionID idwrap.IDWrap) mnodeexecution.NodeExecution {
	completedAt := time.Now().UnixMilli()
	nodeExecution := mnodeexecution.NodeExecution{
		ID:                     executionID,
		NodeID:                 flowNodeStatus.NodeID,
		Name:                   flowNodeStatus.Name,
		State:                  flowNodeStatus.State,
		Error:                  nil,
		InputData:              []byte("{}"),
		InputDataCompressType:  0,
		OutputData:             []byte("{}"),
		OutputDataCompressType: 0,
		ResponseID:             nil,
		CompletedAt:            &completedAt,
	}

	if flowNodeStatus.State != mnnode.NODE_STATE_CANCELED && flowNodeStatus.Error != nil {
		errorStr := formatErrForUser(flowNodeStatus.Error)
		nodeExecution.Error = &errorStr
	}

	if flowNodeStatus.InputData != nil {
		if inputJSON, err := json.Marshal(flowNodeStatus.InputData); err == nil {
			if err := nodeExecution.SetInputJSON(inputJSON); err != nil {
				nodeExecution.InputData = inputJSON
				nodeExecution.InputDataCompressType = 0
			}
		}
	}

	if flowNodeStatus.OutputData != nil {
		if outputJSON, err := json.Marshal(flowNodeStatus.OutputData); err == nil {
			if err := nodeExecution.SetOutputJSON(outputJSON); err != nil {
				nodeExecution.OutputData = outputJSON
				nodeExecution.OutputDataCompressType = 0
			}
		}
	}

	return nodeExecution
}

func pruneIntermediateNoopNodes(
	edgeMap edge.EdgesMap,
	noopNodes []mnnoop.NoopNode,
	startNodeID idwrap.IDWrap,
	nodeNameMap map[idwrap.IDWrap]string,
) map[idwrap.IDWrap]struct{} {
	skipped := make(map[idwrap.IDWrap]struct{})

	for _, noop := range noopNodes {
		if noop.FlowNodeID == startNodeID {
			continue
		}
		skipped[noop.FlowNodeID] = struct{}{}
	}

	if len(skipped) == 0 {
		return skipped
	}

	containsID := func(list []idwrap.IDWrap, target idwrap.IDWrap) bool {
		for _, existing := range list {
			if existing == target {
				return true
			}
		}
		return false
	}

	for noopID := range skipped {
		targetsByHandle := edgeMap[noopID]
		delete(nodeNameMap, noopID)

		for sourceID, handles := range edgeMap {
			if sourceID == noopID {
				continue
			}
			for handle, targets := range handles {
				changed := false
				newTargets := make([]idwrap.IDWrap, 0, len(targets))
				for _, targetID := range targets {
					if targetID == noopID {
						changed = true
						replacements := targetsByHandle[handle]
						if len(replacements) == 0 && handle != edge.HandleUnspecified {
							replacements = targetsByHandle[edge.HandleUnspecified]
						}
						for _, replacement := range replacements {
							if replacement == noopID {
								continue
							}
							if !containsID(newTargets, replacement) {
								newTargets = append(newTargets, replacement)
							}
						}
						continue
					}
					newTargets = append(newTargets, targetID)
				}
				if changed {
					if len(newTargets) == 0 {
						delete(handles, handle)
					} else {
						handles[handle] = newTargets
					}
				}
			}
			if len(handles) == 0 {
				delete(edgeMap, sourceID)
			}
		}

		delete(edgeMap, noopID)
	}

	return skipped
}

func (c *FlowServiceRPC) FlowRunAdHoc(ctx context.Context, req *connect.Request[flowv1.FlowRunRequest], stream api.ServerStreamAdHoc[flowv1.FlowRunResponse]) error {
	flowID, err := idwrap.NewFromBytes(req.Msg.FlowId)
	if err != nil {
		return connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerFlow(ctx, c.fs, c.us, flowID))
	if rpcErr != nil {
		return rpcErr
	}

	flow, err := c.fs.GetFlow(ctx, flowID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	workspace, err := c.ws.Get(ctx, flow.WorkspaceID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	selectedEnvID := workspace.ActiveEnv
	if len(req.Msg.EnvironmentId) > 0 {
		selectedEnvID, err = idwrap.NewFromBytes(req.Msg.EnvironmentId)
		if err != nil {
			return connect.NewError(connect.CodeInvalidArgument, err)
		}
	}

	var globalVars []mvar.Var
	if workspace.GlobalEnv != (idwrap.IDWrap{}) {
		globalEnv, envErr := c.envs.Get(ctx, workspace.GlobalEnv)
		if envErr != nil {
			if !errors.Is(envErr, senv.ErrNoEnvironmentFound) {
				return connect.NewError(connect.CodeInternal, envErr)
			}
		} else if globalEnv.WorkspaceID != workspace.ID {
			return connect.NewError(connect.CodeInternal, errors.New("global environment does not belong to workspace"))
		} else {
			vars, varErr := c.vars.GetVariableByEnvID(ctx, workspace.GlobalEnv)
			if varErr != nil {
				if !errors.Is(varErr, svar.ErrNoVarFound) {
					return connect.NewError(connect.CodeInternal, varErr)
				}
			} else {
				globalVars = vars
			}
		}
	}

	var selectedVars []mvar.Var
	if selectedEnvID != (idwrap.IDWrap{}) {
		selectedEnv, envErr := c.envs.Get(ctx, selectedEnvID)
		if envErr != nil {
			if errors.Is(envErr, senv.ErrNoEnvironmentFound) {
				return connect.NewError(connect.CodeInvalidArgument, errors.New("environment not found"))
			}
			return connect.NewError(connect.CodeInternal, envErr)
		}
		if selectedEnv.WorkspaceID != workspace.ID {
			return connect.NewError(connect.CodeInvalidArgument, errors.New("environment does not belong to workspace"))
		}

		vars, varErr := c.vars.GetVariableByEnvID(ctx, selectedEnvID)
		if varErr != nil {
			if !errors.Is(varErr, svar.ErrNoVarFound) {
				return connect.NewError(connect.CodeInternal, varErr)
			}
		} else {
			selectedVars = vars
		}
	}

	flowVars, err := c.fvs.GetFlowVariablesByFlowID(ctx, flowID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	latestFlowID := flow.ID

	// Clean up old executions before starting
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cleanupCancel()

	if err := c.cleanupNodeExecutions(cleanupCtx, flowID); err != nil {
		// Log but don't fail the run
		log.Printf("Warning: Failed to cleanup old executions for flow %s: %v", flowID, err)
	}

	nodes, err := c.ns.GetNodesByFlowID(ctx, latestFlowID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, errors.New("get nodes"))
	}

	edges, err := c.fes.GetEdgesByFlowID(ctx, latestFlowID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, errors.New("get edges"))
	}
	edgeMap := edge.NewEdgesMap(edges)

	var requestNodes []mnrequest.MNRequest
	var forNodes []mnfor.MNFor
	var forEachNodes []mnforeach.MNForEach
	var ifNodes []mnif.MNIF
	var noopNodes []mnnoop.NoopNode
	var jsNodes []mnjs.MNJS
	var startNodeID idwrap.IDWrap

	nodeNameMap := make(map[idwrap.IDWrap]string, len(nodes))
	nodeKindMap := make(map[idwrap.IDWrap]mnnode.NodeKind, len(nodes))
	// Track loop node IDs for quick lookup when streaming statuses
	loopNodeIDs := make(map[idwrap.IDWrap]bool)
	// Track the last failed node/execution so we can correct terminal state in safety-net
	// kept for potential diagnostics in future; currently unused
	// var lastFailedMu sync.Mutex

	for _, node := range nodes {
		nodeNameMap[node.ID] = node.Name
		nodeKindMap[node.ID] = node.NodeKind

		switch node.NodeKind {
		case mnnode.NODE_KIND_REQUEST:
			rn, err := c.rns.GetNodeRequest(ctx, node.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("get node request: %w", err))
			}
			requestNodes = append(requestNodes, *rn)
		case mnnode.NODE_KIND_FOR:
			fn, err := c.fns.GetNodeFor(ctx, node.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("get node for: %w", err))
			}
			forNodes = append(forNodes, *fn)
		case mnnode.NODE_KIND_FOR_EACH:
			fen, err := c.fens.GetNodeForEach(ctx, node.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("get node for each: %w", err))
			}
			forEachNodes = append(forEachNodes, *fen)
		case mnnode.NODE_KIND_NO_OP:
			sn, err := c.sns.GetNodeNoop(ctx, node.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("get node start: %w", err))
			}
			noopNodes = append(noopNodes, *sn)
		case mnnode.NODE_KIND_CONDITION:
			in, err := c.ins.GetNodeIf(ctx, node.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, errors.New("get node if"))
			}
			ifNodes = append(ifNodes, *in)
		case mnnode.NODE_KIND_JS:
			jsn, err := c.jsns.GetNodeJS(ctx, node.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("get node js: %w", err))
			}
			jsNodes = append(jsNodes, jsn)
		default:
			return connect.NewError(connect.CodeInternal, errors.New("not supported node"))
		}
	}

	var foundStartNode bool
	for _, node := range noopNodes {
		if node.Type == mnnoop.NODE_NO_OP_KIND_START {
			if foundStartNode {
				return connect.NewError(connect.CodeInternal, errors.New("multiple start nodes"))
			}
			foundStartNode = true
			startNodeID = node.FlowNodeID
		}
	}
	if !foundStartNode {
		return connect.NewError(connect.CodeInternal, errors.New("no start node"))
	}

	prunedNoopIDs := pruneIntermediateNoopNodes(edgeMap, noopNodes, startNodeID, nodeNameMap)

	// Get flow variables first to check for timeout override
	flowVarsMap := make(map[string]any, len(flowVars)+len(globalVars)+len(selectedVars))
	for _, flowVar := range flowVars {
		if flowVar.Enabled {
			flowVarsMap[flowVar.Name] = flowVar.Value
		}
	}
	applyEnvVars := func(vars []mvar.Var) {
		for _, envVar := range vars {
			if envVar.IsEnabled() {
				flowVarsMap[envVar.VarKey] = envVar.Value
			}
		}
	}
	applyEnvVars(globalVars)
	applyEnvVars(selectedVars)

	// Create temporary request to safely read timeout variable
	tempReq := &node.FlowNodeRequest{
		VarMap:        flowVarsMap,
		ReadWriteLock: &sync.RWMutex{},
	}

	// Set default timeout to 60 seconds, check for timeout variable override
	nodeTimeout := time.Second * 60
	if timeoutVar, err := node.ReadVarRaw(tempReq, "timeout"); err == nil {
		if timeoutSeconds, ok := timeoutVar.(float64); ok && timeoutSeconds > 0 {
			nodeTimeout = time.Duration(timeoutSeconds) * time.Second
		} else if timeoutSecondsInt, ok := timeoutVar.(int); ok && timeoutSecondsInt > 0 {
			nodeTimeout = time.Duration(timeoutSecondsInt) * time.Second
		}
	}

	flowNodeMap := make(map[idwrap.IDWrap]node.FlowNode, 0)

	// Pre-registration system to fix REQUEST node response_id race condition
	preRegisteredExecutions := make(map[idwrap.IDWrap]struct{})
	preRegisteredMutex := sync.RWMutex{}

	for _, forNode := range forNodes {
		name := nodeNameMap[forNode.FlowNodeID]

		// Use the condition directly - no need to parse it here
		if forNode.Condition.Comparisons.Expression != "" {
			flowNodeMap[forNode.FlowNodeID] = nfor.NewWithCondition(forNode.FlowNodeID, name, forNode.IterCount, nodeTimeout, forNode.ErrorHandling, forNode.Condition)
		} else {
			flowNodeMap[forNode.FlowNodeID] = nfor.New(forNode.FlowNodeID, name, forNode.IterCount, nodeTimeout, forNode.ErrorHandling)
		}
		loopNodeIDs[forNode.FlowNodeID] = true
	}

	requestNodeRespChan := make(chan nrequest.NodeRequestSideResp, len(requestNodes))
	sharedHTTPClient := httpclient.New()
	for _, requestNode := range requestNodes {

		if requestNode.EndpointID == nil || requestNode.ExampleID == nil {
			return connect.NewError(connect.CodeInternal, fmt.Errorf("endpoint or example not found for %s", requestNode.FlowNodeID))
		}

		endpointModel, err := c.loadItemApi(ctx, *requestNode.EndpointID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		endpoint := cloneItemAPI(endpointModel)

		exampleModel, err := c.loadItemApiExample(ctx, *requestNode.ExampleID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		example := cloneExample(exampleModel)

		if example.ItemApiID != endpoint.ID {
			return connect.NewError(connect.CodeInternal, errors.New("example and endpoint not match"))
		}

		headersData, err := c.loadHeaders(ctx, example.ID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		headers := cloneHeaders(headersData)

		queriesData, err := c.loadQueries(ctx, example.ID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		queries := cloneQueries(queriesData)

		rawModel, err := c.loadBodyRaw(ctx, example.ID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		rawBody := cloneRawBody(rawModel)

		formModels, err := c.loadBodyForms(ctx, example.ID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		formBody := cloneFormBody(formModels)

		urlModels, err := c.loadBodyURL(ctx, example.ID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		urlBody := cloneURLBody(urlModels)

		exampleRespModel, err := c.loadExampleResp(ctx, example.ID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if exampleRespModel == nil {
			exampleRespModel = &mexampleresp.ExampleResp{
				ID:        idwrap.NewNow(),
				ExampleID: example.ID,
			}
			if createErr := c.ers.CreateExampleResp(ctx, *exampleRespModel); createErr != nil {
				return connect.NewError(connect.CodeInternal, createErr)
			}
			c.exampleRespCache.Set(example.ID, exampleRespModel)
		}
		exampleResp := cloneExampleResp(exampleRespModel)

		respHeaders, err := c.loadRespHeaders(ctx, exampleResp.ID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		exampleRespHeader := cloneRespHeaders(respHeaders)

		assertModels, err := c.loadAsserts(ctx, example.ID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		asserts := cloneAsserts(assertModels)

		resolveInput := resolve.RequestInput{
			BaseExample:  *example,
			BaseHeaders:  headers,
			BaseQueries:  queries,
			BaseRawBody:  rawBody,
			BaseFormBody: formBody,
			BaseURLBody:  urlBody,
			BaseAsserts:  asserts,
		}

		if requestNode.DeltaExampleID != nil {
			deltaExampleModel, err := c.loadItemApiExample(ctx, *requestNode.DeltaExampleID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			deltaExample := cloneExample(deltaExampleModel)

			if requestNode.DeltaEndpointID != nil {
				deltaEndpointModel, err := c.loadItemApi(ctx, *requestNode.DeltaEndpointID)
				if err != nil {
					return connect.NewError(connect.CodeInternal, err)
				}
				deltaEndpoint := cloneItemAPI(deltaEndpointModel)
				if deltaEndpoint.Url != "" {
					endpoint.Url = deltaEndpoint.Url
				}
				if deltaEndpoint.Method != "" {
					endpoint.Method = deltaEndpoint.Method
				}
			}

			deltaHeadersData, err := c.loadHeaders(ctx, deltaExample.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			deltaQueriesData, err := c.loadQueries(ctx, deltaExample.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}

			rawDeltaModel, err := c.loadBodyRaw(ctx, deltaExample.ID)
			var rawBodyDelta *mbodyraw.ExampleBodyRaw
			if err != nil {
				if errors.Is(err, sbodyraw.ErrNoBodyRawFound) {
					tmp := mbodyraw.ExampleBodyRaw{
						ID:            idwrap.NewNow(),
						ExampleID:     deltaExample.ID,
						VisualizeMode: mbodyraw.VisualizeModeBinary,
						CompressType:  compress.CompressTypeNone,
						Data:          []byte{},
					}
					rawBodyDelta = &tmp
				} else {
					return connect.NewError(connect.CodeInternal, err)
				}
			} else {
				rawBodyDelta = cloneRawBody(rawDeltaModel)
			}

			formDeltaModels, err := c.loadBodyForms(ctx, deltaExample.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			urlDeltaModels, err := c.loadBodyURL(ctx, deltaExample.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			deltaAssertModels, err := c.loadAsserts(ctx, deltaExample.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}

			resolveInput.DeltaExample = deltaExample
			resolveInput.DeltaHeaders = cloneHeaders(deltaHeadersData)
			resolveInput.DeltaQueries = cloneQueries(deltaQueriesData)
			resolveInput.DeltaRawBody = rawBodyDelta
			resolveInput.DeltaFormBody = cloneFormBody(formDeltaModels)
			resolveInput.DeltaURLBody = cloneURLBody(urlDeltaModels)
			resolveInput.DeltaAsserts = cloneAsserts(deltaAssertModels)
		}

		mergeOutput, err := resolve.Request(ctx, c.overlayMgr, resolveInput, requestNode.DeltaExampleID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		example = &mergeOutput.Merged
		headers = mergeOutput.MergeHeaders
		queries = mergeOutput.MergeQueries
		mergedRaw := mergeOutput.MergeRawBody
		rawBody = &mergedRaw
		formBody = mergeOutput.MergeFormBody
		urlBody = mergeOutput.MergeUrlEncodedBody
		asserts = mergeOutput.MergeAsserts

		name := nodeNameMap[requestNode.FlowNodeID]

		requestNodeInstance := nrequest.New(requestNode.FlowNodeID, name, *endpoint, *example, queries, headers, *rawBody, formBody, urlBody,
			*exampleResp, exampleRespHeader, asserts, sharedHTTPClient, requestNodeRespChan, c.logger)

		// Wrap with pre-registration logic
		wrappedNode := &preRegisteredRequestNode{
			nodeRequest:             requestNodeInstance,
			preRegisteredExecutions: preRegisteredExecutions,
			preRegisteredMutex:      &preRegisteredMutex,
		}

		flowNodeMap[requestNode.FlowNodeID] = wrappedNode
	}

	for _, ifNode := range ifNodes {
		comp := ifNode.Condition
		name := nodeNameMap[ifNode.FlowNodeID]
		flowNodeMap[ifNode.FlowNodeID] = nif.New(ifNode.FlowNodeID, name, comp)
	}

	for _, noopNode := range noopNodes {
		if _, skipped := prunedNoopIDs[noopNode.FlowNodeID]; skipped {
			continue
		}
		name := nodeNameMap[noopNode.FlowNodeID]
		flowNodeMap[noopNode.FlowNodeID] = nnoop.New(noopNode.FlowNodeID, name)
	}

	for _, forEachNode := range forEachNodes {
		name := nodeNameMap[forEachNode.FlowNodeID]
		flowNodeMap[forEachNode.FlowNodeID] = nforeach.New(forEachNode.FlowNodeID, name, forEachNode.IterExpression, nodeTimeout,
			forEachNode.Condition, forEachNode.ErrorHandling)
		loopNodeIDs[forEachNode.FlowNodeID] = true
	}

	var clientPtr *nodejs_executorv1connect.NodeJSExecutorServiceClient
	for i, jsNode := range jsNodes {
		if i == 0 {
			client := nodejs_executorv1connect.NewNodeJSExecutorServiceClient(httpclient.New(), "http://localhost:9090")
			clientPtr = &client
		}

		if jsNode.CodeCompressType != compress.CompressTypeNone {
			jsNode.Code, err = compress.Decompress(jsNode.Code, jsNode.CodeCompressType)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		}

		name := nodeNameMap[jsNode.FlowNodeID]

		flowNodeMap[jsNode.FlowNodeID] = njs.New(jsNode.FlowNodeID, name, string(jsNode.Code), *clientPtr)
	}

	// Use the same timeout for the flow runner
	runnerID := idwrap.NewNow()
	runnerInst := flowlocalrunner.CreateFlowRunner(runnerID, latestFlowID, startNodeID, flowNodeMap, edgeMap, nodeTimeout, c.logger)
	runnerInst.SetExecutionMode(flowlocalrunner.ExecutionModeAuto)

	// Calculate buffer size based on expected load
	// For large iteration counts, we need bigger buffers to prevent blocking
	bufferSize := 10000
	if forNodeCount := len(forNodes); forNodeCount > 0 {
		// Estimate based on for node iterations
		var maxIterations int64
		for _, fn := range forNodes {
			if fn.IterCount > maxIterations {
				maxIterations = fn.IterCount
			}
		}
		// Buffer should handle at least all iterations * nodes
		estimatedSize := int(maxIterations) * len(nodes) * 2
		if estimatedSize > bufferSize {
			bufferSize = estimatedSize
		}
	}

	nodeStateChan := make(chan runner.FlowNodeStatus, bufferSize)
	nodeLogChan := make(chan runner.FlowNodeLogPayload, bufferSize)
	flowStatusChan := make(chan runner.FlowStatus, 100)

	// Create a new context without the gRPC deadline for flow execution
	// The flow runner will apply its own timeout (nodeTimeout)
	subCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	var doneOnce sync.Once
	signalDone := func(err error) {
		doneOnce.Do(func() {
			done <- err
		})
	}

	type streamSendRequest struct {
		fn    func() error
		errCh chan error
	}

	streamSendCh := make(chan streamSendRequest, 128)
	var streamWg sync.WaitGroup
	streamWg.Add(1)
	go func() {
		defer streamWg.Done()
		for req := range streamSendCh {
			err := req.fn()
			req.errCh <- err
			close(req.errCh)
		}
	}()
	defer func() {
		close(streamSendCh)
		streamWg.Wait()
	}()

	enqueueStreamSend := func(fn func() error) error {
		errCh := make(chan error, 1)
		streamSendCh <- streamSendRequest{fn: fn, errCh: errCh}
		return <-errCh
	}
	sendNodeStatusSync := func(nodeID idwrap.IDWrap, state nodev1.NodeState, info *string) error {
		return enqueueStreamSend(func() error { return sendNodeStatus(stream, nodeID, state, info) })
	}
	sendExampleResponseSync := func(exampleID, responseID idwrap.IDWrap) error {
		return enqueueStreamSend(func() error { return sendExampleResponse(stream, exampleID, responseID) })
	}
	sendFlowResponseSync := func(resp *flowv1.FlowRunResponse) error {
		return enqueueStreamSend(func() error { return stream.Send(resp) })
	}
	nodeExecutionChan := make(chan mnodeexecution.NodeExecution, bufferSize)

	// Collector goroutine for node executions
	var nodeExecutions []mnodeexecution.NodeExecution
	nodeExecutionsDone := make(chan struct{})

	// Track execution counts per node for naming
	var nodeExecutionCounters sync.Map                // nodeID -> *atomic.Int64
	executionIDToCount := make(map[idwrap.IDWrap]int) // executionID -> execution number
	executionIDToCountMutex := sync.Mutex{}
	executionDisplayNames := make(map[idwrap.IDWrap]string)
	executionDisplayNamesMutex := sync.RWMutex{}

	getOrCreateCounter := func(nodeID idwrap.IDWrap) *atomic.Int64 {
		if nodeID == (idwrap.IDWrap{}) {
			return nil
		}
		counterAny, _ := nodeExecutionCounters.LoadOrStore(nodeID, &atomic.Int64{})
		return counterAny.(*atomic.Int64)
	}

	ensureExecutionCount := func(nodeID, executionID idwrap.IDWrap) int {
		if executionID == (idwrap.IDWrap{}) {
			if counter := getOrCreateCounter(nodeID); counter != nil {
				return int(counter.Add(1))
			}
			return 0
		}
		executionIDToCountMutex.Lock()
		defer executionIDToCountMutex.Unlock()
		if count, ok := executionIDToCount[executionID]; ok {
			return count
		}
		counter := getOrCreateCounter(nodeID)
		if counter == nil {
			executionIDToCount[executionID] = 0
			return 0
		}
		count := int(counter.Add(1))
		executionIDToCount[executionID] = count
		return count
	}

	lookupExecutionCount := func(executionID idwrap.IDWrap) (int, bool) {
		executionIDToCountMutex.Lock()
		defer executionIDToCountMutex.Unlock()
		count, ok := executionIDToCount[executionID]
		return count, ok
	}

	currentNodeExecutionCount := func(nodeID idwrap.IDWrap) int {
		if counterAny, ok := nodeExecutionCounters.Load(nodeID); ok {
			return int(counterAny.(*atomic.Int64).Load())
		}
		return 0
	}

	// Map to store node executions by execution ID for state transitions
	pendingNodeExecutions := make(map[idwrap.IDWrap]*mnodeexecution.NodeExecution)

	// Map to store orphaned responses that arrive before ExecutionID is registered
	orphanedResponses := make(map[idwrap.IDWrap]struct {
		ResponseID idwrap.IDWrap
		Timestamp  int64
	})

	pendingMutex := sync.Mutex{}

	// WaitGroup to track all goroutines that send to channels
	var goroutineWg sync.WaitGroup

	// Channel to signal that sending should stop
	stopSending := make(chan struct{})

	// Atomic flag to prevent sends after closure initiated
	var channelsClosed atomic.Bool

	// Collector goroutine for node executions
	goroutineWg.Add(1)
	go func() {
		defer goroutineWg.Done()
		defer close(nodeExecutionsDone)
		for {
			select {
			case execution, ok := <-nodeExecutionChan:
				if !ok {
					return
				}
				nodeExecutions = append(nodeExecutions, execution)
			case <-stopSending:
				// Drain remaining messages
				for {
					select {
					case execution, ok := <-nodeExecutionChan:
						if !ok {
							return
						}
						nodeExecutions = append(nodeExecutions, execution)
					default:
						return
					}
				}
			}
		}
	}()

	processLogPayload := func(payload runner.FlowNodeLogPayload) {
		if payload.State == mnnode.NODE_STATE_RUNNING {
			return
		}

		executionID := payload.ExecutionID
		nameForLog := payload.Name
		if executionID != (idwrap.IDWrap{}) {
			executionDisplayNamesMutex.RLock()
			if display, ok := executionDisplayNames[executionID]; ok && display != "" {
				nameForLog = display
			}
			executionDisplayNamesMutex.RUnlock()
		}

		if (nameForLog == "" || nameForLog == payload.Name) && payload.Name != "" {
			if execCount, ok := lookupExecutionCount(executionID); ok && execCount > 0 {
				nameForLog = fmt.Sprintf("%s - Execution %d", payload.Name, execCount)
			} else if executionID != (idwrap.IDWrap{}) {
				if execCount := ensureExecutionCount(payload.NodeID, executionID); execCount > 0 {
					nameForLog = fmt.Sprintf("%s - Execution %d", payload.Name, execCount)
				}
			} else if count := currentNodeExecutionCount(payload.NodeID); count > 0 {
				nameForLog = fmt.Sprintf("%s - Execution %d", payload.Name, count)
			}
		}

		if nameForLog == "" {
			nameForLog = payload.NodeID.String()
		}

		if payload.State == mnnode.NODE_STATE_RUNNING {
			return
		}

		stateStrForLog := mnnode.StringNodeState(payload.State)
		idStrForLog := payload.NodeID.String()
		logPayload := buildLogPayload(nameForLog, idStrForLog, stateStrForLog, payload.Error, payload.OutputData)
		if logPayload == nil {
			logPayload = map[string]any{}
		}
		message := map[string]any{nameForLog: logPayload}

		logLevel := logconsole.LogLevelUnspecified
		if payload.Error != nil {
			logLevel = logconsole.LogLevelError
		}

		if channelsClosed.Load() {
			return
		}

		if err := c.logChanMap.SendMsgToUserWithContext(ctx, idwrap.NewNow(), fmt.Sprintf("%s: %s", nameForLog, stateStrForLog), logLevel, message); err != nil {
			if !channelsClosed.Load() {
				signalDone(err)
			}
			log.Printf("Failed to send log error to done channel: %v", err)
		}
	}

	// Log processing goroutine
	goroutineWg.Add(1)
	go func() {
		defer goroutineWg.Done()
		for {
			select {
			case payload, ok := <-nodeLogChan:
				if !ok {
					return
				}
				processLogPayload(payload)
			case <-stopSending:
				for {
					select {
					case payload, ok := <-nodeLogChan:
						if !ok {
							return
						}
						processLogPayload(payload)
					default:
						return
					}
				}
			}
		}
	}()

	processRequestResponse := func(requestNodeResp nrequest.NodeRequestSideResp) error {
		err := c.HandleExampleChanges(ctx, requestNodeResp)
		if err != nil {
			log.Println("cannot update example on flow run", err)
		}

		pendingMutex.Lock()
		targetExecutionID := requestNodeResp.ExecutionID
		responseReceivedTime := time.Now()

		if targetExecutionID != (idwrap.IDWrap{}) && requestNodeResp.Resp.ExampleResp.ID != (idwrap.IDWrap{}) {
			if nodeExec, exists := pendingNodeExecutions[targetExecutionID]; exists {
				respID := requestNodeResp.Resp.ExampleResp.ID
				nodeExec.ResponseID = &respID

				if err := persistUpsert2s(c.nes, *nodeExec); err != nil {
					log.Printf("Failed to upsert node execution with response %s: %v", nodeExec.ID, err)
				}

				if nodeExec.CompletedAt != nil && !channelsClosed.Load() {
					select {
					case nodeExecutionChan <- *nodeExec:
						delete(pendingNodeExecutions, targetExecutionID)
					case <-stopSending:
						// Channel closed, don't send
					}
				} else if nodeExec.CompletedAt != nil {
					delete(pendingNodeExecutions, targetExecutionID)
				}

				for i := range nodeExecutions {
					if nodeExecutions[i].ID == targetExecutionID {
						nodeExecutions[i].ResponseID = &respID
						break
					}
				}
			} else {
				if logOrphanedResponses {
					log.Printf("No pending execution found for response %s (pending: %d, orphaned: %d)",
						targetExecutionID.String(), len(pendingNodeExecutions), len(orphanedResponses))
				}

				respID := requestNodeResp.Resp.ExampleResp.ID
				orphanedResponses[targetExecutionID] = struct {
					ResponseID idwrap.IDWrap
					Timestamp  int64
				}{
					ResponseID: respID,
					Timestamp:  responseReceivedTime.UnixMilli(),
				}
			}
		}
		pendingMutex.Unlock()

		if localErr := sendExampleResponseSync(requestNodeResp.Example.ID, requestNodeResp.Resp.ExampleResp.ID); localErr != nil {
			return localErr
		}
		return nil
	}

	// Removed periodic timeout/cleanup goroutine to simplify flow.

	// Main state processing goroutine
	goroutineWg.Add(1)
	go func() {
		defer goroutineWg.Done()
		nodeStatusFunc := func(flowNodeStatus runner.FlowNodeStatus) {
			// Check if we should stop processing
			if channelsClosed.Load() {
				return
			}

			id := flowNodeStatus.NodeID
			name := flowNodeStatus.Name
			executionID := flowNodeStatus.ExecutionID
			displayName := name
			defer func() {
				if executionID == (idwrap.IDWrap{}) {
					return
				}
				executionDisplayNamesMutex.Lock()
				if displayName != "" {
					executionDisplayNames[executionID] = displayName
				} else {
					delete(executionDisplayNames, executionID)
				}
				executionDisplayNamesMutex.Unlock()
			}()

			// Handle NodeExecution creation/updates based on state
			switch flowNodeStatus.State {
			case mnnode.NODE_STATE_RUNNING:
				// Check if this is an iteration tracking record via explicit flag
				isIterationRecord := flowNodeStatus.IterationEvent

				// Create new NodeExecution for RUNNING state
				pendingMutex.Lock()
				if _, exists := pendingNodeExecutions[executionID]; !exists {
					// Generate execution name with hierarchical format for loop iterations
					var execName string
					if ctx := flowNodeStatus.IterationContext; ctx != nil && len(ctx.Labels) > 0 {
						isLoopNode := loopNodeIDs[id]

						execCount := ensureExecutionCount(id, executionID)
						if execCount == 0 {
							execCount = currentNodeExecutionCount(id)
						}
						execName = formatIterationContext(ctx, nodeNameMap, id, flowNodeStatus.Name, isLoopNode, execCount)
					} else if flowNodeStatus.Name != "" {
						execCount := ensureExecutionCount(id, executionID)
						if execCount == 0 {
							execCount = currentNodeExecutionCount(id)
						}
						execName = fmt.Sprintf("%s - Execution %d", flowNodeStatus.Name, execCount)
					} else {
						// Fallback to execution count
						execCount := ensureExecutionCount(id, executionID)
						if execCount == 0 {
							execCount = currentNodeExecutionCount(id)
							if execCount == 0 {
								execCount = 1
							}
						}
						execName = fmt.Sprintf("Execution %d", execCount)
					}

					nodeExecution := mnodeexecution.NodeExecution{
						ID:                     executionID, // Use executionID as the record ID
						NodeID:                 id,
						Name:                   execName,
						State:                  flowNodeStatus.State,
						Error:                  nil,
						InputData:              []byte("{}"),
						InputDataCompressType:  0,
						OutputData:             []byte("{}"),
						OutputDataCompressType: 0,
						ResponseID:             nil,
						CompletedAt:            nil,
					}
					displayName = nodeExecution.Name

					// Set output data for iteration tracking records
					if isIterationRecord && flowNodeStatus.OutputData != nil {
						if outputJSON, err := json.Marshal(flowNodeStatus.OutputData); err == nil {
							if err := nodeExecution.SetOutputJSON(outputJSON); err != nil {
								nodeExecution.OutputData = outputJSON
								nodeExecution.OutputDataCompressType = 0
							}
						}
					}

					// For iteration tracking records, create immediately to avoid race condition
					// Save ALL iterations to database (both successful and failed)
					if isIterationRecord {
						// ALWAYS save ALL iteration records to database
						if err := c.nes.UpsertNodeExecution(ctx, nodeExecution); err != nil {
							log.Printf("Failed to upsert iteration record %s: %v", executionID.String(), err)
						}
					} else {
						// Check if this is a FOR/FOREACH loop node main execution using cached metadata
						isLoopNode := loopNodeIDs[id]

						if !isLoopNode {
							// Store in pending map for completion (normal flow execution - will be sent to UI)
							pendingNodeExecutions[executionID] = &nodeExecution
						}

						// Only handle orphaned responses for non-loop nodes (loop nodes don't use pending system)
						if !isLoopNode {
							// Check if there's an orphaned response waiting for this ExecutionID
							if orphaned, exists := orphanedResponses[executionID]; exists {
								nodeExecution.ResponseID = &orphaned.ResponseID

								// Update in database immediately (synchronously) to avoid regressing terminal updates later
								if err := persistUpsert2s(c.nes, nodeExecution); err != nil {
									log.Printf("Failed to upsert delayed correlation %s: %v", nodeExecution.ID, err)
								}

								// Remove from orphaned responses now that we've applied it
								delete(orphanedResponses, executionID)
							}

							// Persist RUNNING state synchronously to preserve ordering
							if err := persistUpsert2s(c.nes, nodeExecution); err != nil {
								log.Printf("Failed to upsert node execution %s: %v", nodeExecution.ID, err)
							}
						}
					}
				}
				pendingMutex.Unlock()

			case mnnode.NODE_STATE_SUCCESS, mnnode.NODE_STATE_FAILURE, mnnode.NODE_STATE_CANCELED:
				// Check if this is an iteration tracking record via explicit flag
				isIterationRecord := flowNodeStatus.IterationEvent

				// Get node kind from cache for REQUEST/loop handling
				kind := nodeKindMap[flowNodeStatus.NodeID]

				// Handle iteration records separately (they need updates, not pending lookups)
				if isIterationRecord {
					// Create update record for ALL iterations (both successful and failed)
					completedAt := time.Now().UnixMilli()
					nodeExecution := mnodeexecution.NodeExecution{
						ID:          executionID, // Use same ExecutionID for update
						State:       flowNodeStatus.State,
						CompletedAt: &completedAt,
					}

					// Set error if present
					if flowNodeStatus.Error != nil {
						errorStr := formatErrForUser(flowNodeStatus.Error)
						nodeExecution.Error = &errorStr
					}

					// Compress and store output data
					if flowNodeStatus.OutputData != nil {
						if outputJSON, err := json.Marshal(flowNodeStatus.OutputData); err == nil {
							if err := nodeExecution.SetOutputJSON(outputJSON); err != nil {
								nodeExecution.OutputData = outputJSON
								nodeExecution.OutputDataCompressType = 0
							}
						}
					}

					// Upsert ALL iteration records synchronously to avoid stale RUNNING state after cancel
					if err := persistUpsert2s(c.nes, nodeExecution); err != nil {
						log.Printf("Failed to upsert iteration record %s: %v", nodeExecution.ID.String(), err)
					}
					// ALL iterations are now persisted to database
				} else {
					// Update existing NodeExecution with final state (normal flow)
					pendingMutex.Lock()
					if nodeExec, exists := pendingNodeExecutions[executionID]; exists {
						displayName = nodeExec.Name
						// Update final state
						nodeExec.State = flowNodeStatus.State
						completedAt := time.Now().UnixMilli()
						nodeExec.CompletedAt = &completedAt

						// Track last failure for safety-net correction later
						// Track last failure (kept for potential diagnostics)
						// if flowNodeStatus.State == mnnode.NODE_STATE_FAILURE {
						//     lastFailedMu.Lock()
						//     _ = flowNodeStatus
						//     _ = executionID
						//     lastFailedMu.Unlock()
						// }

						// Set error if present
						if flowNodeStatus.Error != nil {
							errorStr := formatErrForUser(flowNodeStatus.Error)
							nodeExec.Error = &errorStr
						}

						// Compress and store input data
						if flowNodeStatus.InputData != nil {
							if inputJSON, err := json.Marshal(flowNodeStatus.InputData); err == nil {
								if err := nodeExec.SetInputJSON(inputJSON); err != nil {
									nodeExec.InputData = inputJSON
									nodeExec.InputDataCompressType = 0
								}
							}
						}

						// Compress and store output data
						if flowNodeStatus.OutputData != nil {
							if outputJSON, err := json.Marshal(flowNodeStatus.OutputData); err == nil {
								if err := nodeExec.SetOutputJSON(outputJSON); err != nil {
									nodeExec.OutputData = outputJSON
									nodeExec.OutputDataCompressType = 0
								}
							}
						}

						// Upsert execution in DB synchronously to ensure terminal state is persisted
						if err := persistUpsert2s(c.nes, *nodeExec); err != nil {
							log.Printf("Failed to upsert node execution %s: %v", nodeExec.ID, err)
						}

						// For REQUEST nodes, wait for response before sending to channel.
						// If the response already arrived, we can forward the execution now.
						if kind == mnnode.NODE_KIND_REQUEST {
							if nodeExec.ResponseID != nil && !channelsClosed.Load() {
								select {
								case nodeExecutionChan <- *nodeExec:
									delete(pendingNodeExecutions, executionID)
								case <-stopSending:
									// Channel closed, don't send
								}
							}
						} else {
							// For non-REQUEST nodes, send immediately (with safety check)
							if !channelsClosed.Load() {
								select {
								case nodeExecutionChan <- *nodeExec:
									delete(pendingNodeExecutions, executionID)
								case <-stopSending:
									// Channel closed, don't send
								}
							}
						}
					} else if loopNodeIDs[flowNodeStatus.NodeID] {
						nodeExecution := buildLoopNodeExecutionFromStatus(flowNodeStatus, executionID)
						displayName = nodeExecution.Name

						if err := persistUpsert2s(c.nes, nodeExecution); err != nil {
							log.Printf("Failed to upsert loop node execution %s: %v", nodeExecution.ID, err)
						}

						if !channelsClosed.Load() {
							select {
							case nodeExecutionChan <- nodeExecution:
							case <-stopSending:
							}
						}
					} else if flowNodeStatus.State == mnnode.NODE_STATE_CANCELED {
						// Handle the case where we receive a CANCELED status without a prior RUNNING status
						// This can happen when nodes are canceled before they start executing
						completedAt := time.Now().UnixMilli()

						// Get execution name
						var execName string
						if flowNodeStatus.Name != "" {
							execCount := ensureExecutionCount(flowNodeStatus.NodeID, executionID)
							if execCount == 0 {
								execCount = currentNodeExecutionCount(flowNodeStatus.NodeID)
							}
							execName = fmt.Sprintf("%s - Execution %d", flowNodeStatus.Name, execCount)
						} else {
							execName = "Canceled Node"
						}

						nodeExecution := mnodeexecution.NodeExecution{
							ID:                     executionID,
							NodeID:                 flowNodeStatus.NodeID,
							Name:                   execName,
							State:                  mnnode.NODE_STATE_CANCELED,
							Error:                  nil,
							InputData:              []byte("{}"),
							InputDataCompressType:  0,
							OutputData:             []byte("{}"),
							OutputDataCompressType: 0,
							ResponseID:             nil,
							CompletedAt:            &completedAt,
						}
						displayName = nodeExecution.Name

						// Set error if present
						if flowNodeStatus.Error != nil {
							errorStr := formatErrForUser(flowNodeStatus.Error)
							nodeExecution.Error = &errorStr
						}

						// Compress and store input data if available
						if flowNodeStatus.InputData != nil {
							if inputJSON, err := json.Marshal(flowNodeStatus.InputData); err == nil {
								if err := nodeExecution.SetInputJSON(inputJSON); err != nil {
									nodeExecution.InputData = inputJSON
									nodeExecution.InputDataCompressType = 0
								}
							}
						}

						// Compress and store output data if available
						if flowNodeStatus.OutputData != nil {
							if outputJSON, err := json.Marshal(flowNodeStatus.OutputData); err == nil {
								if err := nodeExecution.SetOutputJSON(outputJSON); err != nil {
									nodeExecution.OutputData = outputJSON
									nodeExecution.OutputDataCompressType = 0
								}
							}
						}

						// Track last failure for safety-net correction later
						// Track last failure (kept for potential diagnostics)
						// if flowNodeStatus.State == mnnode.NODE_STATE_FAILURE {
						//     lastFailedMu.Lock()
						//     _ = flowNodeStatus
						//     _ = executionID
						//     lastFailedMu.Unlock()
						// }

						// Upsert to DB synchronously to ensure cancellation is persisted before navigation
						if err := persistUpsert2s(c.nes, nodeExecution); err != nil {
							log.Printf("Failed to upsert canceled node execution %s: %v", nodeExecution.ID, err)
						}

						// Send immediately for canceled nodes (with safety check)
						if !channelsClosed.Load() {
							select {
							case nodeExecutionChan <- nodeExecution:
								// Successfully sent
							case <-stopSending:
								// Channel closed, don't send
							}
						}
					} else {
						// Handle terminal (failure/canceled) loop nodes that weren't in pending
						// (because we skip successful loop main executions)
						if (kind == mnnode.NODE_KIND_FOR || kind == mnnode.NODE_KIND_FOR_EACH) &&
							(flowNodeStatus.State == mnnode.NODE_STATE_FAILURE || flowNodeStatus.State == mnnode.NODE_STATE_CANCELED) {

							// Create execution record for failed loop nodes (these should be visible in UI)
							completedAt := time.Now().UnixMilli()

							// Get execution name
							var execName string
							if flowNodeStatus.Name != "" {
								execCount := ensureExecutionCount(flowNodeStatus.NodeID, executionID)
								if execCount == 0 {
									execCount = currentNodeExecutionCount(flowNodeStatus.NodeID)
								}
								execName = fmt.Sprintf("%s - Execution %d", flowNodeStatus.Name, execCount)
							} else {
								execName = "Failed Loop"
							}

							nodeExecution := mnodeexecution.NodeExecution{
								ID:                     executionID,
								NodeID:                 flowNodeStatus.NodeID,
								Name:                   execName,
								State:                  flowNodeStatus.State,
								Error:                  nil,
								InputData:              []byte("{}"),
								InputDataCompressType:  0,
								OutputData:             []byte("{}"),
								OutputDataCompressType: 0,
								ResponseID:             nil,
								CompletedAt:            &completedAt,
							}

							// Set error if present
							if flowNodeStatus.Error != nil {
								errorStr := formatErrForUser(flowNodeStatus.Error)
								nodeExecution.Error = &errorStr
							}

							// Compress and store output data if available
							if flowNodeStatus.OutputData != nil {
								if outputJSON, err := json.Marshal(flowNodeStatus.OutputData); err == nil {
									if err := nodeExecution.SetOutputJSON(outputJSON); err != nil {
										nodeExecution.OutputData = outputJSON
										nodeExecution.OutputDataCompressType = 0
									}
								}
							}

							// Upsert to DB synchronously to ensure final state is persisted
							if err := persistUpsert2s(c.nes, nodeExecution); err != nil {
								log.Printf("Failed to upsert failed loop node execution %s: %v", nodeExecution.ID, err)
							}

							// Send immediately for terminal loop nodes to make them visible in UI
							if !channelsClosed.Load() {
								select {
								case nodeExecutionChan <- nodeExecution:
								case <-stopSending:
									// Channel closed, don't send
								}
								// Also emit a streamed node state to keep the client in sync even if
								// the runner's final status arrives after flow status.
								nodeMsg := &flowv1.FlowRunNodeResponse{
									NodeId: flowNodeStatus.NodeID.Bytes(),
									State:  nodev1.NodeState(flowNodeStatus.State),
								}
								if flowNodeStatus.Error != nil {
									if !(loopNodeIDs[flowNodeStatus.NodeID] && flowNodeStatus.State == mnnode.NODE_STATE_CANCELED) {
										em := formatErrForUser(flowNodeStatus.Error)
										nodeMsg.Info = &em
									}
								}
								resp := &flowv1.FlowRunResponse{Node: nodeMsg}
								if err := sendFlowResponseSync(resp); err != nil {
									signalDone(err)
									return
								}
							}
						}
					}
					pendingMutex.Unlock()
				}
			}

			// Send node status response
			// Skip per-iteration RUNNING/SUCCESS updates for loop nodes using the explicit flag.
			if flowNodeStatus.IterationEvent && (flowNodeStatus.State == mnnode.NODE_STATE_RUNNING || flowNodeStatus.State == mnnode.NODE_STATE_SUCCESS) {
				return
			}

			var info *string
			if flowNodeStatus.Error != nil {
				skipInfo := loopNodeIDs[flowNodeStatus.NodeID] && flowNodeStatus.State == mnnode.NODE_STATE_CANCELED
				if !skipInfo {
					msg := formatErrForUser(flowNodeStatus.Error)
					info = &msg
				}
			}
			if err := sendNodeStatusSync(flowNodeStatus.NodeID, nodev1.NodeState(flowNodeStatus.State), info); err != nil {
				signalDone(err)
				return
			}
		}

		nodeStatesClosed := false
		flowStatusClosed := false
		sendCompletionIfChannelsClosed := func() {
			if nodeStatesClosed && flowStatusClosed {
				signalDone(nil)
			}
		}
		for {
			select {
			case <-ctx.Done():
				// Client disconnected, signal all goroutines to stop
				channelsClosed.Store(true)
				close(stopSending)

				// Cancel the flow execution
				cancel()

				// DO NOT close channels here - let the runner close them
				// The runner has deferred closes for these channels
				signalDone(errors.New("client disconnected"))
				return
			case <-subCtx.Done():
				// Flow execution cancelled
				channelsClosed.Store(true)
				close(stopSending)

				// DO NOT close channels here - let the runner close them
				signalDone(errors.New("flow execution cancelled"))
				return
			case flowNodeStatus, ok := <-nodeStateChan:
				if !ok {
					// Channel closed by runner
					nodeStateChan = nil
					nodeStatesClosed = true
					sendCompletionIfChannelsClosed()
					continue
				}
				nodeStatusFunc(flowNodeStatus)
			case flowStatus, ok := <-flowStatusChan:
				if !ok {
					// Channel closed by runner without terminal status
					flowStatusChan = nil
					flowStatusClosed = true
					signalDone(errors.New("flow status channel closed unexpectedly"))
					return
				}
				// Process any pending node status messages without blocking
			drainLoop:
				for nodeStateChan != nil && len(nodeStateChan) > 0 {
					select {
					case flowNodeStatus := <-nodeStateChan:
						nodeStatusFunc(flowNodeStatus)
					default:
						// No more messages immediately available, exit loop
						break drainLoop
					}
				}
				if runner.IsFlowStatusDone(flowStatus) {
					// Drain all remaining node statuses until the channel is closed
					if nodeStateChan != nil {
						for flowNodeStatus := range nodeStateChan {
							nodeStatusFunc(flowNodeStatus)
						}
						nodeStateChan = nil
						nodeStatesClosed = true
					}
					flowStatusChan = nil
					flowStatusClosed = true
					signalDone(nil)
					return
				}
			}
		}
	}()

	// Dedicated goroutine to drain request node responses promptly.
	goroutineWg.Add(1)
	go func() {
		defer goroutineWg.Done()
		for {
			select {
			case requestNodeResp, ok := <-requestNodeRespChan:
				if !ok {
					return
				}
				if err := processRequestResponse(requestNodeResp); err != nil {
					signalDone(err)
					return
				}
			case <-stopSending:
				for {
					select {
					case requestNodeResp, ok := <-requestNodeRespChan:
						if !ok {
							return
						}
						if err := processRequestResponse(requestNodeResp); err != nil {
							signalDone(err)
							return
						}
					default:
						return
					}
				}
			}
		}
	}()

	runStartedAt := time.Now()
	flowRunErr := runnerInst.RunWithEvents(subCtx, runner.FlowEventChannels{
		NodeStates: nodeStateChan,
		NodeLogs:   nodeLogChan,
		FlowStatus: flowStatusChan,
	}, flowVarsMap)

	// wait for the flow to finish
	flowErr := <-done
	// Calculate total duration
	durationMs := time.Since(runStartedAt).Milliseconds()

	// Signal all goroutines to stop if not already done
	if !channelsClosed.Load() {
		channelsClosed.Store(true)
		close(stopSending)
	}

	// After flow completes, flush any pending REQUEST node executions
	// that didn't receive responses yet. If any are still RUNNING (no CompletedAt),
	// mark them as CANCELED and persist synchronously to avoid stale RUNNING state.
	pendingMutex.Lock()
	for execID, nodeExec := range pendingNodeExecutions {
		if nodeExec.CompletedAt != nil {
			// Already completed: forward to channel and remove
			select {
			case nodeExecutionChan <- *nodeExec:
				delete(pendingNodeExecutions, execID)
			default:
				delete(pendingNodeExecutions, execID)
			}
			continue
		}

		// Not completed: treat as canceled
		nodeExec.State = mnnode.NODE_STATE_CANCELED
		completedAt := time.Now().UnixMilli()
		nodeExec.CompletedAt = &completedAt

		// Persist synchronously
		dbCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if err := upsertWithRetry(dbCtx, c.nes, *nodeExec); err != nil {
			log.Printf("Failed to upsert pending canceled node execution %s: %v", nodeExec.ID, err)
		}
		cancel()

		// Try to send to channel and remove
		select {
		case nodeExecutionChan <- *nodeExec:
			delete(pendingNodeExecutions, execID)
		default:
			delete(pendingNodeExecutions, execID)
		}
	}
	pendingMutex.Unlock()

	// Wait for all goroutines to finish before closing channels
	goroutineWg.Wait()

	// Now safe to close channels since all senders have stopped
	close(nodeExecutionChan)
	close(requestNodeRespChan)

	// Wait for all node executions to be collected
	<-nodeExecutionsDone

	// Safety net removed: with synchronous RUNNING/terminal/response upserts,
	// lingering RUNNING should not occur. Pending REQUEST executions are flushed above.

	// Final cleanup of pre-registered executions
	preRegisteredMutex.Lock()
	for execID := range preRegisteredExecutions {
		delete(preRegisteredExecutions, execID)
	}
	preRegisteredMutex.Unlock()

	const maxDurationMs = int64(1<<31 - 1)
	if durationMs < 0 {
		durationMs = 0
	}
	if durationMs > maxDurationMs {
		durationMs = maxDurationMs
	}
	flow.Duration = int32(durationMs)
	if updateErr := c.fs.UpdateFlow(ctx, flow); updateErr != nil {
		log.Printf("failed to persist flow duration for %s: %v", flowID.String(), updateErr)
	}

	flow.VersionParentID = &flow.ID

	// Access nodeExecutions (safe after channel reader finished)
	nodeExecutionsCopy := make([]mnodeexecution.NodeExecution, len(nodeExecutions))
	copy(nodeExecutionsCopy, nodeExecutions)

	res, err := c.PrepareCopyFlow(ctx, flow.WorkspaceID, flow, nodeExecutionsCopy)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	tx, err := c.DB.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer devtoolsdb.TxnRollback(tx)

	// NOTE: Node executions are now saved in real-time during flow execution,
	// so we no longer need to batch save them at the end. The batch save has been removed.
	// The nodeExecutions array is still collected for the flow version copy in PrepareCopyFlow.

	err = c.CopyFlow(ctx, tx, res)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	resp := &flowv1.FlowRunResponse{
		Version: tflowversion.ModelToRPC(res.Flow),
	}

	err = sendFlowResponseSync(resp)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	if flowErr != nil {
		return connect.NewError(connect.CodeInternal, flowErr)
	}

	if flowRunErr != nil {
		return connect.NewError(connect.CodeInternal, flowRunErr)
	}

	return nil
}

func (c *FlowServiceRPC) HandleExampleChanges(ctx context.Context, requestNodeResp nrequest.NodeRequestSideResp) error {
	FullHeaders := append(requestNodeResp.Resp.CreateHeaders, requestNodeResp.Resp.UpdateHeaders...)

	var assertResults []massertres.AssertResult
	var assert []massert.Assert
	for _, assertCouple := range requestNodeResp.Resp.AssertCouples {
		assertResults = append(assertResults, assertCouple.AssertRes)
		assert = append(assert, assertCouple.Assert)
	}

	example := requestNodeResp.Example
	endpoint, err := c.ias.GetItemApi(ctx, example.ItemApiID)
	if err != nil {
		return err
	}

	endpoint.VersionParentID = &endpoint.ID
	endpointNewID := idwrap.NewNow()
	endpoint.ID = endpointNewID

	err = c.ias.CreateItemApi(ctx, endpoint)
	if err != nil {
		return err
	}

	example.VersionParentID = &example.ID

	// TODO: should use same transaction as flow
	tx2, err := c.DB.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer devtoolsdb.TxnRollback(tx2)

	txExampleResp, err := sexampleresp.NewTX(ctx, tx2)
	if err != nil {
		return err
	}

	err = txExampleResp.CreateExampleResp(ctx, requestNodeResp.Resp.ExampleResp)
	if err != nil {
		return err
	}

	txHeaderResp, err := sexamplerespheader.NewTX(ctx, tx2)
	if err != nil {
		return err
	}

	for _, header := range requestNodeResp.Resp.CreateHeaders {
		err = txHeaderResp.CreateExampleRespHeader(ctx, header)
		if err != nil {
			return err
		}
	}
	for _, header := range requestNodeResp.Resp.UpdateHeaders {
		err = txHeaderResp.UpdateExampleRespHeader(ctx, header)
		if err != nil {
			return err
		}
	}
	for _, headerID := range requestNodeResp.Resp.DeleteHeaderIds {
		err = txHeaderResp.DeleteExampleRespHeader(ctx, headerID)
		if err != nil {
			return err
		}
	}

	// Handle assert results - create/update them in the database
	if len(assertResults) > 0 {
		txAssertRes, err := sassertres.NewTX(ctx, tx2)
		if err != nil {
			return err
		}

		for _, assertResult := range assertResults {
			err = txAssertRes.CreateAssertResult(ctx, assertResult)
			if err != nil {
				return err
			}
		}
	}

	res, err := ritemapiexample.PrepareCopyExampleNoService(ctx, endpointNewID, example,
		requestNodeResp.Queries, requestNodeResp.Headers, assert,
		&requestNodeResp.RawBody, requestNodeResp.FormBody, requestNodeResp.UrlBody,
		&requestNodeResp.Resp.ExampleResp, FullHeaders, assertResults)
	if err != nil {
		return err
	}

	err = ritemapiexample.CreateCopyExample(ctx, tx2, res)
	if err != nil {
		return err
	}

	err = tx2.Commit()
	if err != nil {
		return err
	}

	return nil
}

type CopyFlowResult struct {
	Flow  mflow.Flow
	Nodes []mnnode.MNode
	Edges []edge.Edge

	// Specific node types
	RequestNodes []mnrequest.MNRequest
	ForNodes     []mnfor.MNFor
	ForEachNodes []mnforeach.MNForEach
	IfNodes      []mnif.MNIF
	NoopNodes    []mnnoop.NoopNode

	// Node executions for this flow run
	NodeExecutions []mnodeexecution.NodeExecution
}

func (c *FlowServiceRPC) PrepareCopyFlow(ctx context.Context, workspaceID idwrap.IDWrap, originalFlow mflow.Flow, nodeExecutions []mnodeexecution.NodeExecution) (CopyFlowResult, error) {
	result := CopyFlowResult{}

	newFlowID := idwrap.NewNow()
	oldFlowID := originalFlow.ID
	originalFlow.ID = newFlowID
	result.Flow = originalFlow

	nodes, err := c.ns.GetNodesByFlowID(ctx, oldFlowID)
	if err != nil {
		return result, fmt.Errorf("get nodes: %w", err)
	}

	edges, err := c.fes.GetEdgesByFlowID(ctx, oldFlowID)
	if err != nil {
		return result, fmt.Errorf("get edges: %w", err)
	}

	oldToNewIDMap := make(map[idwrap.IDWrap]idwrap.IDWrap)

	for _, node := range nodes {
		newNodeID := idwrap.NewNow()
		oldToNewIDMap[node.ID] = newNodeID

		newNode := node
		newNode.ID = newNodeID
		newNode.FlowID = newFlowID
		result.Nodes = append(result.Nodes, newNode)

		// Get and copy specific node data based on type
		switch node.NodeKind {
		case mnnode.NODE_KIND_REQUEST:
			rn, err := c.rns.GetNodeRequest(ctx, node.ID)
			if err != nil {
				return result, fmt.Errorf("get request node: %w", err)
			}
			newRN := *rn
			newRN.FlowNodeID = newNodeID
			result.RequestNodes = append(result.RequestNodes, newRN)

		case mnnode.NODE_KIND_FOR:
			fn, err := c.fns.GetNodeFor(ctx, node.ID)
			if err != nil {
				return result, fmt.Errorf("get for node: %w", err)
			}
			newFN := *fn
			newFN.FlowNodeID = newNodeID
			result.ForNodes = append(result.ForNodes, newFN)

		case mnnode.NODE_KIND_FOR_EACH:
			fen, err := c.fens.GetNodeForEach(ctx, node.ID)
			if err != nil {
				return result, fmt.Errorf("get foreach node: %w", err)
			}
			newFEN := *fen
			newFEN.FlowNodeID = newNodeID
			result.ForEachNodes = append(result.ForEachNodes, newFEN)

		case mnnode.NODE_KIND_CONDITION:
			ifn, err := c.ins.GetNodeIf(ctx, node.ID)
			if err != nil {
				return result, fmt.Errorf("get if node: %w", err)
			}
			newIFN := *ifn
			newIFN.FlowNodeID = newNodeID
			result.IfNodes = append(result.IfNodes, newIFN)

		case mnnode.NODE_KIND_NO_OP:
			nn, err := c.sns.GetNodeNoop(ctx, node.ID)
			if err != nil {
				return result, fmt.Errorf("get noop node: %w", err)
			}
			newNN := *nn
			newNN.FlowNodeID = newNodeID
			result.NoopNodes = append(result.NoopNodes, newNN)
		}
	}

	// Copy edges with new node IDs
	for _, edge := range edges {
		newEdge := edge
		newEdge.ID = idwrap.NewNow()
		newEdge.FlowID = newFlowID
		newEdge.SourceID = oldToNewIDMap[edge.SourceID]
		newEdge.TargetID = oldToNewIDMap[edge.TargetID]
		result.Edges = append(result.Edges, newEdge)
	}

	// Copy node executions with new node IDs
	for _, execution := range nodeExecutions {
		newExecution := execution
		newExecution.ID = idwrap.NewNow()
		// Map to the new node ID
		if newNodeID, ok := oldToNewIDMap[execution.NodeID]; ok {
			newExecution.NodeID = newNodeID
			result.NodeExecutions = append(result.NodeExecutions, newExecution)
		}
	}

	return result, nil
}

func (c *FlowServiceRPC) CopyFlow(ctx context.Context, tx *sql.Tx, copyData CopyFlowResult) error {
	// Create flow
	txFlow, err := sflow.NewTX(ctx, tx)
	if err != nil {
		return fmt.Errorf("create flow service: %w", err)
	}
	err = txFlow.CreateFlow(ctx, copyData.Flow)
	if err != nil {
		return fmt.Errorf("create flow: %w", err)
	}

	// Create nodes
	txNode, err := snode.NewTX(ctx, tx)
	if err != nil {
		return fmt.Errorf("create node service: %w", err)
	}
	for _, node := range copyData.Nodes {
		err = txNode.CreateNode(ctx, node)
		if err != nil {
			return fmt.Errorf("create node: %w", err)
		}
	}

	// Create specific node data
	txRequestNode, err := snoderequest.NewTX(ctx, tx)
	if err != nil {
		return fmt.Errorf("create request node service: %w", err)
	}
	for _, rn := range copyData.RequestNodes {
		err = txRequestNode.CreateNodeRequest(ctx, rn)
		if err != nil {
			return fmt.Errorf("create request node: %w", err)
		}
	}

	txForNode, err := snodefor.NewTX(ctx, tx)
	if err != nil {
		return fmt.Errorf("create for node service: %w", err)
	}
	for _, fn := range copyData.ForNodes {
		err = txForNode.CreateNodeFor(ctx, fn)
		if err != nil {
			return fmt.Errorf("create for node: %w", err)
		}
	}

	txForEachNode, err := snodeforeach.NewTX(ctx, tx)
	if err != nil {
		return fmt.Errorf("create foreach node service: %w", err)
	}
	for _, fen := range copyData.ForEachNodes {
		err = txForEachNode.CreateNodeForEach(ctx, fen)
		if err != nil {
			return fmt.Errorf("create foreach node: %w", err)
		}
	}

	txIfNode, err := snodeif.NewTX(ctx, tx)
	if err != nil {
		return fmt.Errorf("create if node service: %w", err)
	}
	for _, ifn := range copyData.IfNodes {
		err = txIfNode.CreateNodeIf(ctx, ifn)
		if err != nil {
			return fmt.Errorf("create if node: %w", err)
		}
	}

	txNoopNode, err := snodenoop.NewTX(ctx, tx)
	if err != nil {
		return fmt.Errorf("create noop node service: %w", err)
	}
	for _, nn := range copyData.NoopNodes {
		err = txNoopNode.CreateNodeNoop(ctx, nn)
		if err != nil {
			return fmt.Errorf("create noop node: %w", err)
		}
	}

	// Create edges
	txEdge, err := sedge.NewTX(ctx, tx)
	if err != nil {
		return fmt.Errorf("create edge service: %w", err)
	}
	for _, edge := range copyData.Edges {
		err = txEdge.CreateEdge(ctx, edge)
		if err != nil {
			return fmt.Errorf("create edge: %w", err)
		}
	}

	// Create node executions for this flow version
	txNodeExecution, err := snodeexecution.NewTX(ctx, tx)
	if err != nil {
		return fmt.Errorf("create node execution service: %w", err)
	}
	for _, execution := range copyData.NodeExecutions {
		err = txNodeExecution.CreateNodeExecution(ctx, execution)
		if err != nil {
			return fmt.Errorf("create node execution: %w", err)
		}
	}

	return nil
}

func (c *FlowServiceRPC) FlowVersions(ctx context.Context, req *connect.Request[flowv1.FlowVersionsRequest]) (*connect.Response[flowv1.FlowVersionsResponse], error) {
	flowID, err := idwrap.NewFromBytes(req.Msg.FlowId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerFlow(ctx, c.fs, c.us, flowID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	flows, err := c.fs.GetFlowsByVersionParentID(ctx, flowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	translatedFlows := tgeneric.MassConvert(flows, tflowversion.ModelToRPC)
	resp := &flowv1.FlowVersionsResponse{
		Items: translatedFlows,
	}

	sort.Slice(translatedFlows, func(i, j int) bool {
		return bytes.Compare(translatedFlows[i].FlowId, translatedFlows[j].FlowId) > 0
	})

	return connect.NewResponse(resp), nil
}

func CheckOwnerFlow(ctx context.Context, fs sflow.FlowService, us suser.UserService, flowID idwrap.IDWrap) (bool, error) {
	// TODO: add sql query to make it faster
	flow, err := fs.GetFlow(ctx, flowID)
	if err != nil {
		return false, err
	}
	return rworkspace.CheckOwnerWorkspace(ctx, us, flow.WorkspaceID)
}
