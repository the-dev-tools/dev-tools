package rflowv2

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
)

func TestNodeHttpCRUD(t *testing.T) {
	svc, _, ctx, _, workspaceID := setupTestService(t)

	// 1. Setup: Create Flow
	flowID := idwrap.NewNow()
	err := svc.fs.CreateFlow(ctx, mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow HTTP",
	})
	require.NoError(t, err)

	// 2. Setup: Create Node (REQUEST kind)
	nodeID := idwrap.NewNow()
	err = svc.ns.CreateNode(ctx, mflow.Node{
		ID:        nodeID,
		FlowID:    flowID,
		Name:      "HTTP Node",
		NodeKind:  mflow.NODE_KIND_REQUEST,
		PositionX: 0,
		PositionY: 0,
	})
	require.NoError(t, err)

	httpID := idwrap.NewNow()
	deltaHttpID := idwrap.NewNow()

	// Test NodeHttpInsert
	t.Run("Insert", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.NodeHttpInsertRequest{
			Items: []*flowv1.NodeHttpInsert{
				{
					NodeId:      nodeID.Bytes(),
					HttpId:      httpID.Bytes(),
					DeltaHttpId: deltaHttpID.Bytes(),
				},
			},
		})

		resp, err := svc.NodeHttpInsert(ctx, req)
		require.NoError(t, err)
		assert.IsType(t, &emptypb.Empty{}, resp.Msg)

		// Verify
		nodeReq, err := svc.nrs.GetNodeRequest(ctx, nodeID)
		require.NoError(t, err)
		require.NotNil(t, nodeReq)
		assert.Equal(t, httpID, *nodeReq.HttpID)
		assert.Equal(t, deltaHttpID, *nodeReq.DeltaHttpID)
	})

	// Test NodeHttpCollection (Verify Insert)
	t.Run("Collection", func(t *testing.T) {
		req := connect.NewRequest(&emptypb.Empty{})
		resp, err := svc.NodeHttpCollection(ctx, req)
		require.NoError(t, err)

		found := false
		for _, item := range resp.Msg.Items {
			itemNodeID, _ := idwrap.NewFromBytes(item.NodeId)
			if itemNodeID == nodeID {
				found = true
				itemHttpID, _ := idwrap.NewFromBytes(item.HttpId)
				assert.Equal(t, httpID, itemHttpID)

				itemDeltaID, _ := idwrap.NewFromBytes(item.DeltaHttpId)
				assert.Equal(t, deltaHttpID, itemDeltaID)
				break
			}
		}
		assert.True(t, found, "NodeHttp should be found in collection")
	})

	// Test NodeHttpUpdate
	t.Run("Update", func(t *testing.T) {
		newHttpID := idwrap.NewNow()
		req := connect.NewRequest(&flowv1.NodeHttpUpdateRequest{
			Items: []*flowv1.NodeHttpUpdate{
				{
					NodeId: nodeID.Bytes(),
					HttpId: newHttpID.Bytes(),
					// Not updating DeltaHttpId, should result in nil/empty in DB because of overwrite logic
				},
			},
		})

		resp, err := svc.NodeHttpUpdate(ctx, req)
		require.NoError(t, err)
		assert.IsType(t, &emptypb.Empty{}, resp.Msg)

		// Verify
		nodeReq, err := svc.nrs.GetNodeRequest(ctx, nodeID)
		require.NoError(t, err)
		require.NotNil(t, nodeReq)
		assert.Equal(t, newHttpID, *nodeReq.HttpID)
		assert.Nil(t, nodeReq.DeltaHttpID, "DeltaHttpID should be nil after update without providing it")
	})

	// Test NodeHttpDelete
	t.Run("Delete", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.NodeHttpDeleteRequest{
			Items: []*flowv1.NodeHttpDelete{
				{
					NodeId: nodeID.Bytes(),
				},
			},
		})

		resp, err := svc.NodeHttpDelete(ctx, req)
		require.NoError(t, err)
		assert.IsType(t, &emptypb.Empty{}, resp.Msg)

		// Verify
		nodeReq, err := svc.nrs.GetNodeRequest(ctx, nodeID)
		// Assuming GetNodeRequest returns nil when not found based on analysis of sflow.go
		if err != nil {
			// If it returned an error (like sql.ErrNoRows wrapped), check that.
			// But previous analysis said it returns nil, nil on ErrNoRows.
			// However, if GetNodeRequest returns error, assert it is ErrNoRows.
			if !errors.Is(err, sql.ErrNoRows) {
				t.Fatalf("Expected nil or ErrNoRows, got: %v", err)
			}
		} else {
			assert.Nil(t, nodeReq, "NodeRequest should be nil after delete")
		}
	})
}

func TestNodeHttpErrors(t *testing.T) {
	svc, _, ctx, _, workspaceID := setupTestService(t)

	// Setup Flow and Node
	flowID := idwrap.NewNow()
	err := svc.fs.CreateFlow(ctx, mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Error Test Flow",
	})
	require.NoError(t, err)

	nodeID := idwrap.NewNow()
	err = svc.ns.CreateNode(ctx, mflow.Node{
		ID:        nodeID,
		FlowID:    flowID,
		Name:      "HTTP Node",
		NodeKind:  mflow.NODE_KIND_REQUEST,
		PositionX: 0,
		PositionY: 0,
	})
	require.NoError(t, err)

	t.Run("Insert Invalid Node ID", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.NodeHttpInsertRequest{
			Items: []*flowv1.NodeHttpInsert{
				{
					NodeId: []byte("invalid-uuid"),
				},
			},
		})
		_, err := svc.NodeHttpInsert(ctx, req)
		assert.Error(t, err)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	})

	t.Run("Insert Invalid Http ID", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.NodeHttpInsertRequest{
			Items: []*flowv1.NodeHttpInsert{
				{
					NodeId: nodeID.Bytes(),
					HttpId: []byte("invalid-uuid"),
				},
			},
		})
		_, err := svc.NodeHttpInsert(ctx, req)
		assert.Error(t, err)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	})

	t.Run("Update Invalid Node ID", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.NodeHttpUpdateRequest{
			Items: []*flowv1.NodeHttpUpdate{
				{
					NodeId: []byte("invalid-uuid"),
				},
			},
		})
		_, err := svc.NodeHttpUpdate(ctx, req)
		assert.Error(t, err)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	})

	t.Run("Update Invalid Http ID", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.NodeHttpUpdateRequest{
			Items: []*flowv1.NodeHttpUpdate{
				{
					NodeId: nodeID.Bytes(),
					HttpId: []byte("invalid-uuid"),
				},
			},
		})
		_, err := svc.NodeHttpUpdate(ctx, req)
		assert.Error(t, err)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	})

	t.Run("Delete Invalid Node ID", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.NodeHttpDeleteRequest{
			Items: []*flowv1.NodeHttpDelete{
				{
					NodeId: []byte("invalid-uuid"),
				},
			},
		})
		_, err := svc.NodeHttpDelete(ctx, req)
		assert.Error(t, err)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	})

	t.Run("Update Access Denied", func(t *testing.T) {
		// Use a different user context
		otherUserCtx := mwauth.CreateAuthedContext(context.Background(), idwrap.NewNow())

		req := connect.NewRequest(&flowv1.NodeHttpUpdateRequest{
			Items: []*flowv1.NodeHttpUpdate{
				{
					NodeId: nodeID.Bytes(), // Exists but belongs to another user
				},
			},
		})
		_, err := svc.NodeHttpUpdate(otherUserCtx, req)
		assert.Error(t, err)
		// Expect NotFound (to prevent enumeration) or PermissionDenied?
		// Analysis says: "ID Enumeration Prevention... return CodeNotFound"
		assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	})

	t.Run("Delete Access Denied", func(t *testing.T) {
		otherUserCtx := mwauth.CreateAuthedContext(context.Background(), idwrap.NewNow())

		req := connect.NewRequest(&flowv1.NodeHttpDeleteRequest{
			Items: []*flowv1.NodeHttpDelete{
				{
					NodeId: nodeID.Bytes(),
				},
			},
		})
		_, err := svc.NodeHttpDelete(otherUserCtx, req)
		assert.Error(t, err)
		assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	})
}
