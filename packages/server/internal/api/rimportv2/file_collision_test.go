package rimportv2

import (
	"testing"

	"github.com/stretchr/testify/require"

	apiv1 "the-dev-tools/spec/dist/buf/go/api/import/v1"

	"connectrpc.com/connect"
)

func TestImportService_FileCollision(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	fixture := newIntegrationTestFixture(t)

	// Create a HAR with two requests that will end up in different folders but have same names
	// Request 1: https://api.example.com/v1/users
	// Request 2: https://api.other.com/v1/users
	// They both have name "users" (or generated name will be similar)
	// harv2 generates names based on path if not provided?
	// Actually harv2 uses generateRequestName which is just request_1, request_2...
	
	// Let's use a HAR where we explicitly set names if possible, or just rely on URL structure.
	// harv2.createFileStructure uses sanitizeFileName(httpReq.Name) + ".request"
	
	harData := []byte(`{
		"log": {
			"version": "1.2",
			"entries": [
				{
					"startedDateTime": "2024-01-01T00:00:00.000Z",
					"request": {
						"method": "GET",
						"url": "https://api.example.com/users",
						"headers": []
					},
					"response": {"status": 200, "content": {"mimeType": "application/json", "text": "{}"}}
				},
				{
					"startedDateTime": "2024-01-01T00:00:01.000Z",
					"request": {
						"method": "GET",
						"url": "https://api.other.com/users",
						"headers": []
					},
					"response": {"status": 200, "content": {"mimeType": "application/json", "text": "{}"}}
				}
			]
		}
	}`)

	req := connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "Collision Test",
		Data:        harData,
		DomainData:  []*apiv1.ImportDomainData{}, // Signal that we want to proceed without domain mapping
	})

	_, err := fixture.rpc.Import(fixture.ctx, req)
	require.NoError(t, err)

	// Verify files in DB
	files, err := fixture.services.FileService.ListFilesByWorkspace(fixture.ctx, fixture.workspaceID)
	require.NoError(t, err)

	// Log files for debugging
	for _, f := range files {
		t.Logf("File: ID=%s, ParentID=%v, Name=%s, ContentType=%d", f.ID, f.ParentID, f.Name, f.ContentType)
	}

	// We expect:
	// com/example/api/users.request
	// com/other/api/users.request
	// Plus their deltas.
	
	// If the bug exists, they might collide.
	// Actually, if they have different content (different URLs), the HTTP requests won't deduplicate.
	// But the FILES might deduplicate if they have the same logicalPath.
	
	// If they deduplicate, they will share the SAME file ID.
	// But they are different HTTP requests, so they should have different files.
	
	// Wait, in harv2, the file ID IS the HTTP request ID.
	// createFileStructure:
	/*
	file := &mfile.File{
		ID:          httpReq.ID,
        ...
	}
	*/
	
	// In StoreUnifiedResults:
	/*
			newID, isNew, err := txDedup.ResolveFile(ctx, file, logicalPath)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to resolve file %s: %w", file.Name, err)
			}

			file.ID = newID
			fileIDMap[oldID] = newID
	*/
	
	// If logicalPath collides, ResolveFile returns the SAME newID for both.
	// So both HTTP requests will point to the SAME file ID.
	
	// Let's check how many unique file IDs we have for .request files
	requestFileCount := 0
	for _, f := range files {
		if f.ContentType == 1 { // mfile.ContentTypeHTTP
			requestFileCount++
		}
	}
	
	// We expect 2 base request files and 2 delta request files = 4 files.
	// If collisions happen, we will have fewer.
	// Base request 1 and Base request 2 will both have Name="request_1.request" (or similar)
	// and ParentID != nil, so logicalPath="imported/request_1.request" for BOTH.
	
	require.Equal(t, 2, requestFileCount, "Should have 2 unique base request files")
}
