package rhttp

import (
	"bytes"
	"testing"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"

	httpv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestHttpHeaderDeltaCollection_ReturnsCorrectDeltas(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "delta-test-workspace")

	// 1. Setup Base Request & Header
	baseHttpID := f.createHttp(t, ws, "Base Request")

	baseHeaderID := idwrap.NewNow()
	baseHeader := &mhttp.HTTPHeader{
		ID:      baseHeaderID,
		HttpID:  baseHttpID,
		Key:     "X-Base",
		Value:   "true",
		Enabled: true,
		IsDelta: false,
	}
	if err := f.handler.httpHeaderService.Create(f.ctx, baseHeader); err != nil {
		t.Fatalf("failed to create base header: %v", err)
	}

	// 2. Create Delta Header (Override)
	deltaHttpID := idwrap.NewNow()
	deltaHttp := &mhttp.HTTP{
		ID:           deltaHttpID,
		WorkspaceID:  ws,
		Name:         "Delta Request",
		ParentHttpID: &baseHttpID,
		IsDelta:      true,
	}
	if err := f.hs.Create(f.ctx, deltaHttp); err != nil {
		t.Fatalf("failed to create delta http: %v", err)
	}

	deltaHeaderID := idwrap.NewNow()
	deltaValue := "false"
	deltaHeader := &mhttp.HTTPHeader{
		ID:                 deltaHeaderID,
		HttpID:             deltaHttpID,   // The Delta Request this override belongs to
		ParentHttpHeaderID: &baseHeaderID, // The Base Header this overrides
		IsDelta:            true,
		DeltaValue:         &deltaValue, // Override
	}
	// Create the delta header. Assuming Create handles IsDelta correctly (it does based on schema)
	if err := f.handler.httpHeaderService.Create(f.ctx, deltaHeader); err != nil {
		t.Fatalf("failed to create delta header: %v", err)
	}

	// 3. Call RPC
	resp, err := f.handler.HttpHeaderDeltaCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	if err != nil {
		t.Fatalf("HttpHeaderDeltaCollection failed: %v", err)
	}

	// 4. Verify logic
	var foundDelta *httpv1.HttpHeaderDelta
	for _, item := range resp.Msg.Items {
		if bytes.Equal(item.DeltaHttpHeaderId, deltaHeaderID.Bytes()) {
			foundDelta = item
			break
		}
	}

	if foundDelta == nil {
		t.Fatal("Delta header not found in response")
	}

	// CHECK 1: HttpHeaderId should be the PARENT ID (Base Header ID)
	if !bytes.Equal(foundDelta.HttpHeaderId, baseHeaderID.Bytes()) {
		gotID, _ := idwrap.NewFromBytes(foundDelta.HttpHeaderId)
		t.Errorf("Expected HttpHeaderId to be %s (Base), got %s", baseHeaderID, gotID)
	}

	// CHECK 2: Value should be the delta override
	if foundDelta.Value == nil || *foundDelta.Value != deltaValue {
		t.Errorf("Expected Value to be %s, got %v", deltaValue, foundDelta.Value)
	}

	// CHECK 3: Base header should NOT be returned as a delta
	for _, item := range resp.Msg.Items {
		if bytes.Equal(item.DeltaHttpHeaderId, baseHeaderID.Bytes()) {
			t.Error("Base header incorrectly returned in Delta Collection")
		}
	}
}
