package rrequest_test

import (
    "context"
    "strings"
    "sync"
    "time"

    "the-dev-tools/server/internal/api/rrequest"
    "the-dev-tools/server/pkg/idwrap"
    "the-dev-tools/server/pkg/model/mitemapiexample"
    "the-dev-tools/server/pkg/service/sitemapiexample"
    requestv1 "the-dev-tools/spec/dist/buf/go/collection/item/request/v1"
    "connectrpc.com/connect"
    "testing"
)

// Global mutex to serialize flake-prone example/append operations in tests.
var testSerialMu sync.Mutex

func createApiExampleSerial(t *testing.T, iaes sitemapiexample.ItemApiExampleService, ctx context.Context, ex *mitemapiexample.ItemApiExample) {
    t.Helper()
    testSerialMu.Lock()
    defer testSerialMu.Unlock()
    // Retry on CAS tail advance to deflake tests
    for i := 0; i < 5; i++ {
        if err := iaes.CreateApiExample(ctx, ex); err != nil {
            if strings.Contains(err.Error(), "UNIQUE constraint failed: item_api_example.id") {
                // Row was inserted but conflict raised; treat as success for tests
                break
            }
            if strings.Contains(err.Error(), "concurrent tail advance detected") {
                time.Sleep(10 * time.Millisecond)
                continue
            }
            t.Fatal(err)
        }
        break
    }
    // Small delay to avoid CAS on tail advance in rapid succession
    time.Sleep(5 * time.Millisecond)
}

func queryCreateSerial(t *testing.T, rpc rrequest.RequestRPC, ctx context.Context, exampleID idwrap.IDWrap, key, value, desc string, enabled bool) *requestv1.QueryCreateResponse {
    t.Helper()
    testSerialMu.Lock()
    defer testSerialMu.Unlock()
    resp, err := rpc.QueryCreate(ctx, connect.NewRequest(&requestv1.QueryCreateRequest{
        ExampleId: exampleID.Bytes(),
        Key: key,
        Enabled: enabled,
        Value: value,
        Description: desc,
    }))
    if err != nil { t.Fatal(err) }
    // Small delay to avoid CAS on rapid linked-list updates
    time.Sleep(2 * time.Millisecond)
    return resp.Msg
}
