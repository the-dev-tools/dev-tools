package loadrun

import (
	"reflect"
	"regexp"
	"strings"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nrequest"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

// maxConfigDepth bounds the walk over a node's configuration. Node configs are
// shallow (a request's body or header list sits a few levels down), so this
// only guards against unexpectedly deep or self-referencing values.
const maxConfigDepth = 12

// markLeanBodies decides, per request node, whether lean mode may drop its
// response body. A body is kept when any other node's configuration mentions
// "<Name>.response" - in a template ({{ Login.response.body.token }}), a
// condition, or JS code - because that node reads it at run time. Bodies no
// one references are still dropped, which is what keeps memory flat.
func markLeanBodies(nodes map[idwrap.IDWrap]node.FlowNode) {
	texts := make(map[idwrap.IDWrap]string, len(nodes))
	for id, n := range nodes {
		var b strings.Builder
		collectStrings(reflect.ValueOf(n), &b, map[uintptr]bool{}, 0)
		texts[id] = b.String()
	}

	for id, n := range nodes {
		req, ok := n.(*nrequest.NodeRequest)
		if !ok {
			continue
		}
		ref := regexp.MustCompile(`(^|[^A-Za-z0-9_])` + regexp.QuoteMeta(req.Name) + `\.response\b`)
		req.KeepBodyInLean = false
		for otherID, text := range texts {
			if otherID != id && ref.MatchString(text) {
				req.KeepBodyInLean = true
				break
			}
		}
	}
}

// collectStrings appends every string reachable from v to b, one per line. It
// reads unexported fields too (a JS node keeps its code in one), and skips
// interfaces, funcs and channels, which hold clients and plumbing, not config.
func collectStrings(v reflect.Value, b *strings.Builder, seen map[uintptr]bool, depth int) {
	if depth > maxConfigDepth || !v.IsValid() {
		return
	}
	switch v.Kind() {
	case reflect.String:
		b.WriteString(v.String())
		b.WriteByte('\n')
	case reflect.Pointer:
		if v.IsNil() || seen[v.Pointer()] {
			return
		}
		seen[v.Pointer()] = true
		collectStrings(v.Elem(), b, seen, depth+1)
	case reflect.Struct:
		for i := range v.NumField() {
			collectStrings(v.Field(i), b, seen, depth+1)
		}
	case reflect.Slice, reflect.Array:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			// Raw request bodies are []byte. Byte arrays are IDs, never text.
			if v.Kind() == reflect.Slice {
				b.WriteString(string(v.Bytes()))
				b.WriteByte('\n')
			}
			return
		}
		for i := range v.Len() {
			collectStrings(v.Index(i), b, seen, depth+1)
		}
	case reflect.Map:
		iter := v.MapRange()
		for iter.Next() {
			collectStrings(iter.Key(), b, seen, depth+1)
			collectStrings(iter.Value(), b, seen, depth+1)
		}
	default:
		// Interfaces, funcs, channels and scalars carry no templates.
	}
}
