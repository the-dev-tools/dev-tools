package rhttp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	httpv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
)

func TestHttpRun_Delta_MethodOverride(t *testing.T) {
	t.Parallel()

	// Server should receive POST (Delta override), not GET (Base)
	var receivedMethod string
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "delta-test-workspace")

	// 1. Create Base Request (GET)
	baseID := f.createHttpWithUrl(t, ws, "base-http", testServer.URL, "GET")

	// 2. Create Delta Request (POST Override)
	deltaID := idwrap.NewNow()
	deltaMethod := "POST"
	deltaReq := &mhttp.HTTP{
		ID:           deltaID,
		WorkspaceID:  ws,
		Name:         "delta-http",
		ParentHttpID: &baseID,
		IsDelta:      true,
		DeltaMethod:  &deltaMethod,
	}
	err := f.hs.Create(f.ctx, deltaReq)
	require.NoError(t, err)

	// 3. Run Delta Request
	req := connect.NewRequest(&httpv1.HttpRunRequest{
		HttpId: deltaID.Bytes(),
	})

	_, err = f.handler.HttpRun(f.ctx, req)
	require.NoError(t, err)

	// 4. Verify Server received POST
	require.Equal(t, "POST", receivedMethod, "Expected Delta Method (POST) to override Base Method (GET)")
}

func TestHttpRun_Delta_HeaderOverride(t *testing.T) {
	t.Parallel()

	// Server should receive Delta Header value
	var receivedHeader string
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get("X-Custom")
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "delta-test-workspace")

	// 1. Create Base Request
	baseID := f.createHttpWithUrl(t, ws, "base-http", testServer.URL, "GET")
	
	// Add Base Header
	f.createHttpHeader(t, baseID, "X-Custom", "BaseValue")

	// 2. Create Delta Request
	deltaID := idwrap.NewNow()
	deltaReq := &mhttp.HTTP{
		ID:           deltaID,
		WorkspaceID:  ws,
		Name:         "delta-http",
		ParentHttpID: &baseID,
		IsDelta:      true,
	}
	err := f.hs.Create(f.ctx, deltaReq)
	require.NoError(t, err)

	// Add Delta Header Override
	deltaHeaderID := idwrap.NewNow()
	deltaValue := "DeltaValue"
	deltaEnabled := true
	deltaHeader := &mhttp.HTTPHeader{
		ID:           deltaHeaderID,
		HttpID:       deltaID,
		IsDelta:      true,
		DeltaKey:     func() *string { s := "X-Custom"; return &s }(),
		DeltaValue:   &deltaValue,
		DeltaEnabled: &deltaEnabled,
	}
	// We need to link it to the base header to be an override, otherwise it's an addition.
	// But since we are using "X-Custom" key, the resolver *might* match by key if ParentHttpHeaderID is missing?
	// Let's check resolver logic. 
	// resolver.go: resolveHeaders: "For delta headers without parent ID, try to find matching base header by key name" -> NO, it doesn't seem to do that in the `pkg/delta` logic.
	// `pkg/delta` uses ID mapping.
	// `rhttp_integration_test.go` (disabled) implied key matching for legacy.
	// Let's explicitly set ParentHttpHeaderID to be safe and correct.
	
	// Get Base Header ID
	baseHeaders, _ := f.handler.httpHeaderService.GetByHttpIDOrdered(f.ctx, baseID)
	require.NotEmpty(t, baseHeaders)
	baseHeaderID := baseHeaders[0].ID
	deltaHeader.ParentHttpHeaderID = &baseHeaderID

	err = f.handler.httpHeaderService.Create(f.ctx, deltaHeader)
	require.NoError(t, err)

	// 3. Run Delta Request
	req := connect.NewRequest(&httpv1.HttpRunRequest{
		HttpId: deltaID.Bytes(),
	})

	_, err = f.handler.HttpRun(f.ctx, req)
	require.NoError(t, err)

	// 4. Verify Server received Delta Value
	require.Equal(t, "DeltaValue", receivedHeader)
}

func TestHttpRun_Delta_NewHeader(t *testing.T) {
	t.Parallel()

	// Server should receive New Delta Header
	var receivedHeader string
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get("X-New")
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "delta-test-workspace")

	// 1. Create Base Request
	baseID := f.createHttpWithUrl(t, ws, "base-http", testServer.URL, "GET")

	// 2. Create Delta Request
	deltaID := idwrap.NewNow()
	deltaReq := &mhttp.HTTP{
		ID:           deltaID,
		WorkspaceID:  ws,
		Name:         "delta-http",
		ParentHttpID: &baseID,
		IsDelta:      true,
	}
	err := f.hs.Create(f.ctx, deltaReq)
	require.NoError(t, err)

	// Add New Header to Delta (IsDelta=false means it's a new item in the delta context, not an override of parent)
	// Wait, for Delta requests, all child items are usually marked IsDelta=true if they are overrides.
	// If they are NEW additions, they are just items linked to the Delta HTTP ID.
	// BUT the resolver logic expects `IsDelta=false` for additions?
	// `resolveHeaders`: `if !d.IsDelta { additions = append(additions, d) }`
	// Yes.
	
	newHeaderID := idwrap.NewNow()
	newHeader := &mhttp.HTTPHeader{
		ID:      newHeaderID,
		HttpID:  deltaID,
		Key:     "X-New",
		Value:   "NewValue",
		Enabled: true,
		IsDelta: false, 
	}
	err = f.handler.httpHeaderService.Create(f.ctx, newHeader)
	require.NoError(t, err)

	// 3. Run Delta Request
	req := connect.NewRequest(&httpv1.HttpRunRequest{
		HttpId: deltaID.Bytes(),
	})

	_, err = f.handler.HttpRun(f.ctx, req)
	require.NoError(t, err)

	// 4. Verify Server received New Header
	require.Equal(t, "NewValue", receivedHeader)
}
