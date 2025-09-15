package rflow

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"testing"
	"time"

	"connectrpc.com/connect"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/db/pkg/sqlitemem"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logconsole"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/sassert"
	"the-dev-tools/server/pkg/service/sassertres"
	"the-dev-tools/server/pkg/service/sbodyform"
	"the-dev-tools/server/pkg/service/sbodyraw"
	"the-dev-tools/server/pkg/service/sbodyurl"
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
	"the-dev-tools/server/pkg/service/stag"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/service/sworkspacesusers"
	nodev1 "the-dev-tools/spec/dist/buf/go/flow/node/v1"
	flowv1 "the-dev-tools/spec/dist/buf/go/flow/v1"
)

// noopStream implements api.ServerStreamAdHoc by discarding messages.
type noopStream struct{}

func (noopStream) Send(*flowv1.FlowRunResponse) error { return nil }

func setupFlowRunAdHocBench(b *testing.B) (*FlowServiceRPC, context.Context, *connect.Request[flowv1.FlowRunRequest], func()) {
	b.Helper()

	ctx := context.Background()

	db, closeDB, err := sqlitemem.NewSQLiteMem(ctx)
	if err != nil {
		b.Fatalf("open sqlite memory DB: %v", err)
	}

	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		closeDB()
		b.Fatalf("prepare queries: %v", err)
	}

	ws := sworkspace.New(queries)
	us := suser.New(queries)
	ts := stag.New(queries)
	fs := sflow.New(queries)
	fts := sflowtag.New(queries)
	fes := sedge.New(queries)
	fvs := sflowvariable.New(queries)
	ias := sitemapi.New(queries)
	es := sitemapiexample.New(queries)
	qs := sexamplequery.New(queries)
	hs := sexampleheader.New(queries)
	brs := sbodyraw.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)
	ers := sexampleresp.New(queries)
	erhs := sexamplerespheader.New(queries)
	as := sassert.New(queries)
	ars := sassertres.New(queries)
	ns := snode.New(queries)
	rns := snoderequest.New(queries)
	fns := snodefor.New(queries)
	fens := snodeforeach.New(queries)
	sns := snodenoop.New(queries)
	ins := *snodeif.New(queries)
	jsns := snodejs.New(queries)
	nes := snodeexecution.New(queries)
	wus := sworkspacesusers.New(queries)

	logMap := logconsole.NewLogChanMap()

	flowSvc := &FlowServiceRPC{
		DB:         db,
		ws:         ws,
		us:         us,
		ts:         ts,
		fs:         fs,
		fts:        fts,
		fes:        fes,
		fvs:        fvs,
		ias:        ias,
		es:         es,
		qs:         qs,
		hs:         hs,
		brs:        brs,
		bfs:        bfs,
		bues:       bues,
		ers:        ers,
		erhs:       erhs,
		as:         as,
		ars:        ars,
		ns:         ns,
		rns:        rns,
		fns:        fns,
		fens:       fens,
		sns:        sns,
		ins:        ins,
		jsns:       jsns,
		nes:        nes,
		logChanMap: logMap,
	}

	workspaceID := idwrap.NewNow()
	workspace := mworkspace.Workspace{
		ID:              workspaceID,
		Name:            "bench-workspace",
		Updated:         time.Now(),
		ActiveEnv:       idwrap.NewNow(),
		GlobalEnv:       idwrap.NewNow(),
		FlowCount:       0,
		CollectionCount: 0,
	}
	if err := flowSvc.ws.Create(ctx, &workspace); err != nil {
		queries.Close()
		closeDB()
		b.Fatalf("create workspace: %v", err)
	}

	userID := idwrap.NewNow()
	providerID := "bench-provider"
	user := muser.User{
		ID:           userID,
		Email:        "bench@dev.tools",
		Password:     []byte("bench"),
		ProviderType: muser.MagicLink,
		ProviderID:   &providerID,
		Status:       muser.Active,
	}
	if err := flowSvc.us.CreateUser(ctx, &user); err != nil {
		queries.Close()
		closeDB()
		b.Fatalf("create user: %v", err)
	}

	workspaceUser := mworkspaceuser.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        mworkspaceuser.RoleOwner,
	}
	if err := wus.CreateWorkspaceUser(ctx, &workspaceUser); err != nil {
		queries.Close()
		closeDB()
		b.Fatalf("create workspace user: %v", err)
	}

	flowID := idwrap.NewNow()
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Bench Flow",
	}
	if err := flowSvc.fs.CreateFlow(ctx, flow); err != nil {
		queries.Close()
		closeDB()
		b.Fatalf("create flow: %v", err)
	}

	startNodeID := idwrap.NewNow()
	startNode := mnnode.MNode{
		ID:       startNodeID,
		FlowID:   flowID,
		Name:     "Start",
		NodeKind: mnnode.NODE_KIND_NO_OP,
	}
	if err := flowSvc.ns.CreateNode(ctx, startNode); err != nil {
		queries.Close()
		closeDB()
		b.Fatalf("create start node: %v", err)
	}

	startNoop := mnnoop.NoopNode{
		FlowNodeID: startNodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_START,
	}
	if err := flowSvc.sns.CreateNodeNoop(ctx, startNoop); err != nil {
		queries.Close()
		closeDB()
		b.Fatalf("create noop node: %v", err)
	}

	logCh := logMap.AddLogChannel(userID)
	stopLogs := make(chan struct{})
	go func() {
		for {
			select {
			case <-logCh:
			case <-stopLogs:
				return
			}
		}
	}()

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	req := connect.NewRequest(&flowv1.FlowRunRequest{FlowId: flowID.Bytes()})

	cleanup := func() {
		close(stopLogs)
		logMap.DeleteLogChannel(userID)
		if err := queries.Close(); err != nil {
			b.Fatalf("close queries: %v", err)
		}
		closeDB()
	}

	return flowSvc, authedCtx, req, cleanup
}

func BenchmarkFlowRunAdHoc(b *testing.B) {
	b.ReportAllocs()

	svc, authedCtx, req, cleanup := setupFlowRunAdHocBench(b)
	defer cleanup()

	originalWriter := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(originalWriter)

	stream := noopStream{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := svc.FlowRunAdHoc(authedCtx, req, stream); err != nil {
			b.Fatalf("FlowRunAdHoc failed: %v", err)
		}
	}
}

func BenchmarkBuildLogRefs_Error(b *testing.B) {
	b.ReportAllocs()
	name := "request-1"
	idStr := idwrap.NewNow().String()
	state := "FAILURE"
	err := assertErrExample{msg: "assertion failed: status == 200"}
	// small output map to carry safe context keys
	out := map[string]any{"failedAtIndex": 12, "totalItems": 100}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = buildLogRefs(name, idStr, state, err, out)
	}
}

func BenchmarkBuildLogRefs_OutputMap(b *testing.B) {
	b.ReportAllocs()
	name := "node-json"
	idStr := idwrap.NewNow().String()
	state := "SUCCESS"
	// nested output map
	out := map[string]any{
		name: map[string]any{
			"response": map[string]any{
				"status": 200,
				"json": map[string]any{
					"items": []any{
						map[string]any{"id": 1, "name": "foo"},
						map[string]any{"id": 2, "name": "bar"},
					},
				},
			},
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = buildLogRefs(name, idStr, state, nil, out)
	}
}

func BenchmarkNormalizeForLog_ByteJSON(b *testing.B) {
	b.ReportAllocs()
	payload := map[string]any{"a": 1, "b": []any{1, 2, 3}, "c": map[string]any{"k": "v"}}
	by, _ := json.Marshal(payload)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = normalizeForLog(by)
	}
}

func BenchmarkNormalizeForLog_DeepMap(b *testing.B) {
	b.ReportAllocs()
	out := map[string]any{
		"root": map[string]any{
			"array":  []any{1, 2, 3, map[string]any{"x": []byte("not-json")}},
			"nested": map[string]any{"k": "v", "n": map[string]any{"kk": "vv"}},
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = normalizeForLog(out)
	}
}

func BenchmarkSendNodeStatus(b *testing.B) {
	b.ReportAllocs()
	stream := noopStream{}
	nodeID := idwrap.NewNow()
	state := nodev1.NodeState_NODE_STATE_SUCCESS
	var info *string

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sendNodeStatus(stream, nodeID, state, info)
	}
}

// assertErrExample is a minimal error impl for benchmarking.
type assertErrExample struct{ msg string }

func (e assertErrExample) Error() string { return e.msg }
