package rflowv2

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/dbtest"
	gen "github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rlog"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/dbtime"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream/memory"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/flowbuilder"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/http/resolver"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/senv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
)

func TestFlowRun_Logging(t *testing.T) {
	// Setup DB
	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)

	// Setup Services
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	nodeService := sflow.NewNodeService(queries)
	nodeExecService := sflow.NewNodeExecutionService(queries)
	edgeService := sflow.NewEdgeService(queries)
	flowVarService := sflow.NewFlowVariableService(queries)

	// Missing services for builder
	reqService := sflow.NewNodeRequestService(queries)
	forService := sflow.NewNodeForService(queries)
	forEachService := sflow.NewNodeForEachService(queries)
	ifService := sflow.NewNodeIfService(queries)
	jsService := sflow.NewNodeJsService(queries)
	varService := senv.NewVariableService(queries, logger)

	// Readers
	wsReader := sworkspace.NewWorkspaceReaderFromQueries(queries)
	fsReader := sflow.NewFlowReaderFromQueries(queries)
	nsReader := sflow.NewNodeReaderFromQueries(queries)

	// Mock resolver (or use standard with nil services if not used in test)
	// Since we only use NoOp node in this test, resolver won't be called for requests
	// But builder needs it.
	// We can pass nil for resolver dependencies as they are not used for NoOp
	res := resolver.NewStandardResolver(nil, nil, nil, nil, nil, nil, nil)

	// Setup Log Streamer
	logStreamer := memory.NewInMemorySyncStreamer[rlog.LogTopic, rlog.LogEvent]()
	defer logStreamer.Shutdown()

	builder := flowbuilder.New(
		&nodeService,
		&reqService,
		&forService,
		&forEachService,
		ifService,
		&jsService,
		nil, // NodeAIService
		nil, // NodeModelService
		nil, // NodeMemoryService
		&wsService,
		&varService,
		&flowVarService,
		res,
		logger,
		nil, // LLMProviderFactory
	)

	svc := &FlowServiceV2RPC{
		DB:           db,
		wsReader:     wsReader,
		fsReader:     fsReader,
		nsReader:     nsReader,
		ws:           &wsService,
		fs:           &flowService,
		ns:           &nodeService,
		nes:          &nodeExecService,
		es:           &edgeService,
		nrs:          &reqService,
		nfs:          &forService,
		nfes:         &forEachService,
		nifs:         ifService,
		njss:         &jsService,
		fvs:          &flowVarService,
		logger:       logger,
		logStream:    logStreamer,
		builder:      builder,
		runningFlows: make(map[string]context.CancelFunc),
	}

	// Setup Data
	userID := idwrap.NewNow()
	ctx = mwauth.CreateAuthedContext(ctx, userID)

	err = queries.CreateUser(ctx, gen.CreateUserParams{
		ID:    userID,
		Email: "test@example.com",
	})
	require.NoError(t, err)

	workspaceID := idwrap.NewNow()
	workspace := mworkspace.Workspace{
		ID:              workspaceID,
		Name:            "Test Workspace",
		Updated:         dbtime.DBNow(),
		CollectionCount: 0,
		FlowCount:       0,
	}
	err = wsService.Create(ctx, &workspace)
	require.NoError(t, err)

	err = queries.CreateWorkspaceUser(ctx, gen.CreateWorkspaceUserParams{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        1,
	})
	require.NoError(t, err)

	flowID := idwrap.NewNow()
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Logging Test Flow",
	}
	err = flowService.CreateFlow(ctx, flow)
	require.NoError(t, err)

	// Create Start Node (ManualStart)
	startNodeID := idwrap.NewNow()
	startNode := mflow.Node{
		ID:        startNodeID,
		FlowID:    flowID,
		Name:      "Start",
		NodeKind:  mflow.NODE_KIND_MANUAL_START,
		PositionX: 0,
		PositionY: 0,
	}
	err = nodeService.CreateNode(ctx, startNode)
	require.NoError(t, err)

	// Subscribe to logs
	logCh, err := logStreamer.Subscribe(ctx, func(topic rlog.LogTopic) bool {
		return true // Accept all logs since topic is currently empty in main code
	})
	require.NoError(t, err)

	// Run Flow
	req := connect.NewRequest(&flowv1.FlowRunRequest{FlowId: flowID.Bytes()})
	_, err = svc.FlowRun(ctx, req)
	require.NoError(t, err)

	// Wait for logs
	timeout := time.After(2 * time.Second)
	var logs []rlog.LogEvent

	// We expect logs for node execution: PENDING, RUNNING, SUCCESS (at least)
Loop:
	for {
		select {
		case evt := <-logCh:
			logs = append(logs, evt.Payload)
			// Break if we have received logs and the last one is SUCCESS/FAILURE
			if len(logs) > 0 {
				lastLog := logs[len(logs)-1]
				if lastLog.Log != nil && (lastLog.Log.Name == "Node Start: Success" || lastLog.Log.Name == "Node Start: Failure") {
					break Loop
				}
			}
		case <-timeout:
			require.FailNow(t, "Timeout waiting for logs")
		}
	}

	require.NotEmpty(t, logs)
	// Check if we got structured logs
	foundStart := false
	for _, l := range logs {
		assert.Equal(t, rlog.EventTypeInsert, l.Type)
		assert.NotNil(t, l.Log)
		assert.NotEmpty(t, l.Log.LogId)
		assert.NotEmpty(t, l.Log.Name)

		// Check structured value
		val := l.Log.Value.GetStructValue()
		require.NotNil(t, val)
		fields := val.Fields
		assert.Contains(t, fields, "node_id")
		assert.Contains(t, fields, "node_name")
		assert.Contains(t, fields, "state")
		assert.Contains(t, fields, "flow_id")
		assert.Contains(t, fields, "duration_ms")

		nodeIDStr := fields["node_id"].GetStringValue()
		assert.Equal(t, startNodeID.String(), nodeIDStr)

		if l.Log.Name == "Node Start: Success" {
			foundStart = true
		}
	}
	assert.True(t, foundStart, "Should receive Success log for start node")
}
