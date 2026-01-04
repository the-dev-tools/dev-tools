package tpostmanv2

import (
	"os"
	"path/filepath"
	"testing"

	"the-dev-tools/server/pkg/idwrap"
	"github.com/stretchr/testify/require"
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
}
