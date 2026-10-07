package yamlflowsimplev2

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/ioworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

const cleanupYAML = `
workspace_name: Cleanup WS
requests:
  - name: DeleteProduct
    method: DELETE
    url: "https://api.example.com/products/{{ PostProducts.response.body.id }}"
graphql_requests:
  - name: PurgeCache
    url: https://api.example.com/graphql
    query: "mutation { purge }"
flows:
  - name: Catalog
    steps:
      - request:
          name: PostCategory
          method: POST
          url: https://api.example.com/categories
      - request:
          name: PostProducts
          method: POST
          url: https://api.example.com/products
          depends_on: PostCategory
    cleanup:
      - request:
          name: RemoveProduct
          use_request: DeleteProduct
      - request:
          name: RemoveCategory
          method: DELETE
          url: "https://api.example.com/categories/{{PostCategory.response.body.id}}"
          headers:
            X-Product: "{{ RemoveProduct.response.status }}"
      - graphql:
          name: Purge
          use_request: PurgeCache
`

func convertCleanup(t *testing.T, doc string) *ioworkspace.WorkspaceBundle {
	t.Helper()
	bundle, err := ConvertSimplifiedYAML([]byte(doc), GetDefaultOptions(idwrap.NewNow()))
	require.NoError(t, err)
	return bundle
}

func TestConvertCleanup_BuildsHiddenFlow(t *testing.T) {
	bundle := convertCleanup(t, cleanupYAML)

	require.Len(t, bundle.Flows, 1, "the cleanup flow must not be a regular flow")
	require.Len(t, bundle.FlowCleanups, 1)

	cleanup := bundle.FlowCleanups[0]
	require.Equal(t, bundle.Flows[0].ID, cleanup.FlowID)
	require.NotNil(t, cleanup.Bundle)
	require.Len(t, cleanup.Bundle.Flows, 1)
	require.Equal(t, cleanup.CleanupFlowID, cleanup.Bundle.Flows[0].ID)
	require.Equal(t, bundle.Flows[0].WorkspaceID, cleanup.Bundle.Flows[0].WorkspaceID)

	// Cleanup entities stay out of the owning bundle's slices.
	for _, n := range bundle.FlowNodes {
		require.NotEqual(t, cleanup.CleanupFlowID, n.FlowID)
		require.NotContains(t, []string{"RemoveProduct", "RemoveCategory", "Purge"}, n.Name)
	}
	require.Empty(t, cleanup.Bundle.Files, "cleanup steps get no file tree entries")
	require.Empty(t, cleanup.Bundle.FlowEdges, "steps without depends_on need no edges")

	// No start node: cleanup nodes never run as a graph.
	require.Len(t, cleanup.Bundle.FlowNodes, 3)
	for _, n := range cleanup.Bundle.FlowNodes {
		require.Equal(t, cleanup.CleanupFlowID, n.FlowID)
		require.NotEqual(t, mflow.NODE_KIND_MANUAL_START, n.NodeKind)
	}
	require.Len(t, cleanup.Bundle.HTTPRequests, 2)
	require.Len(t, cleanup.Bundle.FlowRequestNodes, 2)
	require.Len(t, cleanup.Bundle.GraphQLRequests, 1)
	require.Len(t, cleanup.Bundle.FlowGraphQLNodes, 1)

	names := make([]string, 0, len(cleanup.Steps))
	for _, s := range cleanup.Steps {
		names = append(names, s.Name)
	}
	require.Equal(t, []string{"RemoveProduct", "RemoveCategory", "Purge"}, names, "listed order")

	// References come from the resolved request, template included.
	require.Equal(t, []string{"PostProducts"}, cleanup.Steps[0].References)
	require.Equal(t, []string{"PostCategory", "RemoveProduct"}, cleanup.Steps[1].References)
	require.Empty(t, cleanup.Steps[2].References)
	for i, s := range cleanup.Steps {
		require.Equal(t, cleanup.Bundle.FlowNodes[i].ID, s.NodeID)
	}
}

func TestConvertCleanup_DependsOnOrdersSteps(t *testing.T) {
	bundle := convertCleanup(t, `
workspace_name: WS
flows:
  - name: F
    steps:
      - request:
          name: Create
          method: POST
          url: https://x.test/a
    cleanup:
      - request:
          name: Second
          method: DELETE
          url: https://x.test/b
          depends_on: First
      - request:
          name: First
          method: DELETE
          url: https://x.test/a
      - request:
          name: Third
          method: GET
          url: https://x.test/c
`)
	steps := bundle.FlowCleanups[0].Steps
	require.Len(t, steps, 3)
	require.Equal(t, "First", steps[0].Name)
	require.Equal(t, "Second", steps[1].Name)
	require.Equal(t, []string{"First"}, steps[1].DependsOn)
	require.Equal(t, "Third", steps[2].Name)
	require.Len(t, bundle.FlowCleanups[0].Bundle.FlowEdges, 1)
}

func TestConvertCleanup_NoCleanupBlock(t *testing.T) {
	bundle := convertCleanup(t, `
workspace_name: WS
flows:
  - name: F
    steps:
      - request:
          name: Create
          method: POST
          url: https://x.test/a
`)
	require.Empty(t, bundle.FlowCleanups)
}

func TestConvertCleanup_Errors(t *testing.T) {
	head := `
workspace_name: WS
flows:
  - name: F
    steps:
      - request:
          name: Create
          method: POST
          url: https://x.test/a
    cleanup:
`
	cases := map[string]struct {
		cleanup string
		want    string
	}{
		"unsupported kind": {
			cleanup: `
      - js:
          name: Script
          code: "1"
`,
			want: "cleanup step 'Script': only request and graphql steps are supported in cleanup",
		},
		"depends on normal step": {
			cleanup: `
      - request:
          name: Remove
          method: DELETE
          url: https://x.test/a
          depends_on: Create
`,
			want: "cleanup step 'Remove' depends on 'Create', which is not a cleanup step",
		},
		"depends on unknown step": {
			cleanup: `
      - request:
          name: Remove
          method: DELETE
          url: https://x.test/a
          depends_on: Ghost
`,
			want: "cleanup step 'Remove' depends on 'Ghost', which is not a cleanup step",
		},
		"name collides with normal step": {
			cleanup: `
      - request:
          name: Create
          method: DELETE
          url: https://x.test/a
`,
			want: "cleanup step 'Create' has the same name as a step",
		},
		"duplicate cleanup name": {
			cleanup: `
      - request:
          name: Remove
          method: DELETE
          url: https://x.test/a
      - request:
          name: Remove
          method: DELETE
          url: https://x.test/b
`,
			want: "duplicate cleanup step name 'Remove'",
		},
		"missing name": {
			cleanup: `
      - request:
          method: DELETE
          url: https://x.test/a
`,
			want: "cleanup step 1 is missing a name",
		},
		"cycle": {
			cleanup: `
      - request:
          name: A
          method: DELETE
          url: https://x.test/a
          depends_on: B
      - request:
          name: B
          method: DELETE
          url: https://x.test/b
          depends_on: A
`,
			want: "cleanup steps have a dependency cycle",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ConvertSimplifiedYAML([]byte(head+tc.cleanup), GetDefaultOptions(idwrap.NewNow()))
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestStepReferences(t *testing.T) {
	names := []string{"PostProducts", "Post", "Login", "Self"}
	cases := map[string][]string{
		"{{ PostProducts.response.body.id }}":       {"PostProducts"},
		"{{PostProducts.response.body.id}}":         {"PostProducts"},
		"PostProducts.response.body.id":             nil, // not inside a template
		"{{ Login.response.body.token }} {{ env }}": {"Login"},
		"{{ #env:Post }}":                           nil, // env lookup, not a step
		"{{ Post }}":                                {"Post"},
		"{{ Self.response.status }}":                nil, // own name is excluded
		"{{ len(PostProducts.response.body) > 0 }}": {"PostProducts"},
		"{{ PostProductsX.id }}":                    nil,
	}
	for in, want := range cases {
		got := stepReferences([]string{in}, names, "Self")
		if want == nil {
			require.Empty(t, got, in)
		} else {
			require.Equal(t, want, got, in)
		}
	}
}
