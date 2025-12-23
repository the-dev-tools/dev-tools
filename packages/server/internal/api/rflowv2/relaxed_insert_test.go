package rflowv2

import (
	"database/sql"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

func TestRelaxedInsert_All(t *testing.T) {
	svc, queries, ctx, _, workspaceID := setupTestService(t)

	// Create a flow for Edge tests (since they have DB FKs to Flow)
	flowID := idwrap.NewNow()
	err := svc.fs.CreateFlow(ctx, mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	})
	require.NoError(t, err)

	// Helper to create HTTP for NodeHttp
	createHttp := func() idwrap.IDWrap {
		id := idwrap.NewNow()
		err := queries.CreateHTTP(ctx, gen.CreateHTTPParams{
			ID:          id,
			WorkspaceID: workspaceID,
			Name:        "Test Http",
			Method:      "GET",
			Url:         "http://example.com",
			BodyKind:    0,
			ContentHash: sql.NullString{Valid: false},
			CreatedAt:   0,
			UpdatedAt:   0,
		})
		require.NoError(t, err)
		return id
	}

	t.Run("NodeConditionInsert without base node", func(t *testing.T) {
		nodeID := idwrap.NewNow()
		req := connect.NewRequest(&flowv1.NodeConditionInsertRequest{
			Items: []*flowv1.NodeConditionInsert{{
				NodeId:    nodeID.Bytes(),
				Condition: "true",
			}},
		})
		_, err := svc.NodeConditionInsert(ctx, req)
		require.NoError(t, err)
	})

	t.Run("NodeForEachInsert without base node", func(t *testing.T) {
		nodeID := idwrap.NewNow()
		req := connect.NewRequest(&flowv1.NodeForEachInsertRequest{
			Items: []*flowv1.NodeForEachInsert{{
				NodeId:        nodeID.Bytes(),
				Path:          "items",
				Condition:     "true",
				ErrorHandling: flowv1.ErrorHandling_ERROR_HANDLING_IGNORE,
			}},
		})
		_, err := svc.NodeForEachInsert(ctx, req)
		require.NoError(t, err)
	})

	t.Run("NodeJsInsert without base node", func(t *testing.T) {
		nodeID := idwrap.NewNow()
		req := connect.NewRequest(&flowv1.NodeJsInsertRequest{
			Items: []*flowv1.NodeJsInsert{{
				NodeId: nodeID.Bytes(),
				Code:   "console.log(1)",
			}},
		})
		_, err := svc.NodeJsInsert(ctx, req)
		require.NoError(t, err)
	})

	t.Run("EdgeInsert without source/target nodes", func(t *testing.T) {
		sourceID := idwrap.NewNow()
		targetID := idwrap.NewNow()
		req := connect.NewRequest(&flowv1.EdgeInsertRequest{
			Items: []*flowv1.EdgeInsert{{
				FlowId:       flowID.Bytes(),
				SourceId:     sourceID.Bytes(),
				TargetId:     targetID.Bytes(),
				SourceHandle: flowv1.HandleKind_HANDLE_KIND_THEN,
			}},
		})
		_, err := svc.EdgeInsert(ctx, req)
		require.NoError(t, err)
	})

	t.Run("NodeHttpInsert without base node", func(t *testing.T) {
		nodeID := idwrap.NewNow()
		httpID := createHttp()
		req := connect.NewRequest(&flowv1.NodeHttpInsertRequest{
			Items: []*flowv1.NodeHttpInsert{{
				NodeId: nodeID.Bytes(),
				HttpId: httpID.Bytes(),
			}},
		})
		_, err := svc.NodeHttpInsert(ctx, req)
		require.NoError(t, err)
	})

	t.Run("NodeForInsert without base node", func(t *testing.T) {
		nodeID := idwrap.NewNow()
		req := connect.NewRequest(&flowv1.NodeForInsertRequest{
			Items: []*flowv1.NodeForInsert{{
				NodeId:     nodeID.Bytes(),
				Iterations: 5,
				Condition:  "true",
			}},
		})
		_, err := svc.NodeForInsert(ctx, req)
		require.NoError(t, err)
	})
}
