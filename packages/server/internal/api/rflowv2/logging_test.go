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

	"the-dev-tools/db/pkg/dbtest"
	gen "the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rlog"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/eventstream/memory"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sflowvariable"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodeexecution"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/sworkspace"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

func TestFlowRun_Logging(t *testing.T) {
	// Setup DB
	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)

	// Setup Services
	wsService := sworkspace.New(queries)
	flowService := sflow.New(queries)
	nodeService := snode.New(queries)
	nodeExecService := snodeexecution.New(queries)
	edgeService := sedge.New(queries)
	noopService := snodenoop.New(queries)
	flowVarService := sflowvariable.New(queries)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Setup Log Streamer
	logStreamer := memory.NewInMemorySyncStreamer[rlog.LogTopic, rlog.LogEvent]()
	defer logStreamer.Shutdown()

	svc := &FlowServiceV2RPC{
		ws:           &wsService,
		fs:           &flowService,
		ns:           &nodeService,
		nes:          &nodeExecService,
		es:           &edgeService,
		nnos:         &noopService,
		fvs:          &flowVarService,
		logger:       logger,
		logStream:    logStreamer,
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

	// Create Start Node (NoOp)
	startNodeID := idwrap.NewNow()
	startNode := mnnode.MNode{
		ID:        startNodeID,
		FlowID:    flowID,
		Name:      "Start",
		NodeKind:  mnnode.NODE_KIND_NO_OP,
		PositionX: 0,
		PositionY: 0,
	}
	err = nodeService.CreateNode(ctx, startNode)
	require.NoError(t, err)

	err = noopService.CreateNodeNoop(ctx, mnnoop.NoopNode{
		FlowNodeID: startNodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_START,
	})
	require.NoError(t, err)

	// Subscribe to logs
	logCh, err := logStreamer.Subscribe(ctx, func(topic rlog.LogTopic) bool {
		return topic.UserID == userID
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
				if lastLog.Log != nil && (lastLog.Log.Name == "Node "+startNodeID.String()+": Success" || lastLog.Log.Name == "Node "+startNodeID.String()+": Failure") {
					break Loop
				}
			}
		case <-timeout:
			t.Fatal("Timeout waiting for logs")
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
		assert.Contains(t, fields, "state")
		assert.Contains(t, fields, "flow_id")

		nodeIDStr := fields["node_id"].GetStringValue()
		assert.Equal(t, startNodeID.String(), nodeIDStr)

		if l.Log.Name == "Node "+startNodeID.String()+": Success" {
			foundStart = true
		}
	}
	assert.True(t, foundStart, "Should receive Success log for start node")
}
