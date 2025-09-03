package rflow_test

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "the-dev-tools/db/pkg/sqlc"
    "the-dev-tools/server/internal/api/middleware/mwauth"
    "the-dev-tools/server/internal/api/rflow"
    "the-dev-tools/server/pkg/flow/edge"
    "the-dev-tools/server/pkg/idwrap"
    "the-dev-tools/server/pkg/logconsole"
    "the-dev-tools/server/pkg/model/mflow"
    "the-dev-tools/server/pkg/model/mbodyraw"
    "the-dev-tools/server/pkg/model/mitemapi"
    "the-dev-tools/server/pkg/model/mitemapiexample"
    "the-dev-tools/server/pkg/model/mnnode"
    "the-dev-tools/server/pkg/model/mnnode/mnnoop"
    "the-dev-tools/server/pkg/model/mnnode/mnrequest"
    "the-dev-tools/server/pkg/reference"
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
    "the-dev-tools/server/pkg/testutil"
    flowv1 "the-dev-tools/spec/dist/buf/go/flow/v1"

    "connectrpc.com/connect"
    "github.com/stretchr/testify/require"
)

// convert a ReferenceTreeItem into a nested map[string]any for easy assertions
func refToMap(ref reference.ReferenceTreeItem) map[string]any {
    out := make(map[string]any)
    switch ref.Kind {
    case reference.ReferenceKind_REFERENCE_KIND_MAP:
        m := make(map[string]any)
        for _, child := range ref.Map {
            childMap := refToMap(child)
            // key child.Key.Key is the map key
            m[child.Key.Key] = childMap[child.Key.Key]
        }
        out[ref.Key.Key] = m
    case reference.ReferenceKind_REFERENCE_KIND_ARRAY:
        // not expected here
        out[ref.Key.Key] = []any{}
    case reference.ReferenceKind_REFERENCE_KIND_VALUE:
        out[ref.Key.Key] = ref.Value
    default:
        out[ref.Key.Key] = nil
    }
    return out
}

func TestRequestLogging_ContainsRequestAndResponseFields(t *testing.T) {
    t.Parallel()
    ctx := context.Background()

    // Spin up a quick HTTP server
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("X-Unit-Test", "ok")
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte(`{"ok":true}`))
    }))
    defer srv.Close()

    base := testutil.CreateBaseDB(ctx, t)
    defer sqlc.CloseQueriesAndLog(base.Queries)
    q := base.Queries
    db := base.DB

    // Services
    ws := sworkspace.New(q)
    us := suser.New(q)
    ts := stag.New(q)
    fs := sflow.New(q)
    fts := sflowtag.New(q)
    fes := sedge.New(q)
    fvs := sflowvariable.New(q)
    ias := sitemapi.New(q)
    es := sitemapiexample.New(q)
    qs := sexamplequery.New(q)
    hs := sexampleheader.New(q)
    brs := sbodyraw.New(q)
    bfs := sbodyform.New(q)
    bues := sbodyurl.New(q)
    ers := sexampleresp.New(q)
    erhs := sexamplerespheader.New(q)
    ns := snode.New(q)
    rns := snoderequest.New(q)
    fns := snodefor.New(q)
    fens := snodeforeach.New(q)
    sns := snodenoop.New(q)
    ins := snodeif.New(q)
    jsns := snodejs.New(q)
    nes := snodeexecution.New(q)

    logMap := logconsole.NewLogChanMapWith(10000)

    svc := rflow.New(db, ws, us, ts,
        fs, fts, fes, fvs,
        ias, es, qs, hs,
        brs, bfs, bues,
        ers, erhs, sassert.New(q), sassertres.New(q),
        ns, rns, fns, fens,
        sns, *ins, jsns, nes,
        logMap,
    )

    // Workspace/user
    wsID := idwrap.NewNow()
    wsUserID := idwrap.NewNow()
    userID := idwrap.NewNow()
    collectionID := idwrap.NewNow()
    base.GetBaseServices().CreateTempCollection(t, ctx, wsID, wsUserID, userID, collectionID)

    // Flow
    flowID := idwrap.NewNow()
    require.NoError(t, fs.CreateFlow(ctx, mflow.Flow{ID: flowID, WorkspaceID: wsID, Name: "req_log"}))

    // Start node
    startID := idwrap.NewNow()
    require.NoError(t, ns.CreateNode(ctx, mnnode.MNode{ID: startID, FlowID: flowID, Name: "Start", NodeKind: mnnode.NODE_KIND_NO_OP}))
    require.NoError(t, sns.CreateNodeNoop(ctx, mnnoop.NoopNode{FlowNodeID: startID, Type: mnnoop.NODE_NO_OP_KIND_START}))

    // Endpoint + example
    endpointID := idwrap.NewNow()
    require.NoError(t, ias.CreateItemApi(ctx, &mitemapi.ItemApi{ID: endpointID, CollectionID: collectionID, Name: "E", Url: srv.URL, Method: "GET"}))
    exampleID := idwrap.NewNow()
    require.NoError(t, es.CreateApiExample(ctx, &mitemapiexample.ItemApiExample{ID: exampleID, ItemApiID: endpointID, CollectionID: collectionID, Name: "Ex"}))

    // Create empty raw body for example to satisfy downstream copy logic
    require.NoError(t, brs.CreateBodyRaw(ctx, mbodyraw.ExampleBodyRaw{ID: idwrap.NewNow(), ExampleID: exampleID}))

    // Request node
    reqNodeID := idwrap.NewNow()
    require.NoError(t, ns.CreateNode(ctx, mnnode.MNode{ID: reqNodeID, FlowID: flowID, Name: "Request", NodeKind: mnnode.NODE_KIND_REQUEST}))
    require.NoError(t, rns.CreateNodeRequest(ctx, mnrequest.MNRequest{FlowNodeID: reqNodeID, EndpointID: &endpointID, ExampleID: &exampleID}))

    // Edge start -> request
    require.NoError(t, fes.CreateEdge(ctx, edge.Edge{ID: idwrap.NewNow(), FlowID: flowID, SourceID: startID, TargetID: reqNodeID, SourceHandler: edge.HandleUnspecified}))

    // Subscribe to logs
    logCh := logMap.AddLogChannel(userID)
    defer logMap.DeleteLogChannel(userID)

    // Run flow
    req := connect.NewRequest(&flowv1.FlowRunRequest{FlowId: flowID.Bytes(), EnvironmentId: idwrap.NewNow().Bytes()})
    authed := mwauth.CreateAuthedContext(ctx, userID)
    err := svc.FlowRunAdHoc(authed, req, ServerStreamingHandlerMock[flowv1.FlowRunResponse]{SendStream: func(*flowv1.FlowRunResponse) {}})
    require.NoError(t, err)

    // Drain logs for a short window and look for request/response fields
    deadline := time.After(1 * time.Second)
    found := false
    for !found {
        select {
        case msg := <-logCh:
            // expect a single ref per message
            if len(msg.Refs) == 0 {
                continue
            }
            m := refToMap(msg.Refs[0])
            // root key is node name
            root := m["Request"]
            if rootMap, ok := root.(map[string]any); ok {
                if reqBlk, ok := rootMap["request"].(map[string]any); ok {
                    _, hasMethod := reqBlk["method"]
                    _, hasURL := reqBlk["url"]
                    if hasMethod && hasURL {
                        if resBlk, ok := rootMap["response"].(map[string]any); ok {
                            _, hasStatus := resBlk["status"]
                            _, hasDuration := resBlk["duration"]
                            // body should be a nested map (parsed JSON), not a raw numeric array string
                            bodyVal, hasBody := resBlk["body"]
                            bodyIsMap := false
                            if _, ok := bodyVal.(map[string]any); ok {
                                bodyIsMap = true
                            }
                            if hasStatus && hasDuration && hasBody && bodyIsMap {
                                found = true
                                break
                            }
                        }
                    }
                }
            }
        case <-deadline:
            t.Fatal("did not see expected request/response fields in log refs within timeout")
        }
    }
}

// ServerStreamingHandlerMock is declared in rflow_test.go in this package; reuse it here.
