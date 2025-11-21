package rhttp

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"the-dev-tools/server/pkg/idwrap"
	httpv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
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

	if _, err := f.handler.HttpHeaderInsert(f.ctx, insertReq); err != nil {
		t.Fatalf("HttpHeaderInsert err: %v", err)
	}

	// Verify ID in DB
	headers, err := f.handler.httpHeaderService.GetByHttpID(f.ctx, httpID)
	if err != nil {
		t.Fatalf("GetByHttpID err: %v", err)
	}

	foundInDB := false
	for _, h := range headers {
		if h.ID == clientGeneratedID {
			foundInDB = true
			if h.Key != headerKey {
				t.Errorf("Header key mismatch in DB. Expected %s, got %s", headerKey, h.Key)
			}
			break
		}
	}
	if !foundInDB {
		t.Fatalf("Header with client ID %s not found in DB", clientGeneratedID.String())
	}

	// Verify ID in Sync Event
	timeout := time.After(2 * time.Second)
	foundInStream := false
	
	// We might receive snapshot events first, so we loop until we find ours or timeout
	for {
		select {
		case resp, ok := <-msgCh:
			if !ok {
				t.Fatal("Stream closed before event received")
			}
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
						if insert.GetKey() != headerKey {
							t.Errorf("Header key mismatch in stream. Expected %s, got %s", headerKey, insert.GetKey())
						}
						goto Done
					}
				}
			}
		case <-timeout:
			t.Fatal("Timeout waiting for sync event")
		}
	}
Done:
	if !foundInStream {
		t.Fatal("Header inserted event not received in stream")
	}
}
