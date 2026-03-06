package flowexec

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/dbtest"
	gen "github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
)

func TestSnapshotRegistry_NilSafe(t *testing.T) {
	t.Parallel()

	var r *SnapshotRegistry
	handler, ok := r.Get(mflow.NODE_KIND_REQUEST)
	assert.False(t, ok)
	assert.Nil(t, handler)
}

func TestSnapshotRegistry_GetUnregistered(t *testing.T) {
	t.Parallel()

	r := NewSnapshotRegistry()
	handler, ok := r.Get(mflow.NODE_KIND_REQUEST)
	assert.False(t, ok)
	assert.Nil(t, handler)
}

func TestSnapshotRegistry_RegisterAndGet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)
	forService := sflow.NewNodeForService(queries)

	r := NewSnapshotRegistry()
	r.Register(&ForSnapshot{Service: &forService})

	handler, ok := r.Get(mflow.NODE_KIND_FOR)
	assert.True(t, ok)
	assert.Equal(t, mflow.NODE_KIND_FOR, handler.Kind())

	// Unregistered kind still returns false
	_, ok = r.Get(mflow.NODE_KIND_REQUEST)
	assert.False(t, ok)
}

// TestWriteTx_TypedNilConfig verifies that WriteTx handles Go's typed nil
// interface gotcha: (*T)(nil) wrapped in any is NOT == nil.
func TestWriteTx_TypedNilConfig(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)
	newNodeID := idwrap.NewNow()

	nrsService := sflow.NewNodeRequestService(queries)
	ngqsService := sflow.NewNodeGraphQLService(queries)
	nwcsService := sflow.NewNodeWsConnectionService(queries)
	nwssService := sflow.NewNodeWsSendService(queries)
	nwaitsService := sflow.NewNodeWaitService(queries)

	tests := []struct {
		name    string
		handler NodeConfigSnapshot
		config  any // typed nil pointer
	}{
		{
			name:    "Request typed nil",
			handler: &RequestSnapshot{Service: &nrsService},
			config:  (*mflow.NodeRequest)(nil),
		},
		{
			name:    "GraphQL typed nil",
			handler: &GraphQLSnapshot{Service: &ngqsService},
			config:  (*mflow.NodeGraphQL)(nil),
		},
		{
			name:    "WsConnection typed nil",
			handler: &WsConnectionSnapshot{Service: &nwcsService},
			config:  (*mflow.NodeWsConnection)(nil),
		},
		{
			name:    "WsSend typed nil",
			handler: &WsSendSnapshot{Service: &nwssService},
			config:  (*mflow.NodeWsSend)(nil),
		},
		{
			name:    "Wait typed nil",
			handler: &WaitSnapshot{Service: &nwaitsService},
			config:  (*mflow.NodeWait)(nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should NOT panic — must handle typed nil gracefully
			result, err := tt.handler.WriteTx(ctx, nil, newNodeID, tt.config)
			assert.NoError(t, err)
			assert.Nil(t, result)
		})
	}
}

// TestWriteTx_DefaultsWithTypedNil verifies that "always create with defaults"
// snapshot types create valid records even with typed nil config.
func TestWriteTx_DefaultsWithTypedNil(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)

	// Create required parent records
	flowID := idwrap.NewNow()
	err = queries.CreateFlow(ctx, gen.CreateFlowParams{
		ID:          flowID,
		WorkspaceID: idwrap.NewNow(),
		Name:        "test",
	})
	require.NoError(t, err)

	nfsService := sflow.NewNodeForService(queries)
	naisService := sflow.NewNodeAIService(queries)
	nmemsService := sflow.NewNodeMemoryService(queries)

	tests := []struct {
		name          string
		handler       NodeConfigSnapshot
		config        any
		checkDefaults func(t *testing.T, result any)
	}{
		{
			name:    "For with typed nil uses defaults",
			handler: &ForSnapshot{Service: &nfsService},
			config:  (*mflow.NodeFor)(nil),
			checkDefaults: func(t *testing.T, result any) {
				data := result.(mflow.NodeFor)
				assert.Equal(t, int64(1), data.IterCount, "default IterCount")
				assert.Equal(t, mflow.ErrorHandling_ERROR_HANDLING_BREAK, data.ErrorHandling, "default ErrorHandling")
			},
		},
		{
			name:    "For with config copies values",
			handler: &ForSnapshot{Service: &nfsService},
			config: &mflow.NodeFor{
				IterCount:     10,
				ErrorHandling: mflow.ErrorHandling_ERROR_HANDLING_IGNORE,
			},
			checkDefaults: func(t *testing.T, result any) {
				data := result.(mflow.NodeFor)
				assert.Equal(t, int64(10), data.IterCount, "copied IterCount")
				assert.Equal(t, mflow.ErrorHandling_ERROR_HANDLING_IGNORE, data.ErrorHandling, "copied ErrorHandling")
			},
		},
		{
			name:    "AI with typed nil uses defaults",
			handler: &AISnapshot{Service: &naisService},
			config:  (*mflow.NodeAI)(nil),
			checkDefaults: func(t *testing.T, result any) {
				data := result.(mflow.NodeAI)
				assert.Equal(t, "", data.Prompt, "default Prompt")
				assert.Equal(t, int32(5), data.MaxIterations, "default MaxIterations")
			},
		},
		{
			name:    "Memory with typed nil uses defaults",
			handler: &MemorySnapshot{Service: &nmemsService},
			config:  (*mflow.NodeMemory)(nil),
			checkDefaults: func(t *testing.T, result any) {
				data := result.(mflow.NodeMemory)
				assert.Equal(t, mflow.AiMemoryTypeWindowBuffer, data.MemoryType)
				assert.Equal(t, int32(10), data.WindowSize)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeID := idwrap.NewNow()
			err := queries.CreateFlowNode(ctx, gen.CreateFlowNodeParams{
				ID:       nodeID,
				FlowID:   flowID,
				Name:     tt.name,
				NodeKind: int32(tt.handler.Kind()),
			})
			require.NoError(t, err)

			tx, err := db.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer tx.Rollback() //nolint:errcheck

			result, err := tt.handler.WriteTx(ctx, tx, nodeID, tt.config)
			require.NoError(t, err)
			require.NotNil(t, result)

			tt.checkDefaults(t, result)

			err = tx.Commit()
			require.NoError(t, err)
		})
	}
}

// TestWriteTx_UntypedNil verifies that pure nil (not typed nil) is also handled.
func TestWriteTx_UntypedNil(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)
	nrsService := sflow.NewNodeRequestService(queries)

	handler := &RequestSnapshot{Service: &nrsService}
	result, err := handler.WriteTx(ctx, nil, idwrap.NewNow(), nil)
	assert.NoError(t, err)
	assert.Nil(t, result)
}
