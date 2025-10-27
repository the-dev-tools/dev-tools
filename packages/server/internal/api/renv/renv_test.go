package renv

import (
	"context"
	"testing"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/testutil"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/environment/v1"
)

func TestEnvironmentCollection(t *testing.T) {
	ctx := context.Background()
	baseDB := testutil.CreateBaseDB(ctx, t)
	defer baseDB.DB.Close()

	// Create services
	services := baseDB.GetBaseServices()
	es := senv.New(baseDB.Queries, baseDB.Logger())
	vs := svar.New(baseDB.Queries, baseDB.Logger())

	// Create handler
	handler := New(baseDB.DB, es, vs, services.Us)

	// Test EnvironmentCollection
	req := connect.NewRequest(&emptypb.Empty{})
	resp, err := handler.EnvironmentCollection(ctx, req)

	if err != nil {
		t.Fatalf("EnvironmentCollection failed: %v", err)
	}

	if resp == nil {
		t.Fatal("Response is nil")
	}

	if resp.Msg.Items == nil {
		t.Fatal("Items is nil")
	}

	// Should return empty collection for now
	if len(resp.Msg.Items) != 0 {
		t.Errorf("Expected 0 items, got %d", len(resp.Msg.Items))
	}
}

func TestEnvironmentCreate(t *testing.T) {
	ctx := context.Background()
	baseDB := testutil.CreateBaseDB(ctx, t)
	defer baseDB.DB.Close()

	// Create services
	services := baseDB.GetBaseServices()
	es := senv.New(baseDB.Queries, baseDB.Logger())
	vs := svar.New(baseDB.Queries, baseDB.Logger())

	// Create handler
	handler := New(baseDB.DB, es, vs, services.Us)

	// Create a test workspace first
	workspaceID := idwrap.NewNow()
	err := services.Ws.Create(ctx, &mworkspace.Workspace{
		ID:   workspaceID,
		Name: "Test Workspace",
	})
	if err != nil {
		t.Fatalf("Failed to create test workspace: %v", err)
	}

	// Test EnvironmentCreate
	req := connect.NewRequest(&apiv1.EnvironmentCreateRequest{
		Items: []*apiv1.EnvironmentCreate{
			{
				WorkspaceId: workspaceID.Bytes(),
				Name:        "Test Environment",
				Description: "A test environment",
			},
		},
	})

	// This will fail due to authentication/authorization, but should validate the structure
	_, err = handler.EnvironmentCreate(ctx, req)

	// We expect this to fail due to missing auth context, which is correct
	if err == nil {
		t.Error("Expected error, got nil")
	}

	// Check that it's the right kind of error (internal or unauthenticated is acceptable)
	code := connect.CodeOf(err)
	if code != connect.CodeInternal && code != connect.CodeUnauthenticated {
		t.Errorf("Expected CodeInternal or CodeUnauthenticated, got %v", code)
	}

	t.Logf("Got expected error: %v (code: %v)", err, code)
}
