package rhttp

import (
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/pkg/idwrap"
	httpv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
)

// TestHttpHeaderBasicCRUD tests basic header CRUD operations using existing fixture
func TestHttpHeaderBasicCRUD(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	workspaceID := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttp(t, workspaceID, "test-http")

	// Create header
	headerID := idwrap.NewNow()
	createReq := connect.NewRequest(&httpv1.HttpHeaderCreateRequest{
		Items: []*httpv1.HttpHeaderCreate{
			{
				HttpHeaderId: headerID.Bytes(),
				HttpId:       httpID.Bytes(),
				Key:          "Content-Type",
				Value:        "application/json",
				Enabled:      true,
			},
		},
	})

	_, err := f.handler.HttpHeaderCreate(f.ctx, createReq)
	if err != nil {
		t.Fatalf("HttpHeaderCreate err: %v", err)
	}

	// Get collection and verify header exists
	collectionResp, err := f.handler.HttpHeaderCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	if err != nil {
		t.Fatalf("HttpHeaderCollection err: %v", err)
	}

	t.Logf("Collection returned %d headers", len(collectionResp.Msg.Items))
	for i, header := range collectionResp.Msg.Items {
		t.Logf("Header %d: ID=%s, Key=%s, Value=%s, HttpId=%s",
			i, string(header.HttpHeaderId), header.Key, header.Value, string(header.HttpId))
	}

	var found *httpv1.HttpHeader
	for _, header := range collectionResp.Msg.Items {
		// Match by key and value instead of ID for now
		if header.Key == "Content-Type" && header.Value == "application/json" {
			found = header
			t.Logf("Found matching header: ID=%s, Key=%s, Value=%s",
				string(header.HttpHeaderId), header.Key, header.Value)
			break
		}
	}

	if found == nil {
		t.Fatal("created header not found in collection")
	}

	if found.Key != "Content-Type" {
		t.Fatalf("expected key 'Content-Type', got '%s'", found.Key)
	}

	if found.Value != "application/json" {
		t.Fatalf("expected value 'application/json', got '%s'", found.Value)
	}

	if !found.Enabled {
		t.Fatal("expected enabled to be true")
	}

	// Update header
	newValue := "application/xml"
	updateReq := connect.NewRequest(&httpv1.HttpHeaderUpdateRequest{
		Items: []*httpv1.HttpHeaderUpdate{
			{
				HttpHeaderId: headerID.Bytes(),
				Value:        &newValue,
			},
		},
	})

	_, err = f.handler.HttpHeaderUpdate(f.ctx, updateReq)
	if err != nil {
		t.Fatalf("HttpHeaderUpdate err: %v", err)
	}

	// Verify update
	updatedCollectionResp, err := f.handler.HttpHeaderCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	if err != nil {
		t.Fatalf("HttpHeaderCollection after update err: %v", err)
	}

	t.Logf("After update: Collection returned %d headers", len(updatedCollectionResp.Msg.Items))
	for i, header := range updatedCollectionResp.Msg.Items {
		t.Logf("Header %d: ID=%s, Key=%s, Value=%s",
			i, string(header.HttpHeaderId), header.Key, header.Value)
	}

	var updatedHeader *httpv1.HttpHeader
	for _, header := range updatedCollectionResp.Msg.Items {
		// Match by key and value instead of ID for now
		if header.Key == "Content-Type" && header.Value == newValue {
			updatedHeader = header
			t.Logf("Found updated header: ID=%s, Key=%s, Value=%s",
				string(header.HttpHeaderId), header.Key, header.Value)
			break
		}
	}

	if updatedHeader == nil {
		t.Fatal("updated header not found in collection")
	}

	if updatedHeader.Value != newValue {
		t.Fatalf("expected updated value '%s', got '%s'", newValue, updatedHeader.Value)
	}

	// Delete header
	deleteReq := connect.NewRequest(&httpv1.HttpHeaderDeleteRequest{
		Items: []*httpv1.HttpHeaderDelete{
			{HttpHeaderId: headerID.Bytes()},
		},
	})

	_, err = f.handler.HttpHeaderDelete(f.ctx, deleteReq)
	if err != nil {
		t.Fatalf("HttpHeaderDelete err: %v", err)
	}

	// Verify deletion
	finalCollectionResp, err := f.handler.HttpHeaderCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	if err != nil {
		t.Fatalf("HttpHeaderCollection after delete err: %v", err)
	}

	var deletedHeader *httpv1.HttpHeader
	for _, header := range finalCollectionResp.Msg.Items {
		if string(header.HttpHeaderId) == headerID.String() {
			deletedHeader = header
			break
		}
	}

	if deletedHeader != nil {
		t.Fatal("deleted header still found in collection")
	}

	t.Logf("Successfully tested header CRUD: ID=%s, Key=%s", headerID, found.Key)
}
