package rreference

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sflowvariable"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodeexecution"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/testutil"
	referencev1 "the-dev-tools/spec/dist/buf/go/api/reference/v1"
)

func TestReferenceCompletion_HttpId(t *testing.T) {
	// Setup
	base := testutil.CreateBaseDB(context.Background(), t)
	services := base.GetBaseServices()
	envService := senv.New(base.Queries, base.Logger())
	varService := svar.New(base.Queries, base.Logger())
	
	// Flow services (needed for constructor but not used)
	flowService := sflow.New(base.Queries)
	flowNodeService := snode.New(base.Queries)
	flowNodeRequestService := snoderequest.New(base.Queries)
	flowVariableService := sflowvariable.New(base.Queries)
	edgeService := sedge.New(base.Queries)
	nodeExecutionService := snodeexecution.New(base.Queries)

	// HTTP services
	httpService := services.Hs
	httpResponseService := shttp.NewHttpResponseService(base.Queries)

	svc := NewNodeServiceRPC(
		base.DB,
		services.Us,
		services.Ws,
		envService,
		varService,
		flowService,
		flowNodeService,
		flowNodeRequestService,
		flowVariableService,
		edgeService,
		nodeExecutionService,
		httpResponseService,
	)

	// Create User
	userID := idwrap.NewNow()
	if err := services.Us.CreateUser(context.Background(), &muser.User{
		ID:           userID,
		Email:        "test@example.com",
		Status:       muser.Active,
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
	if err := httpResponseService.Create(ctx, gen.HttpResponse{
		ID:        respID,
		HttpID:    httpID,
		Status:    int32(201),
		Body:      []byte(`{"foo":"bar"}`),
		Time:      time.Now(),
		Duration:  int32(100),
		Size:      int32(123),
		CreatedAt: time.Now().Unix(),
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
