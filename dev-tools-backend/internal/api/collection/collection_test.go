package collection_test

import (
	"context"
	"dev-tools-backend/internal/api/collection"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mcollection"
	"dev-tools-backend/pkg/model/muser"
	"dev-tools-backend/pkg/model/mworkspace"
	"dev-tools-backend/pkg/model/mworkspaceuser"
	"dev-tools-backend/pkg/service/scollection"
	"dev-tools-backend/pkg/service/suser"
	"dev-tools-backend/pkg/service/sworkspace"
	"dev-tools-backend/pkg/service/sworkspacesusers"
	"dev-tools-db/pkg/sqlc"
	"dev-tools-db/pkg/sqlc/gen"
	collectionv1 "dev-tools-spec/dist/buf/go/collection/v1"
	"testing"
	"time"

	"connectrpc.com/connect"
)

func TestCreateCollection(t *testing.T) {
	ctx := context.Background()

	db, err := sqlc.GetTestDB(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	queries := gen.New(db)
	if err != nil {
		t.Fatal(err)
	}

	cs := sqlc.GetService(ctx, queries, scollection.New)
	ws := sqlc.GetService(ctx, queries, sworkspace.New)
	wus := sqlc.GetService(ctx, queries, sworkspacesusers.New)
	us := sqlc.GetService(ctx, queries, suser.New)

	serviceRPC := collection.New(db, cs, ws, us)

	wsID := idwrap.NewNow()
	workspaceData := mworkspace.Workspace{
		Updated: time.Now(),
		Name:    "test",
		ID:      wsID,
	}

	err = ws.Create(ctx, &workspaceData)
	if err != nil {
		t.Error(err)
	}

	providerID := "test"
	userData := muser.User{
		ID:           idwrap.NewNow(),
		Email:        "test@dev.tools",
		Password:     []byte("test"),
		ProviderID:   &providerID,
		ProviderType: muser.MagicLink,
		Status:       muser.Active,
	}

	err = us.CreateUser(ctx, &userData)
	if err != nil {
		t.Error(err)
	}

	collectionID := idwrap.NewNow()
	collectionData := mcollection.Collection{
		ID:      collectionID,
		OwnerID: wsID,
		Name:    "test",
		Updated: time.Now(),
	}

	err = cs.CreateCollection(ctx, &collectionData)
	if err != nil {
		t.Error(err)
	}

	wsuserID := idwrap.NewNow()
	workspaceUserData := mworkspaceuser.WorkspaceUser{
		ID:          wsuserID,
		WorkspaceID: wsID,
		UserID:      userData.ID,
		Role:        mworkspaceuser.RoleAdmin,
	}

	err = wus.CreateWorkspaceUser(ctx, &workspaceUserData)
	if err != nil {
		t.Error(err)
	}

	req := connect.NewRequest(
		&collectionv1.CollectionGetRequest{
			CollectionId: collectionID.Bytes(),
		},
	)

	authedCtx := mwauth.CreateAuthedContext(ctx, userData.ID)

	resp, err := serviceRPC.CollectionGet(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg == nil {
		t.Fatalf("CollectionGet failed: invalid response")
	}
	msg := resp.Msg

	if msg.CollectionId == nil {
		t.Fatalf("CollectionGet failed: invalid response")
	}

	respCollectionID, err := idwrap.NewFromBytes(msg.CollectionId)
	if err != nil {
		t.Fatal(err)
	}

	if collectionID.Compare(respCollectionID) != 0 {
		t.Fatalf("CollectionGet failed: id mismatch")
	}

	if msg.Name != collectionData.Name {
		t.Fatalf("CollectionGet failed: invalid response")
	}
}
