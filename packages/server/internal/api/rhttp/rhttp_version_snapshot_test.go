package rhttp

import (
	"fmt"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	apiv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/http/v1"
)

// TestHttpRun_VersionSnapshotContainsChildren verifies that when HttpRun creates
// a version, it snapshots ALL child data (headers, search params, assertions) so
// that querying by version ID returns the full request state at that point in time.
func TestHttpRun_VersionSnapshotContainsChildren(t *testing.T) {
	t.Parallel()

	// Create a test server
	testServer := createTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Response-Header", "resp-value")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"success","data":{"id":42}}`)
	})
	defer testServer.Close()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttpWithUrl(t, ws, "snapshot-test", testServer.URL, "GET")

	// Add child entities to the HTTP entry
	f.createHttpHeader(t, httpID, "Authorization", "Bearer test-token")
	f.createHttpHeader(t, httpID, "Accept", "application/json")
	f.createHttpSearchParam(t, httpID, "page", "1")
	f.createHttpSearchParam(t, httpID, "limit", "10")
	f.createHttpAssertion(t, httpID, "response.status == 200", "Status check")

	// Execute HttpRun — this should create a version with snapshot data
	req := connect.NewRequest(&apiv1.HttpRunRequest{
		HttpId: httpID.Bytes(),
	})
	_, err := f.handler.HttpRun(f.ctx, req)
	require.NoError(t, err, "HttpRun should succeed")

	// Get versions to find the version ID
	versions, err := f.handler.getHttpVersionsByHttpID(f.ctx, httpID)
	require.NoError(t, err, "getHttpVersionsByHttpID should succeed")
	require.Len(t, versions, 1, "Should have exactly 1 version")
	versionID := versions[0].ID

	// Verify snapshot HTTP entry exists (by querying the service directly)
	snapshotHTTP, err := f.handler.httpReader.Get(f.ctx, versionID)
	require.NoError(t, err, "Snapshot HTTP entry should exist with version ID")
	require.True(t, snapshotHTTP.IsSnapshot, "Snapshot entry should have IsSnapshot=true")
	require.Equal(t, testServer.URL, snapshotHTTP.Url, "Snapshot should have same URL")
	require.Equal(t, "GET", snapshotHTTP.Method, "Snapshot should have same method")

	// Verify snapshot headers
	snapshotHeaders, err := f.handler.httpHeaderService.GetByHttpID(f.ctx, versionID)
	require.NoError(t, err, "Should get snapshot headers")
	require.Len(t, snapshotHeaders, 2, "Should have 2 cloned headers")

	headerKeys := map[string]string{}
	for _, h := range snapshotHeaders {
		headerKeys[h.Key] = h.Value
	}
	require.Equal(t, "Bearer test-token", headerKeys["Authorization"], "Authorization header should match")
	require.Equal(t, "application/json", headerKeys["Accept"], "Accept header should match")

	// Verify snapshot search params
	snapshotParams, err := f.handler.httpSearchParamService.GetByHttpID(f.ctx, versionID)
	require.NoError(t, err, "Should get snapshot search params")
	require.Len(t, snapshotParams, 2, "Should have 2 cloned search params")

	paramKeys := map[string]string{}
	for _, p := range snapshotParams {
		paramKeys[p.Key] = p.Value
	}
	require.Equal(t, "1", paramKeys["page"], "page param should match")
	require.Equal(t, "10", paramKeys["limit"], "limit param should match")

	// Verify snapshot assertions
	snapshotAsserts, err := f.handler.httpAssertService.GetByHttpID(f.ctx, versionID)
	require.NoError(t, err, "Should get snapshot assertions")
	require.Len(t, snapshotAsserts, 1, "Should have 1 cloned assertion")
	require.Equal(t, "response.status == 200", snapshotAsserts[0].Value, "Assertion value should match")

	// Verify snapshot response exists
	snapshotResponses, err := f.handler.httpResponseService.GetByHttpID(f.ctx, versionID)
	require.NoError(t, err, "Should get snapshot responses")
	require.Len(t, snapshotResponses, 1, "Should have 1 cloned response")
	require.Equal(t, int32(200), snapshotResponses[0].Status, "Response status should be 200")

	// Verify IDs are different from originals (they're clones, not references)
	origHeaders, err := f.handler.httpHeaderService.GetByHttpID(f.ctx, httpID)
	require.NoError(t, err)
	for _, sh := range snapshotHeaders {
		for _, oh := range origHeaders {
			require.NotEqual(t, oh.ID, sh.ID, "Snapshot header should have a new ID, not reuse original")
		}
	}
}

// TestHttpRun_MultipleVersionsHaveIndependentSnapshots verifies that running
// HttpRun multiple times creates independent snapshots. Modifying children
// between runs should result in different snapshot data for each version.
func TestHttpRun_MultipleVersionsHaveIndependentSnapshots(t *testing.T) {
	t.Parallel()

	testServer := createTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"ok":true}`)
	})
	defer testServer.Close()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttpWithUrl(t, ws, "multi-version", testServer.URL, "GET")

	// Run 1: with one header
	f.createHttpHeader(t, httpID, "X-Version", "v1")

	req := connect.NewRequest(&apiv1.HttpRunRequest{HttpId: httpID.Bytes()})
	_, err := f.handler.HttpRun(f.ctx, req)
	require.NoError(t, err)

	// Add a second header before run 2
	f.createHttpHeader(t, httpID, "X-Extra", "extra-value")

	// Run 2: with two headers
	_, err = f.handler.HttpRun(f.ctx, req)
	require.NoError(t, err)

	// Get both versions
	versions, err := f.handler.getHttpVersionsByHttpID(f.ctx, httpID)
	require.NoError(t, err)
	require.Len(t, versions, 2, "Should have 2 versions")

	// Version 1 (older) should have 1 header
	v1ID := versions[0].ID
	v1Headers, err := f.handler.httpHeaderService.GetByHttpID(f.ctx, v1ID)
	require.NoError(t, err)
	require.Len(t, v1Headers, 1, "Version 1 should have 1 header")
	require.Equal(t, "X-Version", v1Headers[0].Key)

	// Version 2 (newer) should have 2 headers
	v2ID := versions[1].ID
	v2Headers, err := f.handler.httpHeaderService.GetByHttpID(f.ctx, v2ID)
	require.NoError(t, err)
	require.Len(t, v2Headers, 2, "Version 2 should have 2 headers")

	// Each version should have exactly 1 response (not accumulated from previous runs)
	v1Responses, err := f.handler.httpResponseService.GetByHttpID(f.ctx, v1ID)
	require.NoError(t, err)
	require.Len(t, v1Responses, 1, "Version 1 should have exactly 1 response (not accumulated)")

	v2Responses, err := f.handler.httpResponseService.GetByHttpID(f.ctx, v2ID)
	require.NoError(t, err)
	require.Len(t, v2Responses, 1, "Version 2 should have exactly 1 response (not accumulated)")
}

// TestHttpRun_SnapshotExcludedFromWorkspaceQuery verifies that GetByWorkspaceID
// excludes snapshot entries (they should not appear in the sidebar/workspace tree).
func TestHttpRun_SnapshotExcludedFromWorkspaceQuery(t *testing.T) {
	t.Parallel()

	testServer := createStatusServer(t, http.StatusOK)
	defer testServer.Close()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttpWithUrl(t, ws, "main-list-test", testServer.URL, "GET")

	// Run to create a version (which creates a snapshot entry)
	req := connect.NewRequest(&apiv1.HttpRunRequest{HttpId: httpID.Bytes()})
	_, err := f.handler.HttpRun(f.ctx, req)
	require.NoError(t, err)

	// GetByWorkspaceID should only return base entries (no snapshots)
	httpList, err := f.handler.httpReader.GetByWorkspaceID(f.ctx, ws)
	require.NoError(t, err)

	var foundIDs []idwrap.IDWrap
	for _, h := range httpList {
		foundIDs = append(foundIDs, h.ID)
		require.False(t, h.IsSnapshot, "GetByWorkspaceID should not return snapshot entries")
	}
	require.Len(t, foundIDs, 1, "Should have exactly 1 HTTP entry")
	require.Equal(t, httpID, foundIDs[0], "The entry should be the original")
}

// TestHttpRun_SnapshotIncludedInCollection verifies that HttpCollection includes
// snapshot HTTP entries so the frontend's TanStack DB has the snapshot's method,
// URL, and body kind for display in Response History.
func TestHttpRun_SnapshotIncludedInCollection(t *testing.T) {
	t.Parallel()

	testServer := createStatusServer(t, http.StatusOK)
	defer testServer.Close()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttpWithUrl(t, ws, "collection-test", testServer.URL, "GET")

	// Run to create a version with a snapshot
	req := connect.NewRequest(&apiv1.HttpRunRequest{HttpId: httpID.Bytes()})
	_, err := f.handler.HttpRun(f.ctx, req)
	require.NoError(t, err)

	// Get version ID
	versions, err := f.handler.getHttpVersionsByHttpID(f.ctx, httpID)
	require.NoError(t, err)
	require.Len(t, versions, 1)
	versionID := versions[0].ID

	// HttpCollection should include BOTH the base entry AND the snapshot entry
	collResp, err := f.handler.HttpCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)

	foundBase := false
	foundSnapshot := false
	for _, item := range collResp.Msg.Items {
		itemID, err := idwrap.NewFromBytes(item.HttpId)
		require.NoError(t, err)
		if itemID == httpID {
			foundBase = true
		}
		if itemID == versionID {
			foundSnapshot = true
			// Verify snapshot has correct method/URL
			require.Equal(t, apiv1.HttpMethod_HTTP_METHOD_GET, item.Method, "Snapshot should have original method")
			require.Equal(t, testServer.URL, item.Url, "Snapshot should have original URL")
		}
	}
	require.True(t, foundBase, "HttpCollection should include the base HTTP entry")
	require.True(t, foundSnapshot, "HttpCollection should include the snapshot HTTP entry")
}

// TestHttpRun_AllCollectionsContainSnapshotData is a comprehensive end-to-end test
// that verifies ALL collection endpoints return snapshot data correctly. This proves
// the frontend's TanStack DB will have all the data it needs to display version details.
func TestHttpRun_AllCollectionsContainSnapshotData(t *testing.T) {
	t.Parallel()

	testServer := createTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Custom", "test-value")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"result":"ok"}`)
	})
	defer testServer.Close()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttpWithUrl(t, ws, "e2e-test", testServer.URL, "POST")

	// Add children
	f.createHttpHeader(t, httpID, "X-Test", "header-val")
	f.createHttpSearchParam(t, httpID, "q", "search")
	f.createHttpAssertion(t, httpID, "status == 200", "check")

	// Run
	req := connect.NewRequest(&apiv1.HttpRunRequest{HttpId: httpID.Bytes()})
	_, err := f.handler.HttpRun(f.ctx, req)
	require.NoError(t, err)

	versions, err := f.handler.getHttpVersionsByHttpID(f.ctx, httpID)
	require.NoError(t, err)
	require.Len(t, versions, 1)
	versionID := versions[0].ID

	// 1. HttpCollection: snapshot HTTP entry present with correct data
	t.Run("HttpCollection", func(t *testing.T) {
		resp, err := f.handler.HttpCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)
		found := false
		for _, item := range resp.Msg.Items {
			itemID, _ := idwrap.NewFromBytes(item.HttpId)
			if itemID == versionID {
				found = true
				require.Equal(t, apiv1.HttpMethod_HTTP_METHOD_POST, item.Method)
				require.Equal(t, testServer.URL, item.Url)
			}
		}
		require.True(t, found, "HttpCollection must include snapshot entry")
	})

	// 2. HttpHeaderCollection: snapshot headers present
	t.Run("HttpHeaderCollection", func(t *testing.T) {
		resp, err := f.handler.HttpHeaderCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)
		found := false
		for _, item := range resp.Msg.Items {
			itemHttpID, _ := idwrap.NewFromBytes(item.HttpId)
			if itemHttpID == versionID && item.Key == "X-Test" {
				found = true
				require.Equal(t, "header-val", item.Value)
			}
		}
		require.True(t, found, "HttpHeaderCollection must include snapshot headers")
	})

	// 3. HttpSearchParamCollection: snapshot params present
	t.Run("HttpSearchParamCollection", func(t *testing.T) {
		resp, err := f.handler.HttpSearchParamCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)
		found := false
		for _, item := range resp.Msg.Items {
			itemHttpID, _ := idwrap.NewFromBytes(item.HttpId)
			if itemHttpID == versionID && item.Key == "q" {
				found = true
				require.Equal(t, "search", item.Value)
			}
		}
		require.True(t, found, "HttpSearchParamCollection must include snapshot params")
	})

	// 4. HttpAssertCollection: snapshot asserts present
	t.Run("HttpAssertCollection", func(t *testing.T) {
		resp, err := f.handler.HttpAssertCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)
		found := false
		for _, item := range resp.Msg.Items {
			itemHttpID, _ := idwrap.NewFromBytes(item.HttpId)
			if itemHttpID == versionID {
				found = true
				require.Equal(t, "status == 200", item.Value)
			}
		}
		require.True(t, found, "HttpAssertCollection must include snapshot asserts")
	})

	// 5. HttpResponseCollection: snapshot response present with correct httpId
	t.Run("HttpResponseCollection", func(t *testing.T) {
		resp, err := f.handler.HttpResponseCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)
		found := false
		for _, item := range resp.Msg.Items {
			itemHttpID, _ := idwrap.NewFromBytes(item.HttpId)
			if itemHttpID == versionID {
				found = true
				require.Equal(t, int32(200), item.Status)
			}
		}
		require.True(t, found, "HttpResponseCollection must include snapshot response with httpId=versionID")
	})

	// 6. HttpResponseHeaderCollection: snapshot response headers present
	t.Run("HttpResponseHeaderCollection", func(t *testing.T) {
		resp, err := f.handler.HttpResponseHeaderCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)

		// Get the snapshot response ID first
		snapshotResponses, err := f.handler.httpResponseService.GetByHttpID(f.ctx, versionID)
		require.NoError(t, err)
		require.NotEmpty(t, snapshotResponses)
		snapshotRespID := snapshotResponses[0].ID

		found := false
		for _, item := range resp.Msg.Items {
			itemRespID, _ := idwrap.NewFromBytes(item.HttpResponseId)
			if itemRespID == snapshotRespID {
				found = true
				break
			}
		}
		require.True(t, found, "HttpResponseHeaderCollection must include snapshot response headers")
	})

	// 7. HttpVersionCollection: version record present with correct httpId
	t.Run("HttpVersionCollection", func(t *testing.T) {
		resp, err := f.handler.HttpVersionCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)
		found := false
		for _, item := range resp.Msg.Items {
			itemVersionID, _ := idwrap.NewFromBytes(item.HttpVersionId)
			if itemVersionID == versionID {
				found = true
				itemHttpID, _ := idwrap.NewFromBytes(item.HttpId)
				require.Equal(t, httpID, itemHttpID, "Version.httpId should reference the base HTTP entry")
			}
		}
		require.True(t, found, "HttpVersionCollection must include the version")
	})
}

// TestEveryVersionHasSnapshotData is an invariant test: every version in the
// collection MUST have a corresponding snapshot HTTP entry with at least a
// response. This would have caught the bug where UpdateHTTP created empty
// versions without snapshot data.
func TestEveryVersionHasSnapshotData(t *testing.T) {
	t.Parallel()

	testServer := createTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"ok":true}`)
	})
	defer testServer.Close()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "test-workspace")

	// Create multiple HTTP entries and run them
	http1 := f.createHttpWithUrl(t, ws, "endpoint-1", testServer.URL+"/1", "GET")
	http2 := f.createHttpWithUrl(t, ws, "endpoint-2", testServer.URL+"/2", "POST")

	f.createHttpHeader(t, http1, "X-Test", "value1")
	f.createHttpHeader(t, http2, "X-Test", "value2")

	// Run each endpoint multiple times
	for i := 0; i < 3; i++ {
		_, err := f.handler.HttpRun(f.ctx, connect.NewRequest(&apiv1.HttpRunRequest{HttpId: http1.Bytes()}))
		require.NoError(t, err)
		_, err = f.handler.HttpRun(f.ctx, connect.NewRequest(&apiv1.HttpRunRequest{HttpId: http2.Bytes()}))
		require.NoError(t, err)
	}

	// Also do some updates (which should NOT create versions)
	newName := "updated-name"
	_, err := f.handler.HttpUpdate(f.ctx, connect.NewRequest(&apiv1.HttpUpdateRequest{
		Items: []*apiv1.HttpUpdate{{HttpId: http1.Bytes(), Name: &newName}},
	}))
	require.NoError(t, err)

	// INVARIANT: every version in HttpVersionCollection must have snapshot data
	versionResp, err := f.handler.HttpVersionCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)

	require.Len(t, versionResp.Msg.Items, 6, "Should have exactly 6 versions (3 runs x 2 endpoints)")

	for _, v := range versionResp.Msg.Items {
		versionID, err := idwrap.NewFromBytes(v.HttpVersionId)
		require.NoError(t, err)

		// 1. Snapshot HTTP entry must exist
		snapshotHTTP, err := f.handler.httpReader.Get(f.ctx, versionID)
		require.NoError(t, err, "Version %s must have a snapshot HTTP entry", versionID)
		require.True(t, snapshotHTTP.IsSnapshot, "Version %s snapshot entry must have IsSnapshot=true", versionID)

		// 2. Snapshot must have at least one response
		responses, err := f.handler.httpResponseService.GetByHttpID(f.ctx, versionID)
		require.NoError(t, err)
		require.NotEmpty(t, responses, "Version %s must have at least one snapshot response", versionID)
	}
}

// TestHttpRun_DeltaVersionSnapshotContainsResolvedData verifies that running a delta
// HTTP entry produces a version snapshot with fully resolved (merged base+delta) data,
// not the raw delta entry with empty scalar fields.
func TestHttpRun_DeltaVersionSnapshotContainsResolvedData(t *testing.T) {
	t.Parallel()

	testServer := createTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"ok":true}`)
	})
	defer testServer.Close()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "delta-snapshot-test")

	// 1. Create Base Request with URL, method, headers, params
	baseID := f.createHttpWithUrl(t, ws, "base-request", testServer.URL, "GET")
	f.createHttpHeader(t, baseID, "Authorization", "Bearer base-token")
	f.createHttpHeader(t, baseID, "Accept", "application/json")
	f.createHttpSearchParam(t, baseID, "page", "1")
	f.createHttpAssertion(t, baseID, "status == 200", "Status check")

	// 2. Create Delta Request that overrides method
	deltaID := idwrap.NewNow()
	deltaMethod := "POST"
	deltaReq := &mhttp.HTTP{
		ID:           deltaID,
		WorkspaceID:  ws,
		Name:         "delta-request",
		ParentHttpID: &baseID,
		IsDelta:      true,
		DeltaMethod:  &deltaMethod,
	}
	err := f.hs.Create(f.ctx, deltaReq)
	require.NoError(t, err)

	// Add a new header to the delta (addition, not override)
	newHeaderID := idwrap.NewNow()
	newHeader := &mhttp.HTTPHeader{
		ID:      newHeaderID,
		HttpID:  deltaID,
		Key:     "X-Delta-Header",
		Value:   "delta-value",
		Enabled: true,
		IsDelta: false, // IsDelta=false means addition in resolver
	}
	err = f.handler.httpHeaderService.Create(f.ctx, newHeader)
	require.NoError(t, err)

	// 3. Run the Delta Request
	req := connect.NewRequest(&apiv1.HttpRunRequest{
		HttpId: deltaID.Bytes(),
	})
	_, err = f.handler.HttpRun(f.ctx, req)
	require.NoError(t, err)

	// 4. Get version and verify snapshot has resolved data
	versions, err := f.handler.getHttpVersionsByHttpID(f.ctx, deltaID)
	require.NoError(t, err)
	require.Len(t, versions, 1, "Should have exactly 1 version")
	versionID := versions[0].ID

	// Verify snapshot HTTP entry has resolved (merged) scalar fields
	snapshotHTTP, err := f.handler.httpReader.Get(f.ctx, versionID)
	require.NoError(t, err, "Snapshot HTTP entry should exist")
	require.True(t, snapshotHTTP.IsSnapshot, "Should be marked as snapshot")
	require.Equal(t, testServer.URL, snapshotHTTP.Url, "Snapshot URL should be the resolved base URL, not empty")
	require.Equal(t, "POST", snapshotHTTP.Method, "Snapshot method should be the delta override (POST), not empty or base (GET)")
	require.Equal(t, "base-request", snapshotHTTP.Name, "Snapshot name should be resolved from base")

	// Verify snapshot headers contain merged set (base headers + delta addition)
	snapshotHeaders, err := f.handler.httpHeaderService.GetByHttpID(f.ctx, versionID)
	require.NoError(t, err, "Should get snapshot headers")
	require.Len(t, snapshotHeaders, 3, "Should have 3 merged headers: 2 from base + 1 delta addition")

	headerKeys := map[string]string{}
	for _, h := range snapshotHeaders {
		headerKeys[h.Key] = h.Value
	}
	require.Equal(t, "Bearer base-token", headerKeys["Authorization"], "Base header should be in snapshot")
	require.Equal(t, "application/json", headerKeys["Accept"], "Base header should be in snapshot")
	require.Equal(t, "delta-value", headerKeys["X-Delta-Header"], "Delta addition header should be in snapshot")

	// Verify snapshot search params contain base params
	snapshotParams, err := f.handler.httpSearchParamService.GetByHttpID(f.ctx, versionID)
	require.NoError(t, err, "Should get snapshot search params")
	require.Len(t, snapshotParams, 1, "Should have 1 param from base")
	require.Equal(t, "page", snapshotParams[0].Key)

	// Verify snapshot has response
	snapshotResponses, err := f.handler.httpResponseService.GetByHttpID(f.ctx, versionID)
	require.NoError(t, err, "Should get snapshot responses")
	require.Len(t, snapshotResponses, 1, "Should have 1 response")
	require.Equal(t, int32(200), snapshotResponses[0].Status)
}
