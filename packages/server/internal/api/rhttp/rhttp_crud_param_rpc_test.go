package rhttp

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	apiv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/http/v1"
)

func TestHttpSearchParamInsert_Success(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttp(t, ws, "test-http")

	paramID := idwrap.NewNow()
	key := "param-key"
	value := "param-value"
	description := "test param"
	enabled := true
	order := float32(1)

	req := connect.NewRequest(&apiv1.HttpSearchParamInsertRequest{
		Items: []*apiv1.HttpSearchParamInsert{
			{
				HttpSearchParamId: paramID.Bytes(),
				HttpId:            httpID.Bytes(),
				Key:               key,
				Value:             value,
				Description:       description,
				Enabled:           enabled,
				Order:             order,
			},
		},
	})

	_, err := f.handler.HttpSearchParamInsert(f.ctx, req)
	require.NoError(t, err)

	// Verify
	param, err := f.handler.httpSearchParamService.GetByID(f.ctx, paramID)
	require.NoError(t, err)
	require.Equal(t, key, param.Key)
	require.Equal(t, value, param.Value)
	require.Equal(t, description, param.Description)
	require.Equal(t, enabled, param.Enabled)
	require.Equal(t, float64(order), param.DisplayOrder)
}

func TestHttpSearchParamUpdate_Success(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttp(t, ws, "test-http")

	// Create initial param directly to have the ID
	paramID := idwrap.NewNow()
	initialParam := &mhttp.HTTPSearchParam{
		ID:      paramID,
		HttpID:  httpID,
		Key:     "old-key",
		Value:   "old-value",
		Enabled: true,
	}
	err := f.handler.httpSearchParamService.Create(f.ctx, initialParam)
	require.NoError(t, err)

	newKey := "new-key"
	newValue := "new-value"
	newEnabled := false
	newDescription := "updated description"
	newOrder := float32(2)

	req := connect.NewRequest(&apiv1.HttpSearchParamUpdateRequest{
		Items: []*apiv1.HttpSearchParamUpdate{
			{
				HttpSearchParamId: paramID.Bytes(),
				Key:               &newKey,
				Value:             &newValue,
				Enabled:           &newEnabled,
				Description:       &newDescription,
				Order:             &newOrder,
			},
		},
	})

	_, err = f.handler.HttpSearchParamUpdate(f.ctx, req)
	require.NoError(t, err)

	// Verify update
	param, err := f.handler.httpSearchParamService.GetByID(f.ctx, paramID)
	require.NoError(t, err)
	require.Equal(t, newKey, param.Key)
	require.Equal(t, newValue, param.Value)
	require.Equal(t, newEnabled, param.Enabled)
	require.Equal(t, newDescription, param.Description)
	require.Equal(t, float64(newOrder), param.DisplayOrder)
}

func TestHttpSearchParamDelete_Success(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttp(t, ws, "test-http")

	// Create param to delete
	paramID := idwrap.NewNow()
	param := &mhttp.HTTPSearchParam{
		ID:      paramID,
		HttpID:  httpID,
		Key:     "to-delete",
		Value:   "value",
		Enabled: true,
	}
	err := f.handler.httpSearchParamService.Create(f.ctx, param)
	require.NoError(t, err)

	req := connect.NewRequest(&apiv1.HttpSearchParamDeleteRequest{
		Items: []*apiv1.HttpSearchParamDelete{
			{
				HttpSearchParamId: paramID.Bytes(),
			},
		},
	})

	_, err = f.handler.HttpSearchParamDelete(f.ctx, req)
	require.NoError(t, err)

	// Verify deleted
	_, err = f.handler.httpSearchParamService.GetByID(f.ctx, paramID)
	require.Error(t, err)
}

func TestHttpSearchParamCollection_Success(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID1 := f.createHttp(t, ws, "http-1")
	httpID2 := f.createHttp(t, ws, "http-2")

	// Create params for http1
	f.createHttpSearchParam(t, httpID1, "p1", "v1")
	f.createHttpSearchParam(t, httpID1, "p2", "v2")

	// Create params for http2
	f.createHttpSearchParam(t, httpID2, "p3", "v3")

	req := connect.NewRequest(&emptypb.Empty{})
	resp, err := f.handler.HttpSearchParamCollection(f.ctx, req)
	require.NoError(t, err)

	// We expect 3 params total
	require.Len(t, resp.Msg.Items, 3)

	// Verify content
	var keys []string
	for _, item := range resp.Msg.Items {
		keys = append(keys, item.Key)
	}
	require.Contains(t, keys, "p1")
	require.Contains(t, keys, "p2")
	require.Contains(t, keys, "p3")
}
