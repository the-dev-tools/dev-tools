package rhttp

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
)

func TestHttpInsert_Success(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	// Create a workspace for the user
	_ = f.createWorkspace(t, "test-workspace")

	httpID := idwrap.NewNow()
	req := connect.NewRequest(&apiv1.HttpInsertRequest{
		Items: []*apiv1.HttpInsert{
			{
				HttpId:   httpID.Bytes(),
				Name:     "new-http",
				Url:      "https://example.com",
				Method:   apiv1.HttpMethod_HTTP_METHOD_POST,
				BodyKind: apiv1.HttpBodyKind_HTTP_BODY_KIND_RAW,
			},
		},
	})

	_, err := f.handler.HttpInsert(f.ctx, req)
	require.NoError(t, err)

	// Verify it was created
	httpModel, err := f.hs.Get(f.ctx, httpID)
	require.NoError(t, err)
	require.Equal(t, "new-http", httpModel.Name)
	require.Equal(t, "https://example.com", httpModel.Url)
	require.Equal(t, "POST", httpModel.Method)
	require.Equal(t, mhttp.HttpBodyKindRaw, httpModel.BodyKind)
}

func TestHttpInsert_Validation(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)

	// No items
	req := connect.NewRequest(&apiv1.HttpInsertRequest{
		Items: []*apiv1.HttpInsert{},
	})
	_, err := f.handler.HttpInsert(f.ctx, req)
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	// No workspace
	// Create a fresh fixture but don't create a workspace
	f2 := newHttpFixture(t)
	httpID := idwrap.NewNow()
	req2 := connect.NewRequest(&apiv1.HttpInsertRequest{
		Items: []*apiv1.HttpInsert{
			{
				HttpId: httpID.Bytes(),
				Name:   "fail-http",
			},
		},
	})
	_, err = f2.handler.HttpInsert(f2.ctx, req2)
	require.Error(t, err)
	// Expect NotFound because "user has no workspaces" check
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestHttpUpdate_Success(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttp(t, ws, "old-name")

	newName := "updated-name"
	newUrl := "https://updated.com"
	newMethod := apiv1.HttpMethod_HTTP_METHOD_PUT
	newBodyKind := apiv1.HttpBodyKind_HTTP_BODY_KIND_FORM_DATA

	req := connect.NewRequest(&apiv1.HttpUpdateRequest{
		Items: []*apiv1.HttpUpdate{
			{
				HttpId:   httpID.Bytes(),
				Name:     &newName,
				Url:      &newUrl,
				Method:   &newMethod,
				BodyKind: &newBodyKind,
			},
		},
	})

	_, err := f.handler.HttpUpdate(f.ctx, req)
	require.NoError(t, err)

	// Verify update
	httpModel, err := f.hs.Get(f.ctx, httpID)
	require.NoError(t, err)
	require.Equal(t, newName, httpModel.Name)
	require.Equal(t, newUrl, httpModel.Url)
	require.Equal(t, "PUT", httpModel.Method)
	require.Equal(t, mhttp.HttpBodyKindFormData, httpModel.BodyKind)
}

func TestHttpUpdate_Partial(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttpWithUrl(t, ws, "original-name", "https://original.com", "GET")

	newName := "updated-name-only"
	req := connect.NewRequest(&apiv1.HttpUpdateRequest{
		Items: []*apiv1.HttpUpdate{
			{
				HttpId: httpID.Bytes(),
				Name:   &newName,
			},
		},
	})

	_, err := f.handler.HttpUpdate(f.ctx, req)
	require.NoError(t, err)

	// Verify update
	httpModel, err := f.hs.Get(f.ctx, httpID)
	require.NoError(t, err)
	require.Equal(t, newName, httpModel.Name)
	// These should remain unchanged
	require.Equal(t, "https://original.com", httpModel.Url)
	require.Equal(t, "GET", httpModel.Method)
}

func TestHttpUpdate_CreatesVersion(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttp(t, ws, "version-test")

	// Verify no versions initially
	versions, err := f.hs.GetHttpVersionsByHttpID(f.ctx, httpID)
	require.NoError(t, err)
	require.Empty(t, versions)

	// Perform update
	newName := "updated-version-test"
	req := connect.NewRequest(&apiv1.HttpUpdateRequest{
		Items: []*apiv1.HttpUpdate{
			{
				HttpId: httpID.Bytes(),
				Name:   &newName,
			},
		},
	})

	_, err = f.handler.HttpUpdate(f.ctx, req)
	require.NoError(t, err)

	// Verify version created
	versions, err = f.hs.GetHttpVersionsByHttpID(f.ctx, httpID)
	require.NoError(t, err)
	require.Len(t, versions, 1)
	require.Equal(t, httpID, versions[0].HttpID)
}

func TestHttpDelete_Success(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttp(t, ws, "to-delete")

	req := connect.NewRequest(&apiv1.HttpDeleteRequest{
		Items: []*apiv1.HttpDelete{
			{
				HttpId: httpID.Bytes(),
			},
		},
	})

	_, err := f.handler.HttpDelete(f.ctx, req)
	require.NoError(t, err)

	// Verify deleted - we expect an error when getting it
	_, err = f.hs.Get(f.ctx, httpID)
	require.Error(t, err)
}

func TestHttpCollection_Success(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	_ = f.createHttp(t, ws, "http-1")
	_ = f.createHttp(t, ws, "http-2")

	req := connect.NewRequest(&emptypb.Empty{})
	resp, err := f.handler.HttpCollection(f.ctx, req)
	require.NoError(t, err)

	require.Len(t, resp.Msg.Items, 2)
	// Basic check that we got items back
	found1 := false
	found2 := false
	for _, item := range resp.Msg.Items {
		if item.Name == "http-1" {
			found1 = true
		}
		if item.Name == "http-2" {
			found2 = true
		}
	}
	require.True(t, found1)
	require.True(t, found2)
}

func TestHttpDuplicate_Success(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttpWithUrl(t, ws, "original", "https://original.com", "GET")

	// Add some children to verify they are copied
	f.createHttpHeader(t, httpID, "X-Test", "value")
	f.createHttpSearchParam(t, httpID, "q", "search")

	req := connect.NewRequest(&apiv1.HttpDuplicateRequest{
		HttpId: httpID.Bytes(),
	})

	_, err := f.handler.HttpDuplicate(f.ctx, req)
	require.NoError(t, err)

	// We can't easily know the new ID, so we list all HTTPs in workspace
	httpList, err := f.hs.GetByWorkspaceID(f.ctx, ws)
	require.NoError(t, err)
	require.Len(t, httpList, 2)

	var duplicate mhttp.HTTP
	found := false
	for _, h := range httpList {
		if h.ID != httpID {
			duplicate = h
			found = true
			break
		}
	}
	require.True(t, found, "Duplicate not found")
	require.Equal(t, "Copy of original", duplicate.Name)
	require.Equal(t, "https://original.com", duplicate.Url)

	// Check children copied
	headers, err := f.handler.httpHeaderService.GetByHttpID(f.ctx, duplicate.ID)
	require.NoError(t, err)
	require.Len(t, headers, 1)
	require.Equal(t, "X-Test", headers[0].Key)

	params, err := f.handler.httpSearchParamService.GetByHttpID(f.ctx, duplicate.ID)
	require.NoError(t, err)
	require.Len(t, params, 1)
	require.Equal(t, "q", params[0].Key)
}

func TestHttpVersionCollection_Success(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttp(t, ws, "version-test")

	// Create a version manually or via update
	// Let's use Update to create a version
	newName := "updated-version-test"
	updateReq := connect.NewRequest(&apiv1.HttpUpdateRequest{
		Items: []*apiv1.HttpUpdate{
			{
				HttpId: httpID.Bytes(),
				Name:   &newName,
			},
		},
	})
	_, err := f.handler.HttpUpdate(f.ctx, updateReq)
	require.NoError(t, err)

	// Test Collection
	req := connect.NewRequest(&emptypb.Empty{})
	resp, err := f.handler.HttpVersionCollection(f.ctx, req)
	require.NoError(t, err)

	require.NotEmpty(t, resp.Msg.Items)
	require.Equal(t, string(httpID.Bytes()), string(resp.Msg.Items[0].HttpId))
}
