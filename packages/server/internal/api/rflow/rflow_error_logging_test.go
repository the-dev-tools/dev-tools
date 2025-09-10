package rflow

import (
    "encoding/json"
    "errors"
    "testing"
    "the-dev-tools/server/pkg/flow/runner"
    "the-dev-tools/server/pkg/idwrap"
    "the-dev-tools/server/pkg/logconsole"
    "the-dev-tools/server/pkg/reference"
)

// sendRefsAndDecode pushes refs to the log console and returns the decoded JSON object.
func sendRefsAndDecode(t *testing.T, name string, refs []reference.ReferenceTreeItem) map[string]any {
    t.Helper()
    ch := make(chan logconsole.LogMessage, 1)
    logconsole.SendLogMessage(ch, idwrap.NewNow(), name, logconsole.LogLevelError, refs)
    msg := <-ch
    if msg.JSON == "" {
        t.Fatalf("expected non-empty JSON payload")
    }
    var m map[string]any
    if err := json.Unmarshal([]byte(msg.JSON), &m); err != nil {
        t.Fatalf("failed to unmarshal JSON: %v", err)
    }
    return m
}

func TestBuildLogRefs_ErrorFirst_WithForEachSummary(t *testing.T) {
    name := "ForEach Node"
    id := "node-123"
    state := "Failure"
    output := map[string]any{
        "failedAtIndex": 3,
        "totalItems":    5,
        // simulate stray iteration vars that should not appear in error context
        "item": "should-not-leak",
        "key":  3,
    }
    refs := buildLogRefs(name, id, state, errors.New("boom"), output)

    obj := sendRefsAndDecode(t, name, refs)

    nodeObj, ok := obj[name].(map[string]any)
    if !ok {
        t.Fatalf("expected top-level key %q", name)
    }
    // error present
    errMap, ok := nodeObj["error"].(map[string]any)
    if !ok {
        t.Fatalf("expected error map in payload")
    }
    if _, ok := errMap["message"]; !ok {
        t.Fatalf("expected error.message present")
    }
    // context should only include allowed failure keys
    ctx, ok := nodeObj["context"].(map[string]any)
    if !ok {
        t.Fatalf("expected context map in payload")
    }
    if _, ok := ctx["failedAtIndex"]; !ok {
        t.Fatalf("expected failedAtIndex in context")
    }
    if _, ok := ctx["totalItems"]; !ok {
        t.Fatalf("expected totalItems in context")
    }
    if _, ok := ctx["item"]; ok {
        t.Fatalf("did not expect item in context")
    }
    if _, ok := ctx["key"]; ok {
        t.Fatalf("did not expect key in context")
    }
}

func TestBuildLogRefs_CancellationKind(t *testing.T) {
    name := "Loop Node"
    id := "node-456"
    state := "Canceled"
    // simulate cancellation using sentinel
    err := runner.ErrFlowCanceledByThrow
    refs := buildLogRefs(name, id, state, err, map[string]any{"failedAtIndex": 1})

    obj := sendRefsAndDecode(t, name, refs)
    nodeObj := obj[name].(map[string]any)
    errMap := nodeObj["error"].(map[string]any)
    if k, _ := errMap["kind"].(string); k != "canceled" {
        t.Fatalf("expected error.kind=canceled, got %v", k)
    }
}

func TestBuildLogRefs_Success_UsesOutputData(t *testing.T) {
    name := "Any Node"
    id := "n-1"
    state := "Success"
    output := map[string]any{"foo": "bar", "arr": []any{1, 2}}
    refs := buildLogRefs(name, id, state, nil, output)

    obj := sendRefsAndDecode(t, name, refs)
    nodeObj := obj[name].(map[string]any)
    if nodeObj["foo"].(string) != "bar" {
        t.Fatalf("expected foo=bar in output, got %v", nodeObj["foo"]) 
    }
    if _, ok := nodeObj["error"]; ok {
        t.Fatalf("did not expect error in success payload")
    }
}

