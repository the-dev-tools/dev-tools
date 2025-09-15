package rflow

import (
    "encoding/json"
    "reflect"
    "the-dev-tools/server/pkg/errmap"
    "the-dev-tools/server/pkg/flow/runner"
    "the-dev-tools/server/pkg/reference"
)

// formatErrForUser returns a user-friendly error string.
// If the error is an errmap.Error, it prefixes the code for quick scanning.
func formatErrForUser(err error) string { return errmap.Friendly(err) }

// normalizeForLog converts OutputData values into log-friendly forms:
// - []byte -> if JSON, unmarshal to any; else convert to string
// - map[string]any / []any -> recurse
func normalizeForLog(v any) any {
    switch t := v.(type) {
    case string:
        // Attempt to parse JSON string into structured value
        bs := []byte(t)
        if len(bs) > 0 && (bs[0] == '{' || bs[0] == '[') && json.Valid(bs) {
            var out any
            if err := json.Unmarshal(bs, &out); err == nil {
                return normalizeForLog(out)
            }
        }
        return t
    case []byte:
        if json.Valid(t) {
            var out any
            if err := json.Unmarshal(t, &out); err == nil {
                return normalizeForLog(out)
            }
        }
        return string(t)
    case json.RawMessage:
        if json.Valid(t) {
            var out any
            if err := json.Unmarshal(t, &out); err == nil {
                return normalizeForLog(out)
            }
        }
        return string(t)
    case map[string]any:
        m := make(map[string]any, len(t))
        for k, val := range t {
            m[k] = normalizeForLog(val)
        }
        return m
    case []any:
        arr := make([]any, len(t))
        for i := range t {
            arr[i] = normalizeForLog(t[i])
        }
        return arr
    default:
        // Fallback: handle maps/slices via reflection to catch typed maps (e.g., map[string][]byte)
        rv := reflect.ValueOf(v)
        switch rv.Kind() {
        case reflect.Map:
            // Only handle string-keyed maps
            if rv.Type().Key().Kind() == reflect.String {
                m := make(map[string]any, rv.Len())
                for _, mk := range rv.MapKeys() {
                    key := mk.String()
                    mv := rv.MapIndex(mk).Interface()
                    m[key] = normalizeForLog(mv)
                }
                return m
            }
        case reflect.Slice:
            // If it's a []uint8 (aka []byte), handle as bytes
            if rv.Type().Elem().Kind() == reflect.Uint8 {
                b := make([]byte, rv.Len())
                reflect.Copy(reflect.ValueOf(b), rv)
                if json.Valid(b) {
                    var out any
                    if err := json.Unmarshal(b, &out); err == nil {
                        return normalizeForLog(out)
                    }
                }
                return string(b)
            }
            // Generic slice
            n := rv.Len()
            arr := make([]any, n)
            for i := 0; i < n; i++ {
                arr[i] = normalizeForLog(rv.Index(i).Interface())
            }
            return arr
        }
        return v
    }
}

// buildLogRefs constructs structured log references for a node state change.
// Error-first behavior:
//   - If nodeError != nil, prefer an error payload with minimal node info and
//     error { message, kind } and optional failure context keys from outputData.
//   - Else, if outputData is a map, normalize and render it as-is.
//   - Else, fall back to a small metadata struct.
func buildLogRefs(nameForLog, idStrForLog, stateStrForLog string, nodeError error, outputData any) []reference.ReferenceTreeItem {
    if nodeError != nil {
        kind := "failed"
        if runner.IsCancellationError(nodeError) {
            kind = "canceled"
        }
        payload := map[string]any{
            "node": map[string]any{
                "id":    idStrForLog,
                "name":  nameForLog,
                "state": stateStrForLog,
            },
            "error": map[string]any{
                "message": nodeError.Error(),
                "kind":    kind,
            },
        }
        // Include only safe failure context keys (from foreach summaries)
        if m, ok := outputData.(map[string]any); ok {
            ctx := map[string]any{}
            if v, ok := m["failedAtIndex"]; ok { ctx["failedAtIndex"] = v }
            if v, ok := m["failedAtKey"]; ok { ctx["failedAtKey"] = v }
            if v, ok := m["totalItems"]; ok { ctx["totalItems"] = v }
            if len(ctx) > 0 { payload["context"] = ctx }
        }
        ref := reference.NewReferenceFromInterfaceWithKey(payload, nameForLog)
        return []reference.ReferenceTreeItem{ref}
    }

    if outputData != nil {
        if out, ok := outputData.(map[string]any); ok {
            // If OutputData is nested under node name, unwrap once
            src := out
            if nb, ok := out[nameForLog].(map[string]any); ok {
                src = nb
            }
            if norm, ok := normalizeForLog(src).(map[string]any); ok {
                r := reference.NewReferenceFromInterfaceWithKey(norm, nameForLog)
                return []reference.ReferenceTreeItem{r}
            }
        }
    }

    // Fallback minimal payload
    logData := struct {
        NodeID string
        Name   string
        State  string
        Error  error
    }{
        NodeID: idStrForLog,
        Name:   nameForLog,
        State:  stateStrForLog,
        Error:  nil,
    }
    ref := reference.NewReferenceFromInterfaceWithKey(logData, nameForLog)
    return []reference.ReferenceTreeItem{ref}
}

