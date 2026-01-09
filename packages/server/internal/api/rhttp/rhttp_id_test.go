package rhttp

import (
	"context"
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	httpv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/http/v1"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestHttpHeaderInsertRespectsClientIDs(t *testing.T) {
	t.Parallel()

	f := newHttpStreamingFixture(t)
	wsID := f.createWorkspace(t, "header-test-workspace")
	httpID := f.createHttp(t, wsID, "header-test-http", "https://example.com", "GET")

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *httpv1.HttpHeaderSyncResponse, 10)
	errCh := make(chan error, 1)

	// Start streaming HttpHeaderSync to verify event payload
	go func() {
		err := f.handler.streamHttpHeaderSync(ctx, f.userID, func(resp *httpv1.HttpHeaderSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	// Generate a specific ID for the new header
	clientGeneratedID := idwrap.NewNow()
	headerKey := "X-Client-ID-Test"
	headerValue := "verified"

	// Create the header using the generated ID
	insertReq := connect.NewRequest(&httpv1.HttpHeaderInsertRequest{
		Items: []*httpv1.HttpHeaderInsert{
			{
				HttpHeaderId: clientGeneratedID.Bytes(),
				HttpId:       httpID.Bytes(),
				Key:          headerKey,
				Value:        headerValue,
				Enabled:      true,
				Description:  "Testing client ID respect",
				Order:        1.0,
			},
		},
	})

	_, err := f.handler.HttpHeaderInsert(f.ctx, insertReq)
	require.NoError(t, err)

	// Verify ID in DB
	headers, err := f.handler.httpHeaderService.GetByHttpID(f.ctx, httpID)
	require.NoError(t, err)

	foundInDB := false
	for _, h := range headers {
		if h.ID == clientGeneratedID {
			foundInDB = true
			require.Equal(t, headerKey, h.Key, "Header key mismatch in DB")
			break
		}
	}
	require.True(t, foundInDB, "Header with client ID %s not found in DB", clientGeneratedID.String())

	// Verify ID in Sync Event
	timeout := time.After(2 * time.Second)
	foundInStream := false

	// We might receive snapshot events first, so we loop until we find ours or timeout
	for {
		select {
		case resp, ok := <-msgCh:
			require.True(t, ok, "Stream closed before event received")
			for _, item := range resp.GetItems() {
				val := item.GetValue()
				if val == nil {
					continue
				}

				// We look for INSERT kind
				if val.GetKind() == httpv1.HttpHeaderSync_ValueUnion_KIND_INSERT {
					insert := val.GetInsert()
					eventID, _ := idwrap.NewFromBytes(insert.GetHttpHeaderId())

					if eventID == clientGeneratedID {
						foundInStream = true
						require.Equal(t, headerKey, insert.GetKey(), "Header key mismatch in stream")
						// Verify HttpId is populated
						require.NotEmpty(t, insert.GetHttpId(), "HttpId missing in sync event")
						goto Done
					}
				}
			}
		case <-timeout:
			require.FailNow(t, "Timeout waiting for sync event")
		}
	}
Done:
	require.True(t, foundInStream, "Header inserted event not received in stream")
}

func TestHttpHeaderUpdatePersistsOrder(t *testing.T) {
	t.Parallel()

	f := newHttpStreamingFixture(t)
	wsID := f.createWorkspace(t, "header-order-test")
	httpID := f.createHttp(t, wsID, "header-order-http", "https://example.com", "GET")

	// Insert a header with initial order
	headerID := idwrap.NewNow()
	initialOrder := float32(1.0)

	insertReq := connect.NewRequest(&httpv1.HttpHeaderInsertRequest{
		Items: []*httpv1.HttpHeaderInsert{
			{
				HttpHeaderId: headerID.Bytes(),
				HttpId:       httpID.Bytes(),
				Key:          "X-Test-Header",
				Value:        "test-value",
				Enabled:      true,
				Order:        initialOrder,
			},
		},
	})

	_, err := f.handler.HttpHeaderInsert(f.ctx, insertReq)
	require.NoError(t, err)

	// Verify initial order in DB
	headers, err := f.handler.httpHeaderService.GetByHttpID(f.ctx, httpID)
	require.NoError(t, err)
	require.Len(t, headers, 1)
	require.Equal(t, initialOrder, headers[0].DisplayOrder)

	// Update header with new order
	newOrder := float32(5.0)
	updateReq := connect.NewRequest(&httpv1.HttpHeaderUpdateRequest{
		Items: []*httpv1.HttpHeaderUpdate{
			{
				HttpHeaderId: headerID.Bytes(),
				Order:        &newOrder,
			},
		},
	})

	_, err = f.handler.HttpHeaderUpdate(f.ctx, updateReq)
	require.NoError(t, err)

	// Verify updated order in DB
	headers, err = f.handler.httpHeaderService.GetByHttpID(f.ctx, httpID)
	require.NoError(t, err)
	require.Len(t, headers, 1)
	require.Equal(t, newOrder, headers[0].DisplayOrder, "Order should be updated to new value")
}

func TestHttpBodyFormDataUpdatePersistsOrder(t *testing.T) {
	t.Parallel()

	f := newHttpStreamingFixture(t)
	wsID := f.createWorkspace(t, "body-form-order-test")
	httpID := f.createHttp(t, wsID, "body-form-order-http", "https://example.com", "POST")

	// Insert a body form entry with initial order
	formID := idwrap.NewNow()
	initialOrder := float32(1.0)

	insertReq := connect.NewRequest(&httpv1.HttpBodyFormDataInsertRequest{
		Items: []*httpv1.HttpBodyFormDataInsert{
			{
				HttpBodyFormDataId: formID.Bytes(),
				HttpId:             httpID.Bytes(),
				Key:                "test-key",
				Value:              "test-value",
				Enabled:            true,
				Order:              initialOrder,
			},
		},
	})

	_, err := f.handler.HttpBodyFormDataInsert(f.ctx, insertReq)
	require.NoError(t, err)

	// Verify initial order in DB
	forms, err := f.handler.httpBodyFormService.GetByHttpID(f.ctx, httpID)
	require.NoError(t, err)
	require.Len(t, forms, 1)
	require.Equal(t, initialOrder, forms[0].DisplayOrder)

	// Update body form with new order
	newOrder := float32(10.0)
	updateReq := connect.NewRequest(&httpv1.HttpBodyFormDataUpdateRequest{
		Items: []*httpv1.HttpBodyFormDataUpdate{
			{
				HttpBodyFormDataId: formID.Bytes(),
				Order:              &newOrder,
			},
		},
	})

	_, err = f.handler.HttpBodyFormDataUpdate(f.ctx, updateReq)
	require.NoError(t, err)

	// Verify updated order in DB
	forms, err = f.handler.httpBodyFormService.GetByHttpID(f.ctx, httpID)
	require.NoError(t, err)
	require.Len(t, forms, 1)
	require.Equal(t, newOrder, forms[0].DisplayOrder, "Order should be updated to new value")
}

func TestHttpBodyUrlEncodedUpdatePersistsOrder(t *testing.T) {
	t.Parallel()

	f := newHttpStreamingFixture(t)
	wsID := f.createWorkspace(t, "body-urlenc-order-test")
	httpID := f.createHttp(t, wsID, "body-urlenc-order-http", "https://example.com", "POST")

	// Insert a body urlencoded entry with initial order
	urlencID := idwrap.NewNow()
	initialOrder := float32(2.0)

	insertReq := connect.NewRequest(&httpv1.HttpBodyUrlEncodedInsertRequest{
		Items: []*httpv1.HttpBodyUrlEncodedInsert{
			{
				HttpBodyUrlEncodedId: urlencID.Bytes(),
				HttpId:               httpID.Bytes(),
				Key:                  "test-key",
				Value:                "test-value",
				Enabled:              true,
				Order:                initialOrder,
			},
		},
	})

	_, err := f.handler.HttpBodyUrlEncodedInsert(f.ctx, insertReq)
	require.NoError(t, err)

	// Verify initial order in DB
	urlencoded, err := f.handler.httpBodyUrlEncodedService.GetByHttpID(f.ctx, httpID)
	require.NoError(t, err)
	require.Len(t, urlencoded, 1)
	require.Equal(t, initialOrder, urlencoded[0].DisplayOrder)

	// Update body urlencoded with new order
	newOrder := float32(15.0)
	updateReq := connect.NewRequest(&httpv1.HttpBodyUrlEncodedUpdateRequest{
		Items: []*httpv1.HttpBodyUrlEncodedUpdate{
			{
				HttpBodyUrlEncodedId: urlencID.Bytes(),
				Order:                &newOrder,
			},
		},
	})

	_, err = f.handler.HttpBodyUrlEncodedUpdate(f.ctx, updateReq)
	require.NoError(t, err)

	// Verify updated order in DB
	urlencoded, err = f.handler.httpBodyUrlEncodedService.GetByHttpID(f.ctx, httpID)
	require.NoError(t, err)
	require.Len(t, urlencoded, 1)
	require.Equal(t, newOrder, urlencoded[0].DisplayOrder, "Order should be updated to new value")
}

func TestHttpHeaderOrderRoundTrip(t *testing.T) {
	t.Parallel()

	f := newHttpStreamingFixture(t)
	wsID := f.createWorkspace(t, "header-roundtrip-test")
	httpID := f.createHttp(t, wsID, "header-roundtrip-http", "https://example.com", "GET")

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	// Insert a header with specific order
	headerID := idwrap.NewNow()
	expectedOrder := float32(42.5)

	insertReq := connect.NewRequest(&httpv1.HttpHeaderInsertRequest{
		Items: []*httpv1.HttpHeaderInsert{
			{
				HttpHeaderId: headerID.Bytes(),
				HttpId:       httpID.Bytes(),
				Key:          "X-Order-Test",
				Value:        "test",
				Enabled:      true,
				Order:        expectedOrder,
			},
		},
	})

	_, err := f.handler.HttpHeaderInsert(f.ctx, insertReq)
	require.NoError(t, err)

	// Test 1: Verify order via Collection endpoint
	collectionResp, err := f.handler.HttpHeaderCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)

	var foundInCollection bool
	for _, item := range collectionResp.Msg.GetItems() {
		itemID, _ := idwrap.NewFromBytes(item.GetHttpHeaderId())
		if itemID == headerID {
			foundInCollection = true
			require.Equal(t, expectedOrder, item.GetOrder(), "Order should match in Collection response")
			break
		}
	}
	require.True(t, foundInCollection, "Header should be found in Collection response")

	// Test 2: Verify order via Sync stream
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

	// Update the header to trigger a sync event
	newOrder := float32(99.9)
	updateReq := connect.NewRequest(&httpv1.HttpHeaderUpdateRequest{
		Items: []*httpv1.HttpHeaderUpdate{
			{
				HttpHeaderId: headerID.Bytes(),
				Order:        &newOrder,
			},
		},
	})

	_, err = f.handler.HttpHeaderUpdate(f.ctx, updateReq)
	require.NoError(t, err)

	// Wait for UPDATE event in sync stream
	timeout := time.After(2 * time.Second)
	var foundInSync bool

	for !foundInSync {
		select {
		case resp, ok := <-msgCh:
			require.True(t, ok, "Stream closed before event received")
			for _, item := range resp.GetItems() {
				val := item.GetValue()
				if val == nil {
					continue
				}
				if val.GetKind() == httpv1.HttpHeaderSync_ValueUnion_KIND_UPDATE {
					update := val.GetUpdate()
					eventID, _ := idwrap.NewFromBytes(update.GetHttpHeaderId())
					if eventID == headerID {
						foundInSync = true
						require.Equal(t, newOrder, update.GetOrder(), "Order should match in Sync response")
						break
					}
				}
			}
		case <-timeout:
			require.FailNow(t, "Timeout waiting for sync event")
		}
	}
}

func TestHttpBodyFormDataOrderRoundTrip(t *testing.T) {
	t.Parallel()

	f := newHttpStreamingFixture(t)
	wsID := f.createWorkspace(t, "form-roundtrip-test")
	httpID := f.createHttp(t, wsID, "form-roundtrip-http", "https://example.com", "POST")

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	// Insert a form entry with specific order
	formID := idwrap.NewNow()
	expectedOrder := float32(33.3)

	insertReq := connect.NewRequest(&httpv1.HttpBodyFormDataInsertRequest{
		Items: []*httpv1.HttpBodyFormDataInsert{
			{
				HttpBodyFormDataId: formID.Bytes(),
				HttpId:             httpID.Bytes(),
				Key:                "order-test-key",
				Value:              "test",
				Enabled:            true,
				Order:              expectedOrder,
			},
		},
	})

	_, err := f.handler.HttpBodyFormDataInsert(f.ctx, insertReq)
	require.NoError(t, err)

	// Test 1: Verify order via Collection endpoint
	collectionResp, err := f.handler.HttpBodyFormDataCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)

	var foundInCollection bool
	for _, item := range collectionResp.Msg.GetItems() {
		itemID, _ := idwrap.NewFromBytes(item.GetHttpBodyFormDataId())
		if itemID == formID {
			foundInCollection = true
			require.Equal(t, expectedOrder, item.GetOrder(), "Order should match in Collection response")
			break
		}
	}
	require.True(t, foundInCollection, "Form entry should be found in Collection response")

	// Test 2: Verify order via Sync stream
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

	// Update the form entry to trigger a sync event
	newOrder := float32(77.7)
	updateReq := connect.NewRequest(&httpv1.HttpBodyFormDataUpdateRequest{
		Items: []*httpv1.HttpBodyFormDataUpdate{
			{
				HttpBodyFormDataId: formID.Bytes(),
				Order:              &newOrder,
			},
		},
	})

	_, err = f.handler.HttpBodyFormDataUpdate(f.ctx, updateReq)
	require.NoError(t, err)

	// Wait for UPDATE event in sync stream
	timeout := time.After(2 * time.Second)
	var foundInSync bool

	for !foundInSync {
		select {
		case resp, ok := <-msgCh:
			require.True(t, ok, "Stream closed before event received")
			for _, item := range resp.GetItems() {
				val := item.GetValue()
				if val == nil {
					continue
				}
				if val.GetKind() == httpv1.HttpBodyFormDataSync_ValueUnion_KIND_UPDATE {
					update := val.GetUpdate()
					eventID, _ := idwrap.NewFromBytes(update.GetHttpBodyFormDataId())
					if eventID == formID {
						foundInSync = true
						require.Equal(t, newOrder, update.GetOrder(), "Order should match in Sync response")
						break
					}
				}
			}
		case <-timeout:
			require.FailNow(t, "Timeout waiting for sync event")
		}
	}
}

func TestHttpBodyUrlEncodedOrderRoundTrip(t *testing.T) {
	t.Parallel()

	f := newHttpStreamingFixture(t)
	wsID := f.createWorkspace(t, "urlenc-roundtrip-test")
	httpID := f.createHttp(t, wsID, "urlenc-roundtrip-http", "https://example.com", "POST")

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	// Insert a urlencoded entry with specific order
	urlencID := idwrap.NewNow()
	expectedOrder := float32(55.5)

	insertReq := connect.NewRequest(&httpv1.HttpBodyUrlEncodedInsertRequest{
		Items: []*httpv1.HttpBodyUrlEncodedInsert{
			{
				HttpBodyUrlEncodedId: urlencID.Bytes(),
				HttpId:               httpID.Bytes(),
				Key:                  "order-test-key",
				Value:                "test",
				Enabled:              true,
				Order:                expectedOrder,
			},
		},
	})

	_, err := f.handler.HttpBodyUrlEncodedInsert(f.ctx, insertReq)
	require.NoError(t, err)

	// Test 1: Verify order via Collection endpoint
	collectionResp, err := f.handler.HttpBodyUrlEncodedCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)

	var foundInCollection bool
	for _, item := range collectionResp.Msg.GetItems() {
		itemID, _ := idwrap.NewFromBytes(item.GetHttpBodyUrlEncodedId())
		if itemID == urlencID {
			foundInCollection = true
			require.Equal(t, expectedOrder, item.GetOrder(), "Order should match in Collection response")
			break
		}
	}
	require.True(t, foundInCollection, "UrlEncoded entry should be found in Collection response")

	// Test 2: Verify order via Sync stream
	msgCh := make(chan *httpv1.HttpBodyUrlEncodedSyncResponse, 10)
	errCh := make(chan error, 1)
	readyCh := make(chan struct{})

	go func() {
		err := f.handler.streamHttpBodyUrlEncodedSync(ctx, f.userID, func(resp *httpv1.HttpBodyUrlEncodedSyncResponse) error {
			msgCh <- resp
			return nil
		}, &eventstream.BulkOptions{
			Ready: readyCh,
		})
		errCh <- err
		close(msgCh)
	}()

	// Wait for stream to be ready
	<-readyCh

	// Update the urlencoded entry to trigger a sync event
	newOrder := float32(88.8)
	updateReq := connect.NewRequest(&httpv1.HttpBodyUrlEncodedUpdateRequest{
		Items: []*httpv1.HttpBodyUrlEncodedUpdate{
			{
				HttpBodyUrlEncodedId: urlencID.Bytes(),
				Order:                &newOrder,
			},
		},
	})

	_, err = f.handler.HttpBodyUrlEncodedUpdate(f.ctx, updateReq)
	require.NoError(t, err)

	// Wait for UPDATE event in sync stream
	timeout := time.After(2 * time.Second)
	var foundInSync bool

	for !foundInSync {
		select {
		case resp, ok := <-msgCh:
			require.True(t, ok, "Stream closed before event received")
			for _, item := range resp.GetItems() {
				val := item.GetValue()
				if val == nil {
					continue
				}
				if val.GetKind() == httpv1.HttpBodyUrlEncodedSync_ValueUnion_KIND_UPDATE {
					update := val.GetUpdate()
					eventID, _ := idwrap.NewFromBytes(update.GetHttpBodyUrlEncodedId())
					if eventID == urlencID {
						foundInSync = true
						require.Equal(t, newOrder, update.GetOrder(), "Order should match in Sync response")
						break
					}
				}
			}
		case <-timeout:
			require.FailNow(t, "Timeout waiting for sync event")
		}
	}
}
