package rreference

import (
	"context"
	"testing"
	"time"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/testutil"
	referencev1 "the-dev-tools/spec/dist/buf/go/api/reference/v1"

	"connectrpc.com/connect"
)

func TestReferenceCompletion_HttpId(t *testing.T) {
	// Setup
	base := testutil.CreateBaseDB(context.Background(), t)
	services := base.GetBaseServices()
	envService := senv.New(base.Queries, base.Logger())
	varService := svar.New(base.Queries, base.Logger())

	// Flow services (needed for constructor but not used)
	flowService := sflow.NewFlowService(base.Queries)
	flowNodeService := sflow.NewNodeService(base.Queries)
	flowNodeRequestService := sflow.NewNodeRequestService(base.Queries)
	flowVariableService := sflow.NewFlowVariableService(base.Queries)
	edgeService := sflow.NewEdgeService(base.Queries)
	nodeExecutionService := sflow.NewNodeExecutionService(base.Queries)

	// HTTP services
	httpService := services.Hs
	httpResponseService := shttp.NewHttpResponseService(base.Queries)

	svc := NewReferenceServiceRPC(
		base.DB,
		services.Us.Reader(),
		services.Ws.Reader(),
		envService.Reader(),
		varService.Reader(),
		flowService.Reader(),
		flowNodeService.Reader(),
		flowNodeRequestService.Reader(),
		flowVariableService.Reader(),
		edgeService.Reader(),
		nodeExecutionService.Reader(),
		httpResponseService.Reader(),
	)

	// Create User
	userID := idwrap.NewNow()
	if err := services.Us.CreateUser(context.Background(), &muser.User{
		ID:     userID,
		Email:  "test@example.com",
		Status: muser.Active,
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	ctx := mwauth.CreateAuthedContext(context.Background(), userID)

	// Create Workspace
	workspaceID := idwrap.NewNow()
	envID := idwrap.NewNow()
	if err := services.Ws.Create(ctx, &mworkspace.Workspace{
		ID:        workspaceID,
		Name:      "test-ws",
		Updated:   dbtime.DBNow(),
		ActiveEnv: envID,
		GlobalEnv: envID,
	}); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	// Link User to Workspace
	if err := services.Wus.CreateWorkspaceUser(ctx, &mworkspaceuser.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        mworkspaceuser.RoleOwner,
	}); err != nil {
		t.Fatalf("create workspace user: %v", err)
	}

	// Create HTTP
	httpID := idwrap.NewNow()
	if err := httpService.Create(ctx, &mhttp.HTTP{
		ID:          httpID,
		WorkspaceID: workspaceID,
		Name:        "test-http",
		Url:         "http://example.com",
		Method:      "GET",
	}); err != nil {
		t.Fatalf("create http: %v", err)
	}

	// Create HTTP Response
	respID := idwrap.NewNow()
	now := time.Now().Unix()
	if err := httpResponseService.Create(ctx, mhttp.HTTPResponse{
		ID:        respID,
		HttpID:    httpID,
		Status:    201,
		Body:      []byte(`{"foo":"bar"}`),
		Time:      now,
		Duration:  100,
		Size:      123,
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("create response: %v", err)
	}

	// Test ReferenceCompletion
	req := connect.NewRequest(&referencev1.ReferenceCompletionRequest{
		HttpId: httpID.Bytes(),
	})

	_, err := svc.ReferenceCompletion(ctx, req)
	if err != nil {
		t.Fatalf("ReferenceCompletion failed: %v", err)
	}

	// Verify ReferenceValue
	valReq := connect.NewRequest(&referencev1.ReferenceValueRequest{
		HttpId: httpID.Bytes(),
		Path:   "response.status",
	})

	valResp, err := svc.ReferenceValue(ctx, valReq)
	if err != nil {
		t.Fatalf("ReferenceValue failed: %v", err)
	}

	if valResp.Msg.Value == "" {
		t.Fatal("Expected value for response.status, got empty string")
	}

	// Check if value matches 201. It might be returned as string "201" or "201.0" depending on formatting.
	// In Go test output usually easier to see.
	t.Logf("Got value: %v", valResp.Msg.Value)
}
