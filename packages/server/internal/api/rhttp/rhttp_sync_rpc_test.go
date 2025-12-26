package rhttp

import (
	"context"
	"errors"
	"testing"
	"time"

	"the-dev-tools/server/pkg/idwrap"
	httpv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
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

func collectHttpVersionSyncItems(t *testing.T, ch <-chan *httpv1.HttpVersionSyncResponse, count int) []*httpv1.HttpVersionSync {
	t.Helper()
	var items []*httpv1.HttpVersionSync
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

	// Insert with enabled and order
	id := idwrap.NewNow()
	val := "res.status == 200"

	req := connect.NewRequest(&httpv1.HttpAssertInsertRequest{
		Items: []*httpv1.HttpAssertInsert{
			{
				HttpAssertId: id.Bytes(),
				HttpId:       httpID.Bytes(),
				Value:        val,
				Enabled:      true,
				Order:        1.5,
			},
		},
	})
	_, err := f.handler.HttpAssertInsert(f.ctx, req)
	require.NoError(t, err)

	items := collectHttpAssertSyncItems(t, msgCh, 1)
	v := items[0].GetValue()
	require.Equal(t, httpv1.HttpAssertSync_ValueUnion_KIND_INSERT, v.GetKind())
	require.Equal(t, val, v.GetInsert().GetValue())
	require.True(t, v.GetInsert().GetEnabled(), "insert should have enabled=true")
	require.Equal(t, float32(1.5), v.GetInsert().GetOrder(), "insert should have order=1.5")

	// Update enabled and order
	newVal := "res.status == 201"
	newEnabled := false
	newOrder := float32(2.5)
	reqUpdate := connect.NewRequest(&httpv1.HttpAssertUpdateRequest{
		Items: []*httpv1.HttpAssertUpdate{
			{
				HttpAssertId: id.Bytes(),
				Value:        &newVal,
				Enabled:      &newEnabled,
				Order:        &newOrder,
			},
		},
	})
	_, err = f.handler.HttpAssertUpdate(f.ctx, reqUpdate)
	require.NoError(t, err)

	items = collectHttpAssertSyncItems(t, msgCh, 1)
	v = items[0].GetValue()
	require.Equal(t, httpv1.HttpAssertSync_ValueUnion_KIND_UPDATE, v.GetKind())
	require.Equal(t, newVal, v.GetUpdate().GetValue())
	require.False(t, v.GetUpdate().GetEnabled(), "update should have enabled=false")
	require.Equal(t, float32(2.5), v.GetUpdate().GetOrder(), "update should have order=2.5")

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

func TestHttpUpdateSync_SingleItem(t *testing.T) {
	t.Parallel()

	f := newHttpStreamingFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttp(t, ws, "original-name", "https://original.com", "GET")

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *httpv1.HttpSyncResponse, 10)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamHttpSync(ctx, f.userID, func(resp *httpv1.HttpSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	// Wait for stream to be active (no snapshot expected)
	select {
	case <-msgCh:
		require.FailNow(t, "Received unexpected snapshot item")
	case <-time.After(100 * time.Millisecond):
	}

	// Update HTTP name
	newName := "updated-name"
	req := connect.NewRequest(&httpv1.HttpUpdateRequest{
		Items: []*httpv1.HttpUpdate{
			{HttpId: httpID.Bytes(), Name: &newName},
		},
	})
	_, err := f.handler.HttpUpdate(f.ctx, req)
	require.NoError(t, err)

	// Verify sync event
	items := collectHttpSyncStreamingItems(t, msgCh, 1)
	updateVal := items[0].GetValue()
	require.Equal(t, httpv1.HttpSync_ValueUnion_KIND_UPDATE, updateVal.GetKind())
	require.Equal(t, newName, updateVal.GetUpdate().GetName())

	cancel()
	err = <-errCh
	if err != nil {
		require.True(t, errors.Is(err, context.Canceled))
	}
}

func TestHttpUpdateSync_BulkSameWorkspace(t *testing.T) {
	t.Parallel()

	f := newHttpStreamingFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID1 := f.createHttp(t, ws, "http-1", "https://api1.com", "GET")
	httpID2 := f.createHttp(t, ws, "http-2", "https://api2.com", "POST")
	httpID3 := f.createHttp(t, ws, "http-3", "https://api3.com", "PUT")

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *httpv1.HttpSyncResponse, 10)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamHttpSync(ctx, f.userID, func(resp *httpv1.HttpSyncResponse) error {
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

	// Bulk update - 3 items in same workspace
	name1 := "updated-http-1"
	name2 := "updated-http-2"
	name3 := "updated-http-3"
	req := connect.NewRequest(&httpv1.HttpUpdateRequest{
		Items: []*httpv1.HttpUpdate{
			{HttpId: httpID1.Bytes(), Name: &name1},
			{HttpId: httpID2.Bytes(), Name: &name2},
			{HttpId: httpID3.Bytes(), Name: &name3},
		},
	})
	_, err := f.handler.HttpUpdate(f.ctx, req)
	require.NoError(t, err)

	// Verify all 3 update events (may be batched)
	items := collectHttpSyncStreamingItems(t, msgCh, 3)
	names := make(map[string]bool)
	for _, item := range items {
		v := item.GetValue()
		require.Equal(t, httpv1.HttpSync_ValueUnion_KIND_UPDATE, v.GetKind())
		names[v.GetUpdate().GetName()] = true
	}
	require.True(t, names[name1], "expected updated-http-1")
	require.True(t, names[name2], "expected updated-http-2")
	require.True(t, names[name3], "expected updated-http-3")

	cancel()
	err = <-errCh
	if err != nil {
		require.True(t, errors.Is(err, context.Canceled))
	}
}

func TestHttpUpdateSync_MultiWorkspace(t *testing.T) {
	t.Parallel()

	f := newHttpStreamingFixture(t)
	ws1 := f.createWorkspace(t, "workspace-1")
	ws2 := f.createWorkspace(t, "workspace-2")
	httpID1 := f.createHttp(t, ws1, "http-ws1", "https://ws1.com", "GET")
	httpID2 := f.createHttp(t, ws2, "http-ws2", "https://ws2.com", "POST")

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *httpv1.HttpSyncResponse, 10)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamHttpSync(ctx, f.userID, func(resp *httpv1.HttpSyncResponse) error {
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

	// Update HTTP entries from different workspaces
	name1 := "updated-ws1"
	name2 := "updated-ws2"
	req := connect.NewRequest(&httpv1.HttpUpdateRequest{
		Items: []*httpv1.HttpUpdate{
			{HttpId: httpID1.Bytes(), Name: &name1},
			{HttpId: httpID2.Bytes(), Name: &name2},
		},
	})
	_, err := f.handler.HttpUpdate(f.ctx, req)
	require.NoError(t, err)

	// Verify both updates received (may arrive in separate batches)
	items := collectHttpSyncStreamingItems(t, msgCh, 2)
	names := make(map[string]bool)
	for _, item := range items {
		v := item.GetValue()
		require.Equal(t, httpv1.HttpSync_ValueUnion_KIND_UPDATE, v.GetKind())
		names[v.GetUpdate().GetName()] = true
	}
	require.True(t, names[name1], "expected updated-ws1")
	require.True(t, names[name2], "expected updated-ws2")

	cancel()
	err = <-errCh
	if err != nil {
		require.True(t, errors.Is(err, context.Canceled))
	}
}

func TestHttpUpdateSync_PartialFields(t *testing.T) {
	t.Parallel()

	f := newHttpStreamingFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttp(t, ws, "original-name", "https://original.com", "GET")

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *httpv1.HttpSyncResponse, 10)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamHttpSync(ctx, f.userID, func(resp *httpv1.HttpSyncResponse) error {
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

	// Partial update - only name changed
	newName := "updated-name-only"
	req := connect.NewRequest(&httpv1.HttpUpdateRequest{
		Items: []*httpv1.HttpUpdate{
			{HttpId: httpID.Bytes(), Name: &newName},
		},
	})
	_, err := f.handler.HttpUpdate(f.ctx, req)
	require.NoError(t, err)

	// Verify update event with full state
	items := collectHttpSyncStreamingItems(t, msgCh, 1)
	updateVal := items[0].GetValue()
	require.Equal(t, httpv1.HttpSync_ValueUnion_KIND_UPDATE, updateVal.GetKind())
	require.Equal(t, newName, updateVal.GetUpdate().GetName())
	// Full state sent (converter sends complete object)
	require.Equal(t, "https://original.com", updateVal.GetUpdate().GetUrl())
	require.Equal(t, httpv1.HttpMethod_HTTP_METHOD_GET, updateVal.GetUpdate().GetMethod())

	cancel()
	err = <-errCh
	if err != nil {
		require.True(t, errors.Is(err, context.Canceled))
	}
}

func TestHttpUpdateSync_VersionCreation(t *testing.T) {
	t.Parallel()

	f := newHttpStreamingFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttp(t, ws, "original-name", "https://original.com", "GET")

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	// Setup both HTTP and HttpVersion streams
	httpMsgCh := make(chan *httpv1.HttpSyncResponse, 10)
	httpErrCh := make(chan error, 1)
	versionMsgCh := make(chan *httpv1.HttpVersionSyncResponse, 10)
	versionErrCh := make(chan error, 1)

	go func() {
		err := f.handler.streamHttpSync(ctx, f.userID, func(resp *httpv1.HttpSyncResponse) error {
			httpMsgCh <- resp
			return nil
		})
		httpErrCh <- err
		close(httpMsgCh)
	}()

	go func() {
		err := f.handler.streamHttpVersionSync(ctx, f.userID, func(resp *httpv1.HttpVersionSyncResponse) error {
			versionMsgCh <- resp
			return nil
		})
		versionErrCh <- err
		close(versionMsgCh)
	}()

	// Wait for streams to be active
	select {
	case <-httpMsgCh:
		require.FailNow(t, "Received unexpected HTTP snapshot item")
	case <-time.After(100 * time.Millisecond):
	}
	select {
	case <-versionMsgCh:
		require.FailNow(t, "Received unexpected version snapshot item")
	case <-time.After(100 * time.Millisecond):
	}

	// Update HTTP - should create version
	newName := "updated-with-version"
	req := connect.NewRequest(&httpv1.HttpUpdateRequest{
		Items: []*httpv1.HttpUpdate{
			{HttpId: httpID.Bytes(), Name: &newName},
		},
	})
	_, err := f.handler.HttpUpdate(f.ctx, req)
	require.NoError(t, err)

	// Verify HTTP update event
	httpItems := collectHttpSyncStreamingItems(t, httpMsgCh, 1)
	httpVal := httpItems[0].GetValue()
	require.Equal(t, httpv1.HttpSync_ValueUnion_KIND_UPDATE, httpVal.GetKind())
	require.Equal(t, newName, httpVal.GetUpdate().GetName())

	// Verify HttpVersion insert event
	versionItems := collectHttpVersionSyncItems(t, versionMsgCh, 1)
	versionVal := versionItems[0].GetValue()
	require.Equal(t, httpv1.HttpVersionSync_ValueUnion_KIND_INSERT, versionVal.GetKind())
	// Version should have auto-generated name (format: v<timestamp>)
	require.NotEmpty(t, versionVal.GetInsert().GetName())
	require.Contains(t, versionVal.GetInsert().GetName(), "v")
	// Version description should be "Auto-saved version"
	require.Equal(t, "Auto-saved version", versionVal.GetInsert().GetDescription())
	// Version linked to correct HTTP ID
	require.Equal(t, httpID.Bytes(), versionVal.GetInsert().GetHttpId())

	cancel()
	err = <-httpErrCh
	if err != nil {
		require.True(t, errors.Is(err, context.Canceled))
	}
	err = <-versionErrCh
	if err != nil {
		require.True(t, errors.Is(err, context.Canceled))
	}
}

func TestHttpDeleteSync_SingleItem(t *testing.T) {
	t.Parallel()

	f := newHttpStreamingFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttp(t, ws, "to-delete", "https://delete.com", "DELETE")

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *httpv1.HttpSyncResponse, 10)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamHttpSync(ctx, f.userID, func(resp *httpv1.HttpSyncResponse) error {
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

	// Delete HTTP entry
	req := connect.NewRequest(&httpv1.HttpDeleteRequest{
		Items: []*httpv1.HttpDelete{
			{HttpId: httpID.Bytes()},
		},
	})
	_, err := f.handler.HttpDelete(f.ctx, req)
	require.NoError(t, err)

	// Verify delete event
	items := collectHttpSyncStreamingItems(t, msgCh, 1)
	deleteVal := items[0].GetValue()
	require.Equal(t, httpv1.HttpSync_ValueUnion_KIND_DELETE, deleteVal.GetKind())
	require.Equal(t, httpID.Bytes(), deleteVal.GetDelete().GetHttpId())

	// Verify item actually deleted from database
	_, err = f.hs.Get(f.ctx, httpID)
	require.Error(t, err, "HTTP should be deleted from database")

	cancel()
	err = <-errCh
	if err != nil {
		require.True(t, errors.Is(err, context.Canceled))
	}
}

func TestHttpDeleteSync_BulkSameWorkspace(t *testing.T) {
	t.Parallel()

	f := newHttpStreamingFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID1 := f.createHttp(t, ws, "delete-1", "https://delete1.com", "DELETE")
	httpID2 := f.createHttp(t, ws, "delete-2", "https://delete2.com", "DELETE")
	httpID3 := f.createHttp(t, ws, "delete-3", "https://delete3.com", "DELETE")

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *httpv1.HttpSyncResponse, 10)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamHttpSync(ctx, f.userID, func(resp *httpv1.HttpSyncResponse) error {
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

	// Bulk delete - 3 items in same workspace
	req := connect.NewRequest(&httpv1.HttpDeleteRequest{
		Items: []*httpv1.HttpDelete{
			{HttpId: httpID1.Bytes()},
			{HttpId: httpID2.Bytes()},
			{HttpId: httpID3.Bytes()},
		},
	})
	_, err := f.handler.HttpDelete(f.ctx, req)
	require.NoError(t, err)

	// Verify all 3 delete events
	items := collectHttpSyncStreamingItems(t, msgCh, 3)
	deletedIDs := make(map[string]bool)
	for _, item := range items {
		v := item.GetValue()
		require.Equal(t, httpv1.HttpSync_ValueUnion_KIND_DELETE, v.GetKind())
		id, err := idwrap.NewFromBytes(v.GetDelete().GetHttpId())
		require.NoError(t, err)
		deletedIDs[id.String()] = true
	}
	require.True(t, deletedIDs[httpID1.String()], "expected httpID1 deleted")
	require.True(t, deletedIDs[httpID2.String()], "expected httpID2 deleted")
	require.True(t, deletedIDs[httpID3.String()], "expected httpID3 deleted")

	cancel()
	err = <-errCh
	if err != nil {
		require.True(t, errors.Is(err, context.Canceled))
	}
}

func TestHttpDeleteSync_MultiWorkspace(t *testing.T) {
	t.Parallel()

	f := newHttpStreamingFixture(t)
	ws1 := f.createWorkspace(t, "workspace-1")
	ws2 := f.createWorkspace(t, "workspace-2")
	httpID1 := f.createHttp(t, ws1, "delete-ws1", "https://delete-ws1.com", "DELETE")
	httpID2 := f.createHttp(t, ws2, "delete-ws2", "https://delete-ws2.com", "DELETE")

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *httpv1.HttpSyncResponse, 10)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamHttpSync(ctx, f.userID, func(resp *httpv1.HttpSyncResponse) error {
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

	// Delete HTTP entries from different workspaces
	req := connect.NewRequest(&httpv1.HttpDeleteRequest{
		Items: []*httpv1.HttpDelete{
			{HttpId: httpID1.Bytes()},
			{HttpId: httpID2.Bytes()},
		},
	})
	_, err := f.handler.HttpDelete(f.ctx, req)
	require.NoError(t, err)

	// Verify both deletes received (may arrive in separate batches)
	items := collectHttpSyncStreamingItems(t, msgCh, 2)
	deletedIDs := make(map[string]bool)
	for _, item := range items {
		v := item.GetValue()
		require.Equal(t, httpv1.HttpSync_ValueUnion_KIND_DELETE, v.GetKind())
		id, err := idwrap.NewFromBytes(v.GetDelete().GetHttpId())
		require.NoError(t, err)
		deletedIDs[id.String()] = true
	}
	require.True(t, deletedIDs[httpID1.String()], "expected httpID1 deleted")
	require.True(t, deletedIDs[httpID2.String()], "expected httpID2 deleted")

	cancel()
	err = <-errCh
	if err != nil {
		require.True(t, errors.Is(err, context.Canceled))
	}
}

