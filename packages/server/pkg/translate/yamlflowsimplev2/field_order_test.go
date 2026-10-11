package yamlflowsimplev2

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/ioworkspace"
)

// Field names deliberately out of alphabetical order so neither Go map
// iteration nor sorting can produce the YAML order by accident.
var orderedFieldNames = []string{
	"zeta", "1_$ACTION_ID_abc", "alpha", "mike", "bravo",
	"yankee", "charlie", "xray", "delta", "kilo",
}

const fieldOrderRuns = 30

func mapBlock(indent string) string {
	var sb strings.Builder
	for i, name := range orderedFieldNames {
		sb.WriteString(indent + `"` + name + `": "v` + string(rune('0'+i)) + `"` + "\n")
	}
	return sb.String()
}

func fieldOrderYAML(bodyType, bodyKey string) string {
	return `workspace_name: Order Test
flows:
  - name: Order Flow
    steps:
      - manual_start:
          name: Start
      - request:
          name: Login
          method: POST
          url: https://example.com/login
          depends_on:
            - Start
          headers:
` + mapBlock("            ") + `          query_params:
` + mapBlock("            ") + `          body:
            type: "` + bodyType + `"
            ` + bodyKey + `:
` + mapBlock("              ")
}

func requireOrder(t *testing.T, label string, got []string) {
	t.Helper()
	require.Equal(t, orderedFieldNames, got, "%s order must match YAML order", label)
}

func formKeys(b *ioworkspace.WorkspaceBundle) []string {
	out := make([]string, 0, len(b.HTTPBodyForms))
	for i, f := range b.HTTPBodyForms {
		if i > 0 && f.DisplayOrder <= b.HTTPBodyForms[i-1].DisplayOrder {
			return nil
		}
		out = append(out, f.Key)
	}
	return out
}

func urlencodedKeys(b *ioworkspace.WorkspaceBundle) []string {
	out := make([]string, 0, len(b.HTTPBodyUrlencoded))
	for i, f := range b.HTTPBodyUrlencoded {
		if i > 0 && f.DisplayOrder <= b.HTTPBodyUrlencoded[i-1].DisplayOrder {
			return nil
		}
		out = append(out, f.Key)
	}
	return out
}

func headerKeys(b *ioworkspace.WorkspaceBundle) []string {
	out := make([]string, 0, len(b.HTTPHeaders))
	for i, h := range b.HTTPHeaders {
		if i > 0 && h.DisplayOrder <= b.HTTPHeaders[i-1].DisplayOrder {
			return nil
		}
		out = append(out, h.Key)
	}
	return out
}

func queryKeys(b *ioworkspace.WorkspaceBundle) []string {
	out := make([]string, 0, len(b.HTTPSearchParams))
	for i, q := range b.HTTPSearchParams {
		if i > 0 && q.DisplayOrder <= b.HTTPSearchParams[i-1].DisplayOrder {
			return nil
		}
		out = append(out, q.Key)
	}
	return out
}

// requireYAMLKeyOrder checks the names appear in the exported YAML in order.
func requireYAMLKeyOrder(t *testing.T, out string) {
	t.Helper()
	pos := 0
	for _, name := range orderedFieldNames {
		idx := strings.Index(out[pos:], name)
		require.GreaterOrEqual(t, idx, 0, "field %q missing or out of order in export:\n%s", name, out)
		pos += idx + len(name)
	}
}

func TestFieldOrderPreserved(t *testing.T) {
	cases := []struct {
		name     string
		bodyType string
		bodyKey  string
		bodyKeys func(*ioworkspace.WorkspaceBundle) []string
	}{
		{"form_data", BodyTypeFormData, "form_data", formKeys},
		{"urlencoded", BodyTypeUrlEncoded, "urlencoded", urlencodedKeys},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte(fieldOrderYAML(tc.bodyType, tc.bodyKey))
			for range fieldOrderRuns {
				bundle, err := ConvertSimplifiedYAML(data, GetDefaultOptions(idwrap.NewNow()))
				require.NoError(t, err)

				requireOrder(t, tc.name, tc.bodyKeys(bundle))
				requireOrder(t, "headers", headerKeys(bundle))
				requireOrder(t, "query_params", queryKeys(bundle))

				out, err := MarshalSimplifiedYAML(bundle)
				require.NoError(t, err)
				exported := string(out)
				bodyIdx := strings.Index(exported, tc.bodyKey+":")
				require.GreaterOrEqual(t, bodyIdx, 0, exported)
				requireYAMLKeyOrder(t, exported[bodyIdx:])

				again, err := ConvertSimplifiedYAML(out, GetDefaultOptions(idwrap.NewNow()))
				require.NoError(t, err)
				requireOrder(t, tc.name+" (re-import)", tc.bodyKeys(again))
				requireOrder(t, "headers (re-import)", headerKeys(again))
				requireOrder(t, "query_params (re-import)", queryKeys(again))
			}
		})
	}
}
