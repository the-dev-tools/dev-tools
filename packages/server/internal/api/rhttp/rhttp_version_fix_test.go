package rhttp

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/pkg/idwrap"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
)

func TestHttpVersionCollection_HasHttpId(t *testing.T) {
	f := newHttpFixture(t)
	ctx := f.ctx

	// 1. Create Workspace
	f.createWorkspace(t, "Test Workspace")

	// 2. Create HTTP Request
	httpID := idwrap.NewNow()
	_, err := f.handler.HttpInsert(ctx, connect.NewRequest(&apiv1.HttpInsertRequest{
		Items: []*apiv1.HttpInsert{
			{
				HttpId:   httpID.Bytes(),
				Name:     "Test Request",
				Method:   apiv1.HttpMethod_HTTP_METHOD_GET,
				Url:      "https://example.com",
				BodyKind: apiv1.HttpBodyKind_HTTP_BODY_KIND_RAW,
			},
		},
	}))
	require.NoError(t, err)

	// 3. Insert HttpVersion manually (raw SQL)
	_, err = f.base.DB.ExecContext(ctx, `
		INSERT INTO http_version (id, http_id, version_name, version_description, is_active, created_at, created_by)
		VALUES (?, ?, 'v1', 'Initial version', 1, ?, ?)
	`, idwrap.NewNow().Bytes(), httpID.Bytes(), time.Now().Unix(), f.userID.Bytes())
	require.NoError(t, err)

	// 4. Call HttpVersionCollection
	resp, err := f.handler.HttpVersionCollection(ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)

	// 5. Verify HttpId is present
	found := false
	for _, item := range resp.Msg.Items {
		if string(item.HttpId) == string(httpID.Bytes()) {
			found = true
			break
		}
	}
	require.True(t, found, "HttpId should be present in HttpVersionCollection response")
}
