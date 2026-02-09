package test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	apiv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/http/v1"
)

// TestDeltaExecution_HeaderOverride verifies that a delta HTTP request with header overrides
// correctly sends the overridden header values to the server, and the version snapshot
// stores the merged headers.
func TestDeltaExecution_HeaderOverride(t *testing.T) {
	f := newDeltaExecutionFixture(t)

	var receivedHeaders http.Header
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	t.Cleanup(testServer.Close)

	// 1. Create Base HTTP Request
	baseID := idwrap.NewNow()
	err := f.httpService.Create(f.ctx, &mhttp.HTTP{
		ID:          baseID,
		WorkspaceID: f.workspaceID,
		Name:        "Base Request",
		Url:         testServer.URL,
		Method:      "GET",
	})
	require.NoError(t, err)

	// 2. Create base headers
	baseAuthHeaderID := idwrap.NewNow()
	err = f.httpHeaderService.Create(f.ctx, &mhttp.HTTPHeader{
		ID:      baseAuthHeaderID,
		HttpID:  baseID,
		Key:     "Authorization",
		Value:   "Bearer base-token",
		Enabled: true,
	})
	require.NoError(t, err)

	baseContentHeaderID := idwrap.NewNow()
	err = f.httpHeaderService.Create(f.ctx, &mhttp.HTTPHeader{
		ID:      baseContentHeaderID,
		HttpID:  baseID,
		Key:     "X-Custom",
		Value:   "base-value",
		Enabled: true,
	})
	require.NoError(t, err)

	// 3. Create Delta HTTP Request
	deltaID := idwrap.NewNow()
	err = f.httpService.Create(f.ctx, &mhttp.HTTP{
		ID:           deltaID,
		WorkspaceID:  f.workspaceID,
		Name:         "Delta Request",
		Url:          testServer.URL,
		Method:       "GET",
		IsDelta:      true,
		ParentHttpID: &baseID,
	})
	require.NoError(t, err)

	// 4. Create delta header records with overrides
	// Override: Authorization header value changes
	deltaAuthHeaderID := idwrap.NewNow()
	deltaAuthValue := "Bearer delta-token"
	err = f.httpHeaderService.Create(f.ctx, &mhttp.HTTPHeader{
		ID:                 deltaAuthHeaderID,
		HttpID:             deltaID,
		Key:                "Authorization",
		Value:              "Bearer base-token",
		Enabled:            true,
		ParentHttpHeaderID: &baseAuthHeaderID,
		IsDelta:            true,
		DeltaValue:         &deltaAuthValue,
	})
	require.NoError(t, err)

	// 5. Run the delta request
	_, err = f.handler.HttpRun(f.ctx, connect.NewRequest(&apiv1.HttpRunRequest{
		HttpId: deltaID.Bytes(),
	}))
	require.NoError(t, err)

	// 6. Verify the server received the correct MERGED headers
	require.Equal(t, "Bearer delta-token", receivedHeaders.Get("Authorization"),
		"Authorization header should be overridden by delta")
	require.Equal(t, "base-value", receivedHeaders.Get("X-Custom"),
		"X-Custom header should be inherited from base (no delta override)")

	// 7. Verify the version snapshot stores the correct merged headers
	versions, err := f.httpService.GetHttpVersionsByHttpID(f.ctx, deltaID)
	require.NoError(t, err)
	require.Len(t, versions, 1, "should have exactly one version after running")

	versionID := versions[0].ID
	snapshotHeaders, err := f.httpHeaderService.GetByHttpIDOrdered(f.ctx, versionID)
	require.NoError(t, err)
	require.Len(t, snapshotHeaders, 2, "snapshot should have 2 headers (base inherited + delta override)")

	// Build map for easy assertion
	headerMap := make(map[string]string)
	for _, h := range snapshotHeaders {
		headerMap[h.Key] = h.Value
	}
	require.Equal(t, "Bearer delta-token", headerMap["Authorization"],
		"snapshot should store the delta-overridden Authorization value")
	require.Equal(t, "base-value", headerMap["X-Custom"],
		"snapshot should store the base-inherited X-Custom value")
}

// TestDeltaExecution_NewHeader verifies that a new header added in a delta context
// (not an override of base) is correctly sent and stored in the version snapshot.
func TestDeltaExecution_NewHeader(t *testing.T) {
	f := newDeltaExecutionFixture(t)

	var receivedHeaders http.Header
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	t.Cleanup(testServer.Close)

	// 1. Create Base HTTP Request with one header
	baseID := idwrap.NewNow()
	err := f.httpService.Create(f.ctx, &mhttp.HTTP{
		ID:          baseID,
		WorkspaceID: f.workspaceID,
		Name:        "Base",
		Url:         testServer.URL,
		Method:      "GET",
	})
	require.NoError(t, err)

	baseHeaderID := idwrap.NewNow()
	err = f.httpHeaderService.Create(f.ctx, &mhttp.HTTPHeader{
		ID:      baseHeaderID,
		HttpID:  baseID,
		Key:     "X-Base",
		Value:   "base-only",
		Enabled: true,
	})
	require.NoError(t, err)

	// 2. Create Delta HTTP
	deltaID := idwrap.NewNow()
	err = f.httpService.Create(f.ctx, &mhttp.HTTP{
		ID:           deltaID,
		WorkspaceID:  f.workspaceID,
		Name:         "Delta",
		Url:          testServer.URL,
		Method:       "GET",
		IsDelta:      true,
		ParentHttpID: &baseID,
	})
	require.NoError(t, err)

	// 3. Add a NEW header in the delta context (no parent, not an override)
	newHeaderID := idwrap.NewNow()
	err = f.httpHeaderService.Create(f.ctx, &mhttp.HTTPHeader{
		ID:      newHeaderID,
		HttpID:  deltaID,
		Key:     "X-Delta-New",
		Value:   "delta-only",
		Enabled: true,
		IsDelta: false, // New addition, not an override
	})
	require.NoError(t, err)

	// 4. Run delta
	_, err = f.handler.HttpRun(f.ctx, connect.NewRequest(&apiv1.HttpRunRequest{
		HttpId: deltaID.Bytes(),
	}))
	require.NoError(t, err)

	// 5. Verify server received BOTH headers
	require.Equal(t, "base-only", receivedHeaders.Get("X-Base"),
		"base header should be inherited")
	require.Equal(t, "delta-only", receivedHeaders.Get("X-Delta-New"),
		"new delta header should be included")

	// 6. Verify snapshot
	versions, err := f.httpService.GetHttpVersionsByHttpID(f.ctx, deltaID)
	require.NoError(t, err)
	require.Len(t, versions, 1)

	snapshotHeaders, err := f.httpHeaderService.GetByHttpIDOrdered(f.ctx, versions[0].ID)
	require.NoError(t, err)
	require.Len(t, snapshotHeaders, 2, "snapshot should have 2 headers")

	headerMap := make(map[string]string)
	for _, h := range snapshotHeaders {
		headerMap[h.Key] = h.Value
	}
	require.Equal(t, "base-only", headerMap["X-Base"])
	require.Equal(t, "delta-only", headerMap["X-Delta-New"])
}

// TestDeltaExecution_HeaderDisabled verifies that disabling a header via delta override
// correctly excludes it from the executed request.
func TestDeltaExecution_HeaderDisabled(t *testing.T) {
	f := newDeltaExecutionFixture(t)

	var receivedHeaders http.Header
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	t.Cleanup(testServer.Close)

	// 1. Create Base with enabled header
	baseID := idwrap.NewNow()
	err := f.httpService.Create(f.ctx, &mhttp.HTTP{
		ID:          baseID,
		WorkspaceID: f.workspaceID,
		Name:        "Base",
		Url:         testServer.URL,
		Method:      "GET",
	})
	require.NoError(t, err)

	baseHeaderID := idwrap.NewNow()
	err = f.httpHeaderService.Create(f.ctx, &mhttp.HTTPHeader{
		ID:      baseHeaderID,
		HttpID:  baseID,
		Key:     "X-Disable-Me",
		Value:   "should-not-appear",
		Enabled: true,
	})
	require.NoError(t, err)

	// 2. Create Delta that disables the header
	deltaID := idwrap.NewNow()
	err = f.httpService.Create(f.ctx, &mhttp.HTTP{
		ID:           deltaID,
		WorkspaceID:  f.workspaceID,
		Name:         "Delta",
		Url:          testServer.URL,
		Method:       "GET",
		IsDelta:      true,
		ParentHttpID: &baseID,
	})
	require.NoError(t, err)

	disabledVal := false
	deltaHeaderID := idwrap.NewNow()
	err = f.httpHeaderService.Create(f.ctx, &mhttp.HTTPHeader{
		ID:                 deltaHeaderID,
		HttpID:             deltaID,
		Key:                "X-Disable-Me",
		Value:              "should-not-appear",
		Enabled:            true,
		ParentHttpHeaderID: &baseHeaderID,
		IsDelta:            true,
		DeltaEnabled:       &disabledVal,
	})
	require.NoError(t, err)

	// 3. Run delta
	_, err = f.handler.HttpRun(f.ctx, connect.NewRequest(&apiv1.HttpRunRequest{
		HttpId: deltaID.Bytes(),
	}))
	require.NoError(t, err)

	// 4. Verify the disabled header was NOT sent
	require.Empty(t, receivedHeaders.Get("X-Disable-Me"),
		"header disabled via delta should not be sent to server")
}

// TestDeltaExecution_HttpHeaderDeltaInsert_CreatesChildRecord verifies that calling
// HttpHeaderDeltaInsert with the delta HTTP ID and a base header ID creates a new
// delta child record on the delta HTTP, which the resolver correctly picks up.
func TestDeltaExecution_HttpHeaderDeltaInsert_CreatesChildRecord(t *testing.T) {
	f := newDeltaExecutionFixture(t)

	var receivedHeaders http.Header
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		_, _ = io.ReadAll(r.Body)
		w.Write([]byte("OK"))
	}))
	t.Cleanup(testServer.Close)

	// 1. Create Base HTTP with a header
	baseID := idwrap.NewNow()
	err := f.httpService.Create(f.ctx, &mhttp.HTTP{
		ID:          baseID,
		WorkspaceID: f.workspaceID,
		Name:        "Base",
		Url:         testServer.URL,
		Method:      "GET",
	})
	require.NoError(t, err)

	baseHeaderID := idwrap.NewNow()
	err = f.httpHeaderService.Create(f.ctx, &mhttp.HTTPHeader{
		ID:      baseHeaderID,
		HttpID:  baseID,
		Key:     "Authorization",
		Value:   "Bearer base-token",
		Enabled: true,
	})
	require.NoError(t, err)

	// 2. Create Delta HTTP
	deltaID := idwrap.NewNow()
	err = f.httpService.Create(f.ctx, &mhttp.HTTP{
		ID:           deltaID,
		WorkspaceID:  f.workspaceID,
		Name:         "Delta",
		Url:          testServer.URL,
		Method:       "GET",
		IsDelta:      true,
		ParentHttpID: &baseID,
	})
	require.NoError(t, err)

	// 3. Call HttpHeaderDeltaInsert with the delta HTTP ID — this should CREATE
	// a new delta child record on the delta HTTP.
	overrideValue := "Bearer delta-token"
	deltaHeaderID := idwrap.NewNow()
	_, err = f.handler.HttpHeaderDeltaInsert(f.ctx, connect.NewRequest(&apiv1.HttpHeaderDeltaInsertRequest{
		Items: []*apiv1.HttpHeaderDeltaInsert{
			{
				HttpId:            deltaID.Bytes(),
				HttpHeaderId:      baseHeaderID.Bytes(),
				DeltaHttpHeaderId: deltaHeaderID.Bytes(),
				Value:             &overrideValue,
			},
		},
	}))
	require.NoError(t, err)

	// 4. Verify a new delta child record was created on the delta HTTP
	createdHeader, err := f.httpHeaderService.GetByID(f.ctx, deltaHeaderID)
	require.NoError(t, err)
	require.True(t, createdHeader.IsDelta, "created record should be a delta")
	require.Equal(t, deltaID, createdHeader.HttpID, "created record should belong to delta HTTP")
	require.NotNil(t, createdHeader.ParentHttpHeaderID, "created record should reference the base header")
	require.Equal(t, baseHeaderID, *createdHeader.ParentHttpHeaderID)
	require.NotNil(t, createdHeader.DeltaValue)
	require.Equal(t, "Bearer delta-token", *createdHeader.DeltaValue)

	// 5. Verify the base header was NOT modified
	baseHeader, err := f.httpHeaderService.GetByID(f.ctx, baseHeaderID)
	require.NoError(t, err)
	require.Nil(t, baseHeader.DeltaValue, "base header should not have delta columns set")

	// 6. Run the delta request — resolver should now find the delta child record
	_, err = f.handler.HttpRun(f.ctx, connect.NewRequest(&apiv1.HttpRunRequest{
		HttpId: deltaID.Bytes(),
	}))
	require.NoError(t, err)

	// 7. Verify the server received the OVERRIDDEN header value
	require.Equal(t, "Bearer delta-token", receivedHeaders.Get("Authorization"),
		"resolver should pick up the delta child record and send the overridden value")

	// 8. Verify HttpHeaderDeltaUpdate succeeds on the created record
	updatedValue := "Bearer updated-delta-token"
	_, err = f.handler.HttpHeaderDeltaUpdate(f.ctx, connect.NewRequest(&apiv1.HttpHeaderDeltaUpdateRequest{
		Items: []*apiv1.HttpHeaderDeltaUpdate{
			{
				DeltaHttpHeaderId: deltaHeaderID.Bytes(),
				Value: &apiv1.HttpHeaderDeltaUpdate_ValueUnion{
					Kind:  apiv1.HttpHeaderDeltaUpdate_ValueUnion_KIND_VALUE,
					Value: &updatedValue,
				},
			},
		},
	}))
	require.NoError(t, err)

	// Verify the update took effect
	updatedHeader, err := f.httpHeaderService.GetByID(f.ctx, deltaHeaderID)
	require.NoError(t, err)
	require.NotNil(t, updatedHeader.DeltaValue)
	require.Equal(t, "Bearer updated-delta-token", *updatedHeader.DeltaValue)
}
