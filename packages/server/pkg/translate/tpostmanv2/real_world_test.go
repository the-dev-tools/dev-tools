package tpostmanv2

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

func TestConvertPostmanCollection_RealWorldGalaxy(t *testing.T) {
	// Path to the real Postman collection
	path := filepath.Join("..", "..", "..", "test", "collection", "GalaxyCollection.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	opts := ConvertOptions{
		WorkspaceID:    idwrap.NewNow(),
		CollectionName: "Galaxy Collection",
	}

	resolved, err := ConvertPostmanCollection(data, opts)
	require.NoError(t, err)

	// Verify the new Flow features
	require.NotEmpty(t, resolved.Flow.ID, "Should have generated a Flow ID")
	require.Greater(t, len(resolved.Nodes), 1, "Should have generated Nodes (Start + Requests)")
	require.Greater(t, len(resolved.Edges), 0, "Should have generated Edges connecting the nodes")
	require.Greater(t, len(resolved.RequestNodes), 0, "Should have generated Request Node metadata")

	t.Logf("Imported Real World Collection:")
	t.Logf("  - Requests: %d (Base+Delta)", len(resolved.HTTPRequests))
	t.Logf("  - Flow Nodes: %d", len(resolved.Nodes))
	t.Logf("  - Flow Edges: %d", len(resolved.Edges))
	t.Logf("  - Files/Folders: %d", len(resolved.Files))
	t.Logf("  - Variables: %d", len(resolved.Variables))

	// Verify template variables are extracted (Galaxy collection uses {{your-collection-link}})
	require.Greater(t, len(resolved.Variables), 0, "Should have extracted template variables")

	// Check for your-collection-link variable specifically
	foundCollectionLink := false
	for _, v := range resolved.Variables {
		t.Logf("    Variable: %s = %s", v.Key, v.Value)
		if v.Key == "your-collection-link" {
			foundCollectionLink = true
			require.Equal(t, "https://dev.tools/", v.Value, "Placeholder should default to https://dev.tools/")
		}
	}
	require.True(t, foundCollectionLink, "Should have extracted 'your-collection-link' variable as placeholder")
}
