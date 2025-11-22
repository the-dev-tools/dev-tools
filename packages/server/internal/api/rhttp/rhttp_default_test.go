package rhttp

import (
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
	"the-dev-tools/server/pkg/idwrap"
	httpv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
)

func TestHttpInsert_DefaultBodyKind(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	f.createWorkspace(t, "test-workspace") // Ensure user has a workspace

	// Test Case 1: Unspecified BodyKind (should default to None)
	httpID1 := idwrap.NewNow()
	createReq1 := connect.NewRequest(&httpv1.HttpInsertRequest{
		Items: []*httpv1.HttpInsert{
			{
				HttpId: httpID1.Bytes(),
				Name:   "unspecified-body-kind",
				Url:    "https://example.com",
				Method: httpv1.HttpMethod_HTTP_METHOD_GET,
				// BodyKind is omitted (0 / UNSPECIFIED)
			},
		},
	})

	if _, err := f.handler.HttpInsert(f.ctx, createReq1); err != nil {
		t.Fatalf("HttpInsert failed: %v", err)
	}

	// Retrieve the item
	resp, err := f.handler.HttpCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	if err != nil {
		t.Fatalf("HttpCollection failed: %v", err)
	}

	var foundUnspecified bool
	for _, item := range resp.Msg.Items {
		if string(item.HttpId) == string(httpID1.Bytes()) {
			foundUnspecified = true
			// Expect UNSPECIFIED (0)
			if item.BodyKind != httpv1.HttpBodyKind_HTTP_BODY_KIND_UNSPECIFIED {
				t.Errorf("Expected BodyKind UNSPECIFIED for unspecified insert, got %v", item.BodyKind)
			}
		}
	}
	if !foundUnspecified {
		t.Fatal("Did not find inserted item 1")
	}

	// Test Case 2: Explicit FormData (should remain FormData)
	httpID2 := idwrap.NewNow()
	createReq2 := connect.NewRequest(&httpv1.HttpInsertRequest{
		Items: []*httpv1.HttpInsert{
			{
				HttpId:   httpID2.Bytes(),
				Name:     "form-data-body-kind",
				Url:      "https://example.com",
				Method:   httpv1.HttpMethod_HTTP_METHOD_POST,
				BodyKind: httpv1.HttpBodyKind_HTTP_BODY_KIND_FORM_DATA,
			},
		},
	})

	if _, err := f.handler.HttpInsert(f.ctx, createReq2); err != nil {
		t.Fatalf("HttpInsert failed: %v", err)
	}

	// Retrieve again
	resp, err = f.handler.HttpCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	if err != nil {
		t.Fatalf("HttpCollection failed: %v", err)
	}

	var foundFormData bool
	for _, item := range resp.Msg.Items {
		if string(item.HttpId) == string(httpID2.Bytes()) {
			foundFormData = true
			// Expect FORM_DATA (1)
			if item.BodyKind != httpv1.HttpBodyKind_HTTP_BODY_KIND_FORM_DATA {
				t.Errorf("Expected BodyKind FORM_DATA for explicit insert, got %v", item.BodyKind)
			}
		}
	}
	if !foundFormData {
		t.Fatal("Did not find inserted item 2")
	}
}
