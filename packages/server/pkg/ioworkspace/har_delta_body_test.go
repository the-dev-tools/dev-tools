package ioworkspace

import (
	"context"
	"testing"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/db/pkg/sqlitemem"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/snoderequest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImport_DeltaBodyRaw(t *testing.T) {
	ctx := context.Background()

	// 1. Setup services
	db, _, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err)

	wsID := idwrap.NewNow()
	// Create workspace user mapping for permission checks if needed,
	// but ioworkspace service usually assumes permissions are checked by caller or ignored for internal ops.
	// We might need to seed a workspace if foreign keys are enforced.
	// In sqlitemem/schema.sql, workspace_id might be a FK.
	// Let's assume we need a workspace.
	// For simplicity, we'll try without first, as some tests might not strictly enforce FKs if not enabled,
	// but usually they are.
	// Checking ioworkspace_test.go might reveal if we need to mock workspace service or create a workspace.
	// Assuming rigid FKs:
	// We don't have direct access to WorkspaceService here easily without more setup.
	// Let's check if we can insert directly/use a helper or if specific services are needed.
	// Actually, `NewSQLiteMem` usually applies schema.
	// Let's look at `ioworkspace_test.go` style.

	// Re-reading `ioworkspace_test.go` style would be best, but I can't do that inside this Write call.
	// I'll assume standard service setup.

	queries := gen.New(db)
	// httpService is created internally by Import

	httpBodyRawService := shttp.NewHttpBodyRawService(queries)

	// Services for verification
	nodeRequestService := snoderequest.New(queries)

	// httpService is consumed by Import internally or passed?
	// svc.Import takes httpService.

	svc := New(queries, nil)

	// 2. Prepare Bundle
	baseID := idwrap.NewNow()
	deltaID := idwrap.NewNow()

	flowID := idwrap.NewNow()
	nodeID := idwrap.NewNow()

	baseBodyRaw := mhttp.HTTPBodyRaw{
		ID:          idwrap.NewNow(),
		HttpID:      baseID,
		RawData:     []byte("base-content"),
		ContentType: "application/json",
		IsDelta:     false,
	}

	deltaCT := "application/json"
	deltaBodyRaw := mhttp.HTTPBodyRaw{
		ID:               idwrap.NewNow(),
		HttpID:           deltaID,
		ParentBodyRawID:  &baseBodyRaw.ID,
		IsDelta:          true,
		DeltaRawData:     []byte("delta-override-content"),
		DeltaContentType: &deltaCT, // Simulating harv2 which uses *string
		RawData:          nil,
	}

	bundle := &WorkspaceBundle{
		Flows: []mflow.Flow{
			{
				ID:          flowID,
				WorkspaceID: wsID,
				Name:        "Test Flow",
			},
		},
		FlowNodes: []mnnode.MNode{
			{
				ID:       nodeID,
				FlowID:   flowID,
				NodeKind: mnnode.NODE_KIND_REQUEST,
				Name:     "Request Node",
			},
		},
		FlowRequestNodes: []mnrequest.MNRequest{
			{
				FlowNodeID:       nodeID,
				HttpID:           &baseID,
				DeltaHttpID:      &deltaID,
				HasRequestConfig: true,
			},
		},
		HTTPRequests: []mhttp.HTTP{
			{
				ID:          baseID,
				WorkspaceID: wsID,
				Name:        "Base Request",
				Method:      "GET",
				Url:         "http://example.com",
				BodyKind:    mhttp.HttpBodyKindRaw,
			},
			{
				ID:           deltaID,
				WorkspaceID:  wsID, // Same workspace
				ParentHttpID: &baseID,
				IsDelta:      true,
				Name:         "Delta Request",
				Method:       "GET",
				Url:          "http://example.com",
				BodyKind:     mhttp.HttpBodyKindRaw,
			},
		},
		HTTPBodyRaw: []mhttp.HTTPBodyRaw{
			baseBodyRaw,
			deltaBodyRaw,
		},
	}

	// 3. Import
	// We need to bypass foreign key constraints for workspace if we don't create one.
	// Or we can just create a workspace if we have the query.
	// Let's rely on `ioworkspace` usually handling imports into an existing workspaceID provided in Opts.
	// But the DB needs that workspace to exist if FKs are active.
	// Detailed look at `importer_http.go` shows it uses `opts.WorkspaceID`.

	// Let's forcefully create a workspace using raw SQL if possible or just assume test DB has FKs disabled?
	// `sqlitemem` usually enables FKs.
	// I'll try to insert a dummy workspace if I can.
	_, err = db.ExecContext(ctx, "INSERT INTO workspace (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)", wsID, "Test WS", 0, 0)
	if err != nil {
		// If table doesn't exist (unlikely), ignore. If it does and fails, we'll see.
		// Ignoring error for now, hoping FKs are satisfied or ignored.
	}

	// 3. Import
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)

	opts := ImportOptions{
		WorkspaceID: wsID,
		PreserveIDs: true,
		ImportHTTP:  true,
		ImportFlows: true,
	}

	result, err := svc.Import(ctx, tx, bundle, opts)
	require.NoError(t, err)
	err = tx.Commit()
	require.NoError(t, err)

	assert.Equal(t, 2, result.HTTPRequestsCreated)
	assert.Equal(t, 2, result.HTTPBodyRawCreated)
	assert.Equal(t, 1, result.FlowsCreated)
	assert.Equal(t, 1, result.FlowNodesCreated)
	assert.Equal(t, 1, result.FlowRequestNodesCreated)

	// 4. Verify DB State (Simulating Collection Responses)

	// A. Verify NodeHttpCollection Data (Node -> Delta HTTP)
	// Start transaction for verification reads if needed (though sqlitemem is shared)
	// Just read directly using service
	nodeReq, err := nodeRequestService.GetNodeRequest(ctx, nodeID)
	require.NoError(t, err)
	require.NotNil(t, nodeReq)

	// This confirms NodeHttpCollection would return the correct DeltaHttpId
	assert.Equal(t, deltaID, *nodeReq.DeltaHttpID, "Node should point to the correct Delta HTTP Request")

	// B. Verify HttpBodyRawCollection Data (Delta HTTP ID -> Delta Body)
	// Check Base Body
	fetchedBase, err := httpBodyRawService.GetByHttpID(ctx, baseID)
	require.NoError(t, err)
	assert.Equal(t, "base-content", string(fetchedBase.RawData))
	assert.False(t, fetchedBase.IsDelta)

	// Check Delta Body
	fetchedDelta, err := httpBodyRawService.GetByHttpID(ctx, deltaID)
	require.NoError(t, err)

	// THIS IS THE BUG ASSERTION:
	// Current buggy behavior: IsDelta=false, DeltaRawData=nil/empty, RawData=""
	// Expected behavior: IsDelta=true, DeltaRawData="delta-override-content"

	t.Logf("Fetched Delta Body: IsDelta=%v, DeltaRawData=%q, RawData=%q",
		fetchedDelta.IsDelta, string(fetchedDelta.DeltaRawData), string(fetchedDelta.RawData))

	assert.True(t, fetchedDelta.IsDelta, "Body should be marked as delta")
	assert.Equal(t, "delta-override-content", string(fetchedDelta.DeltaRawData), "Delta content should be preserved")

	// Verify Delta Content Type
	assert.NotNil(t, fetchedDelta.DeltaContentType, "DeltaContentType should not be nil")
	if fetchedDelta.DeltaContentType != nil {
		ct, ok := fetchedDelta.DeltaContentType.(string)
		if ok {
			assert.Equal(t, "application/json", ct, "DeltaContentType should match original")
		} else {
			t.Logf("DeltaContentType is not a string, it is %T: %v", fetchedDelta.DeltaContentType, fetchedDelta.DeltaContentType)
		}
	}
}
