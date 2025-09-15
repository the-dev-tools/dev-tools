package rflow

import (
    "context"
    "errors"
    "testing"
    "the-dev-tools/server/pkg/flow/runner"
    "the-dev-tools/server/pkg/reference"
)

// childByKey returns the direct map child with the given key.
func childByKey(r reference.ReferenceTreeItem, key string) (reference.ReferenceTreeItem, bool) {
    if r.Kind != reference.ReferenceKind_REFERENCE_KIND_MAP {
        return reference.ReferenceTreeItem{}, false
    }
    for _, ch := range r.Map {
        if ch.Key.Kind == reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY && ch.Key.Key == key {
            return ch, true
        }
    }
    return reference.ReferenceTreeItem{}, false
}

// stringValue extracts the Value from a VALUE node.
func stringValue(r reference.ReferenceTreeItem) (string, bool) {
    if r.Kind != reference.ReferenceKind_REFERENCE_KIND_VALUE {
        return "", false
    }
    return r.Value, true
}

func TestBuildLogRefs_ErrorKindClassification(t *testing.T) {
    // Non-cancellation error should be labeled as "failed"
    refs := buildLogRefs("nodeA", "id-1", "FAILURE", errors.New("boom"), nil)
    if len(refs) == 0 {
        t.Fatalf("expected reference items")
    }
    root := refs[0]
    errRef, ok := childByKey(root, "error")
    if !ok {
        t.Fatalf("missing error map")
    }
    kindRef, ok := childByKey(errRef, "kind")
    if !ok {
        t.Fatalf("missing error.kind")
    }
    if kind, ok := stringValue(kindRef); !ok || kind != "failed" {
        t.Fatalf("expected kind 'failed'")
    }

    // Cancellation by throw should be labeled as "canceled"
    refs = buildLogRefs("nodeB", "id-2", "CANCELED", runner.ErrFlowCanceledByThrow, nil)
    root = refs[0]
    errRef, ok = childByKey(root, "error")
    if !ok {
        t.Fatalf("missing error map")
    }
    kindRef, ok = childByKey(errRef, "kind")
    if !ok {
        t.Fatalf("missing error.kind")
    }
    if kind, ok := stringValue(kindRef); !ok || kind != "canceled" {
        t.Fatalf("expected kind 'canceled' for throw")
    }

    // Context cancellation should be labeled as "canceled"
    refs = buildLogRefs("nodeC", "id-3", "CANCELED", context.Canceled, nil)
    root = refs[0]
    errRef, ok = childByKey(root, "error")
    if !ok {
        t.Fatalf("missing error map")
    }
    kindRef, ok = childByKey(errRef, "kind")
    if !ok {
        t.Fatalf("missing error.kind")
    }
    if kind, ok := stringValue(kindRef); !ok || kind != "canceled" {
        t.Fatalf("expected kind 'canceled' for context cancellation")
    }
}
