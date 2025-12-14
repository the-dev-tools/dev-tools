package rhttp

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"the-dev-tools/server/pkg/idwrap"
	httpv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
)

// ========== HELPERS ==========

func collectHttpSearchParamSyncItems(t *testing.T, ch <-chan *httpv1.HttpSearchParamSyncResponse, count int) []*httpv1.HttpSearchParamSync {
	t.Helper()
	var items []*httpv1.HttpSearchParamSync
	timeout := time.After(2 * time.Second)
	for len(items) < count {
		select {
		case resp, ok := <-ch:
			require.True(t, ok, "channel closed")
			for _, item := range resp.GetItems() {
				if item != nil {
					items = append(items, item)
					if len(items) == count {
						break
					}
				}
			}
		case <-timeout:
			require.FailNow(t, "timeout waiting for items")
		}
	}
	return items
}

func collectHttpHeaderSyncItems(t *testing.T, ch <-chan *httpv1.HttpHeaderSyncResponse, count int) []*httpv1.HttpHeaderSync {
	t.Helper()
	var items []*httpv1.HttpHeaderSync
	timeout := time.After(2 * time.Second)
	for len(items) < count {
		select {
		case resp, ok := <-ch:
			require.True(t, ok, "channel closed")
			for _, item := range resp.GetItems() {
				if item != nil {
					items = append(items, item)
					if len(items) == count {
						break
					}
				}
			}
		case <-timeout:
			require.FailNow(t, "timeout waiting for items")
		}
	}
	return items
}

func collectHttpBodyFormDataSyncItems(t *testing.T, ch <-chan *httpv1.HttpBodyFormDataSyncResponse, count int) []*httpv1.HttpBodyFormDataSync {
	t.Helper()
	var items []*httpv1.HttpBodyFormDataSync
	timeout := time.After(2 * time.Second)
	for len(items) < count {
		select {
		case resp, ok := <-ch:
			require.True(t, ok, "channel closed")
			for _, item := range resp.GetItems() {
				if item != nil {
					items = append(items, item)
					if len(items) == count {
						break
					}
				}
			}
		case <-timeout:
			require.FailNow(t, "timeout waiting for items")
		}
	}
	return items
}

func collectHttpBodyUrlEncodedSyncItems(t *testing.T, ch <-chan *httpv1.HttpBodyUrlEncodedSyncResponse, count int) []*httpv1.HttpBodyUrlEncodedSync {
	t.Helper()
	var items []*httpv1.HttpBodyUrlEncodedSync
	timeout := time.After(2 * time.Second)
	for len(items) < count {
		select {
		case resp, ok := <-ch:
			require.True(t, ok, "channel closed")
			for _, item := range resp.GetItems() {
				if item != nil {
					items = append(items, item)
					if len(items) == count {
						break
					}
				}
			}
		case <-timeout:
			require.FailNow(t, "timeout waiting for items")
		}
	}
	return items
}

func collectHttpBodyRawSyncItems(t *testing.T, ch <-chan *httpv1.HttpBodyRawSyncResponse, count int) []*httpv1.HttpBodyRawSync {
	t.Helper()
	var items []*httpv1.HttpBodyRawSync
	timeout := time.After(2 * time.Second)
	for len(items) < count {
		select {
		case resp, ok := <-ch:
			require.True(t, ok, "channel closed")
			for _, item := range resp.GetItems() {
				if item != nil {
					items = append(items, item)
					if len(items) == count {
						break
					}
				}
			}
		case <-timeout:
			require.FailNow(t, "timeout waiting for items")
		}
	}
	return items
}

func collectHttpAssertSyncItems(t *testing.T, ch <-chan *httpv1.HttpAssertSyncResponse, count int) []*httpv1.HttpAssertSync {
	t.Helper()
	var items []*httpv1.HttpAssertSync
	timeout := time.After(2 * time.Second)
	for len(items) < count {
		select {
		case resp, ok := <-ch:
			require.True(t, ok, "channel closed")
			for _, item := range resp.GetItems() {
				if item != nil {
					items = append(items, item)
					if len(items) == count {
						break
					}
				}
			}
		case <-timeout:
			require.FailNow(t, "timeout waiting for items")
		}
	}
	return items
}

// ========== TESTS ==========

func TestHttpSearchParamSync_Streaming(t *testing.T) {
	t.Parallel()

	f := newHttpStreamingFixture(t)
	ws := f.createWorkspace(t, "ws")
	httpID := f.createHttp(t, ws, "http", "url", "GET")

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *httpv1.HttpSearchParamSyncResponse, 10)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamHttpSearchParamSync(ctx, f.userID, func(resp *httpv1.HttpSearchParamSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	// No initial items expected
	select {
	case <-msgCh:
		require.FailNow(t, "Received unexpected snapshot item")
	case <-time.After(100 * time.Millisecond):
	}

	// Insert
	paramID := idwrap.NewNow()
	key := "foo"
	val := "bar"
	enabled := true
	order := float32(1.0)

	req := connect.NewRequest(&httpv1.HttpSearchParamInsertRequest{
		Items: []*httpv1.HttpSearchParamInsert{
			{
				HttpSearchParamId: paramID.Bytes(),
				HttpId:            httpID.Bytes(),
				Key:               key,
				Value:             val,
				Enabled:           enabled,
				Order:             order,
			},
		},
	})
	_, err := f.handler.HttpSearchParamInsert(f.ctx, req)
	require.NoError(t, err)

	// Verify insert event
	items := collectHttpSearchParamSyncItems(t, msgCh, 1)
	v := items[0].GetValue()
	require.Equal(t, httpv1.HttpSearchParamSync_ValueUnion_KIND_INSERT, v.GetKind())
	require.Equal(t, key, v.GetInsert().GetKey())

	// Update
	newKey := "foo2"
	reqUpdate := connect.NewRequest(&httpv1.HttpSearchParamUpdateRequest{
		Items: []*httpv1.HttpSearchParamUpdate{
			{
				HttpSearchParamId: paramID.Bytes(),
				Key:               &newKey,
			},
		},
	})
	_, err = f.handler.HttpSearchParamUpdate(f.ctx, reqUpdate)
	require.NoError(t, err)

	// Verify update event
	items = collectHttpSearchParamSyncItems(t, msgCh, 1)
	v = items[0].GetValue()
	require.Equal(t, httpv1.HttpSearchParamSync_ValueUnion_KIND_UPDATE, v.GetKind())
	require.Equal(t, newKey, v.GetUpdate().GetKey())

	// Delete
	reqDelete := connect.NewRequest(&httpv1.HttpSearchParamDeleteRequest{
		Items: []*httpv1.HttpSearchParamDelete{
			{
				HttpSearchParamId: paramID.Bytes(),
			},
		},
	})
	_, err = f.handler.HttpSearchParamDelete(f.ctx, reqDelete)
	require.NoError(t, err)

	// Verify delete event
	items = collectHttpSearchParamSyncItems(t, msgCh, 1)
	v = items[0].GetValue()
	require.Equal(t, httpv1.HttpSearchParamSync_ValueUnion_KIND_DELETE, v.GetKind())

	cancel()
	err = <-errCh
	if err != nil {
		require.True(t, errors.Is(err, context.Canceled))
	}
}

func TestHttpHeaderSync_Streaming(t *testing.T) {
	t.Parallel()

	f := newHttpStreamingFixture(t)
	ws := f.createWorkspace(t, "ws")
	httpID := f.createHttp(t, ws, "http", "url", "GET")

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *httpv1.HttpHeaderSyncResponse, 10)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamHttpHeaderSync(ctx, f.userID, func(resp *httpv1.HttpHeaderSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	select {
	case <-msgCh:
		require.FailNow(t, "Received unexpected snapshot item")
	case <-time.After(100 * time.Millisecond):
	}

	// Insert
	headerID := idwrap.NewNow()
	key := "Content-Type"
	val := "application/json"
	enabled := true
	order := float32(1.0)

	req := connect.NewRequest(&httpv1.HttpHeaderInsertRequest{
		Items: []*httpv1.HttpHeaderInsert{
			{
				HttpHeaderId: headerID.Bytes(),
				HttpId:       httpID.Bytes(),
				Key:          key,
				Value:        val,
				Enabled:      enabled,
				Order:        order,
			},
		},
	})
	_, err := f.handler.HttpHeaderInsert(f.ctx, req)
	require.NoError(t, err)

	// Verify insert event
	items := collectHttpHeaderSyncItems(t, msgCh, 1)
	v := items[0].GetValue()
	require.Equal(t, httpv1.HttpHeaderSync_ValueUnion_KIND_INSERT, v.GetKind())
	require.Equal(t, key, v.GetInsert().GetKey())

	// Update
	newVal := "text/plain"
	reqUpdate := connect.NewRequest(&httpv1.HttpHeaderUpdateRequest{
		Items: []*httpv1.HttpHeaderUpdate{
			{
				HttpHeaderId: headerID.Bytes(),
				Value:        &newVal,
			},
		},
	})
	_, err = f.handler.HttpHeaderUpdate(f.ctx, reqUpdate)
	require.NoError(t, err)

	items = collectHttpHeaderSyncItems(t, msgCh, 1)
	v = items[0].GetValue()
	require.Equal(t, httpv1.HttpHeaderSync_ValueUnion_KIND_UPDATE, v.GetKind())
	require.Equal(t, newVal, v.GetUpdate().GetValue())

	// Delete
	reqDelete := connect.NewRequest(&httpv1.HttpHeaderDeleteRequest{
		Items: []*httpv1.HttpHeaderDelete{
			{
				HttpHeaderId: headerID.Bytes(),
			},
		},
	})
	_, err = f.handler.HttpHeaderDelete(f.ctx, reqDelete)
	require.NoError(t, err)

	items = collectHttpHeaderSyncItems(t, msgCh, 1)
	v = items[0].GetValue()
	require.Equal(t, httpv1.HttpHeaderSync_ValueUnion_KIND_DELETE, v.GetKind())

	cancel()
	err = <-errCh
	if err != nil {
		require.True(t, errors.Is(err, context.Canceled))
	}
}

func TestHttpBodyFormDataSync_Streaming(t *testing.T) {
	t.Parallel()

	f := newHttpStreamingFixture(t)
	ws := f.createWorkspace(t, "ws")
	httpID := f.createHttp(t, ws, "http", "url", "GET")

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *httpv1.HttpBodyFormDataSyncResponse, 10)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamHttpBodyFormSync(ctx, f.userID, func(resp *httpv1.HttpBodyFormDataSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	select {
	case <-msgCh:
		require.FailNow(t, "Received unexpected snapshot item")
	case <-time.After(100 * time.Millisecond):
	}

	// Insert
	id := idwrap.NewNow()
	key := "file"
	val := "data"
	enabled := true
	order := float32(1.0)

	req := connect.NewRequest(&httpv1.HttpBodyFormDataInsertRequest{
		Items: []*httpv1.HttpBodyFormDataInsert{
			{
				HttpBodyFormDataId: id.Bytes(),
				HttpId:             httpID.Bytes(),
				Key:                key,
				Value:              val,
				Enabled:            enabled,
				Order:              order,
			},
		},
	})
	_, err := f.handler.HttpBodyFormDataInsert(f.ctx, req)
	require.NoError(t, err)

	items := collectHttpBodyFormDataSyncItems(t, msgCh, 1)
	v := items[0].GetValue()
	require.Equal(t, httpv1.HttpBodyFormDataSync_ValueUnion_KIND_INSERT, v.GetKind())
	require.Equal(t, key, v.GetInsert().GetKey())

	// Update
	newKey := "file2"
	reqUpdate := connect.NewRequest(&httpv1.HttpBodyFormDataUpdateRequest{
		Items: []*httpv1.HttpBodyFormDataUpdate{
			{
				HttpBodyFormDataId: id.Bytes(),
				Key:                &newKey,
			},
		},
	})
	_, err = f.handler.HttpBodyFormDataUpdate(f.ctx, reqUpdate)
	require.NoError(t, err)

	items = collectHttpBodyFormDataSyncItems(t, msgCh, 1)
	v = items[0].GetValue()
	require.Equal(t, httpv1.HttpBodyFormDataSync_ValueUnion_KIND_UPDATE, v.GetKind())
	require.Equal(t, newKey, v.GetUpdate().GetKey())

	// Delete
	reqDelete := connect.NewRequest(&httpv1.HttpBodyFormDataDeleteRequest{
		Items: []*httpv1.HttpBodyFormDataDelete{
			{
				HttpBodyFormDataId: id.Bytes(),
			},
		},
	})
	_, err = f.handler.HttpBodyFormDataDelete(f.ctx, reqDelete)
	require.NoError(t, err)

	items = collectHttpBodyFormDataSyncItems(t, msgCh, 1)
	v = items[0].GetValue()
	require.Equal(t, httpv1.HttpBodyFormDataSync_ValueUnion_KIND_DELETE, v.GetKind())

	cancel()
	err = <-errCh
	if err != nil {
		require.True(t, errors.Is(err, context.Canceled))
	}
}

func TestHttpBodyUrlEncodedSync_Streaming(t *testing.T) {
	t.Parallel()

	f := newHttpStreamingFixture(t)
	ws := f.createWorkspace(t, "ws")
	httpID := f.createHttp(t, ws, "http", "url", "GET")

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *httpv1.HttpBodyUrlEncodedSyncResponse, 10)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamHttpBodyUrlEncodedSync(ctx, f.userID, func(resp *httpv1.HttpBodyUrlEncodedSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	select {
	case <-msgCh:
		require.FailNow(t, "Received unexpected snapshot item")
	case <-time.After(100 * time.Millisecond):
	}

	// Insert
	id := idwrap.NewNow()
	key := "foo"
	val := "bar"
	enabled := true
	order := float32(1.0)

	req := connect.NewRequest(&httpv1.HttpBodyUrlEncodedInsertRequest{
		Items: []*httpv1.HttpBodyUrlEncodedInsert{
			{
				HttpBodyUrlEncodedId: id.Bytes(),
				HttpId:               httpID.Bytes(),
				Key:                  key,
				Value:                val,
				Enabled:              enabled,
				Order:                order,
			},
		},
	})
	_, err := f.handler.HttpBodyUrlEncodedInsert(f.ctx, req)
	require.NoError(t, err)

	items := collectHttpBodyUrlEncodedSyncItems(t, msgCh, 1)
	v := items[0].GetValue()
	require.Equal(t, httpv1.HttpBodyUrlEncodedSync_ValueUnion_KIND_INSERT, v.GetKind())
	require.Equal(t, key, v.GetInsert().GetKey())

	// Update
	newVal := "baz"
	reqUpdate := connect.NewRequest(&httpv1.HttpBodyUrlEncodedUpdateRequest{
		Items: []*httpv1.HttpBodyUrlEncodedUpdate{
			{
				HttpBodyUrlEncodedId: id.Bytes(),
				Value:                &newVal,
			},
		},
	})
	_, err = f.handler.HttpBodyUrlEncodedUpdate(f.ctx, reqUpdate)
	require.NoError(t, err)

	items = collectHttpBodyUrlEncodedSyncItems(t, msgCh, 1)
	v = items[0].GetValue()
	require.Equal(t, httpv1.HttpBodyUrlEncodedSync_ValueUnion_KIND_UPDATE, v.GetKind())
	require.Equal(t, newVal, v.GetUpdate().GetValue())

	// Delete
	reqDelete := connect.NewRequest(&httpv1.HttpBodyUrlEncodedDeleteRequest{
		Items: []*httpv1.HttpBodyUrlEncodedDelete{
			{
				HttpBodyUrlEncodedId: id.Bytes(),
			},
		},
	})
	_, err = f.handler.HttpBodyUrlEncodedDelete(f.ctx, reqDelete)
	require.NoError(t, err)

	items = collectHttpBodyUrlEncodedSyncItems(t, msgCh, 1)
	v = items[0].GetValue()
	require.Equal(t, httpv1.HttpBodyUrlEncodedSync_ValueUnion_KIND_DELETE, v.GetKind())

	cancel()
	err = <-errCh
	if err != nil {
		require.True(t, errors.Is(err, context.Canceled))
	}
}

func TestHttpBodyRawSync_Streaming(t *testing.T) {
	t.Parallel()

	f := newHttpStreamingFixture(t)
	ws := f.createWorkspace(t, "ws")
	httpID := f.createHttp(t, ws, "http", "url", "GET")

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *httpv1.HttpBodyRawSyncResponse, 10)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamHttpBodyRawSync(ctx, f.userID, func(resp *httpv1.HttpBodyRawSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	select {
	case <-msgCh:
		require.FailNow(t, "Received unexpected snapshot item")
	case <-time.After(100 * time.Millisecond):
	}

	// Manually publish event since CRUD doesn't seem to publish it (or we want to test streaming regardless of CRUD implementation status)
	f.handler.streamers.HttpBodyRaw.Publish(HttpBodyRawTopic{WorkspaceID: ws}, HttpBodyRawEvent{
		Type:    eventTypeInsert,
		IsDelta: false,
		HttpBodyRaw: &httpv1.HttpBodyRaw{
			HttpId: httpID.Bytes(),
			Data:   "raw data",
		},
	})

	items := collectHttpBodyRawSyncItems(t, msgCh, 1)
	v := items[0].GetValue()
	require.Equal(t, httpv1.HttpBodyRawSync_ValueUnion_KIND_INSERT, v.GetKind())
	require.Equal(t, "raw data", v.GetInsert().GetData())

	cancel()
	err := <-errCh
	if err != nil {
		require.True(t, errors.Is(err, context.Canceled))
	}
}

func TestHttpAssertSync_Streaming(t *testing.T) {
	t.Parallel()

	f := newHttpStreamingFixture(t)
	ws := f.createWorkspace(t, "ws")
	httpID := f.createHttp(t, ws, "http", "url", "GET")

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *httpv1.HttpAssertSyncResponse, 10)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamHttpAssertSync(ctx, f.userID, func(resp *httpv1.HttpAssertSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	select {
	case <-msgCh:
		require.FailNow(t, "Received unexpected snapshot item")
	case <-time.After(100 * time.Millisecond):
	}

	// Insert
	id := idwrap.NewNow()
	val := "res.status == 200"

	req := connect.NewRequest(&httpv1.HttpAssertInsertRequest{
		Items: []*httpv1.HttpAssertInsert{
			{
				HttpAssertId: id.Bytes(),
				HttpId:       httpID.Bytes(),
				Value:        val,
			},
		},
	})
	_, err := f.handler.HttpAssertInsert(f.ctx, req)
	require.NoError(t, err)

	items := collectHttpAssertSyncItems(t, msgCh, 1)
	v := items[0].GetValue()
	require.Equal(t, httpv1.HttpAssertSync_ValueUnion_KIND_INSERT, v.GetKind())
	require.Equal(t, val, v.GetInsert().GetValue())

	// Update
	newVal := "res.status == 201"
	reqUpdate := connect.NewRequest(&httpv1.HttpAssertUpdateRequest{
		Items: []*httpv1.HttpAssertUpdate{
			{
				HttpAssertId: id.Bytes(),
				Value:        &newVal,
			},
		},
	})
	_, err = f.handler.HttpAssertUpdate(f.ctx, reqUpdate)
	require.NoError(t, err)

	items = collectHttpAssertSyncItems(t, msgCh, 1)
	v = items[0].GetValue()
	require.Equal(t, httpv1.HttpAssertSync_ValueUnion_KIND_UPDATE, v.GetKind())
	require.Equal(t, newVal, v.GetUpdate().GetValue())

	// Delete
	reqDelete := connect.NewRequest(&httpv1.HttpAssertDeleteRequest{
		Items: []*httpv1.HttpAssertDelete{
			{
				HttpAssertId: id.Bytes(),
			},
		},
	})
	_, err = f.handler.HttpAssertDelete(f.ctx, reqDelete)
	require.NoError(t, err)

	items = collectHttpAssertSyncItems(t, msgCh, 1)
	v = items[0].GetValue()
	require.Equal(t, httpv1.HttpAssertSync_ValueUnion_KIND_DELETE, v.GetKind())

	cancel()
	err = <-errCh
	if err != nil {
		require.True(t, errors.Is(err, context.Canceled))
	}
}
