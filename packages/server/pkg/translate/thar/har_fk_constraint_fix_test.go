package thar

import (
	"testing"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mexampleheader"
)

// TestHARConversionNoFKReferences verifies that HAR conversion no longer sets
// Prev/Next references that would cause foreign key constraint violations
func TestHARConversionNoFKReferences(t *testing.T) {
	collectionID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()

	harData := `{
		"log": {
			"entries": [
				{
					"startedDateTime": "2024-01-01T10:00:00.000Z",
					"_resourceType": "xhr",
					"request": {
						"method": "GET",
						"url": "https://api.example.com/users",
						"headers": [
							{"name": "Authorization", "value": "Bearer token123"},
							{"name": "Content-Type", "value": "application/json"}
						]
					},
					"response": {
						"status": 200,
						"headers": [],
						"content": {"text": "{}"}
					}
				},
				{
					"startedDateTime": "2024-01-01T10:00:01.000Z",
					"_resourceType": "xhr", 
					"request": {
						"method": "POST",
						"url": "https://api.example.com/users",
						"headers": [
							{"name": "Authorization", "value": "Bearer token123"},
							{"name": "Content-Type", "value": "application/json"},
							{"name": "X-Request-ID", "value": "req-456"}
						]
					},
					"response": {
						"status": 201,
						"headers": [],
						"content": {"text": "{}"}
					}
				}
			]
		}
	}`

	har, err := ConvertRaw([]byte(harData))
	if err != nil {
		t.Fatalf("Failed to parse HAR: %v", err)
	}

	resolved, err := ConvertHAR(har, collectionID, workspaceID)
	if err != nil {
		t.Fatalf("Failed to convert HAR: %v", err)
	}

	t.Run("Headers_Have_No_Prev_Next_References", func(t *testing.T) {
		if len(resolved.Headers) == 0 {
			t.Fatal("Expected headers to be created")
		}

		// Verify that ALL headers have nil Prev and Next
		for i, header := range resolved.Headers {
			if header.Prev != nil {
				t.Errorf("Header %d has Prev reference %v, expected nil", i, header.Prev)
			}
			if header.Next != nil {
				t.Errorf("Header %d has Next reference %v, expected nil", i, header.Next)
			}
		}
		
		t.Logf("✅ Successfully verified %d headers have no Prev/Next references", len(resolved.Headers))
	})

	t.Run("Multiple_Examples_Multiple_Headers", func(t *testing.T) {
		// Count headers per example
		headersByExample := make(map[idwrap.IDWrap][]mexampleheader.Header)
		for _, header := range resolved.Headers {
			headersByExample[header.ExampleID] = append(headersByExample[header.ExampleID], header)
		}

		if len(headersByExample) < 2 {
			t.Errorf("Expected at least 2 examples with headers, got %d", len(headersByExample))
		}

		for exampleID, headers := range headersByExample {
			if len(headers) < 2 {
				continue // Skip examples with fewer than 2 headers
			}

			t.Logf("Example %s has %d headers", exampleID.String()[:8], len(headers))

			// Verify each header in this example has no links
			for i, header := range headers {
				if header.Prev != nil || header.Next != nil {
					t.Errorf("Example %s header %d has links: Prev=%v, Next=%v",
						exampleID.String()[:8], i, header.Prev, header.Next)
				}
			}
		}
	})

	t.Run("Headers_Can_Be_Created_Individually", func(t *testing.T) {
		// Simulate what the database layer would do - try to create headers individually
		// without any FK constraint violations
		
		for i, header := range resolved.Headers {
			// Each header should be creatable independently since no FK references exist
			if header.Prev != nil {
				t.Errorf("Header %d would cause FK constraint violation with Prev=%v", i, header.Prev)
			}
			if header.Next != nil {
				t.Errorf("Header %d would cause FK constraint violation with Next=%v", i, header.Next)
			}

			// Verify essential fields are set
			if len(header.ID.Bytes()) == 0 {
				t.Errorf("Header %d has empty ID", i)
			}
			if len(header.ExampleID.Bytes()) == 0 {
				t.Errorf("Header %d has empty ExampleID", i)
			}
			if header.HeaderKey == "" {
				t.Errorf("Header %d has empty HeaderKey", i)
			}
		}

		t.Logf("✅ All %d headers can be created independently without FK constraints", len(resolved.Headers))
	})
}

// TestHARConversionFixRegression verifies that the fix doesn't break other functionality
func TestHARConversionFixRegression(t *testing.T) {
	collectionID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()

	harData := `{
		"log": {
			"entries": [
				{
					"startedDateTime": "2024-01-01T10:00:00.000Z",
					"_resourceType": "xhr",
					"request": {
						"method": "GET",
						"url": "https://api.example.com/test",
						"headers": [{"name": "Accept", "value": "application/json"}]
					},
					"response": {
						"status": 200,
						"headers": [],
						"content": {"text": "{}"}
					}
				}
			]
		}
	}`

	har, err := ConvertRaw([]byte(harData))
	if err != nil {
		t.Fatalf("Failed to parse HAR: %v", err)
	}

	resolved, err := ConvertHAR(har, collectionID, workspaceID)
	if err != nil {
		t.Fatalf("Failed to convert HAR: %v", err)
	}

	// Verify core functionality still works
	if len(resolved.Apis) == 0 {
		t.Error("Expected APIs to be created")
	}
	if len(resolved.Examples) == 0 {
		t.Error("Expected Examples to be created")
	}
	if len(resolved.Headers) == 0 {
		t.Error("Expected Headers to be created")
	}
	if len(resolved.Nodes) == 0 {
		t.Error("Expected Flow Nodes to be created")
	}

	t.Logf("✅ HAR conversion still creates: %d APIs, %d Examples, %d Headers, %d Nodes",
		len(resolved.Apis), len(resolved.Examples), len(resolved.Headers), len(resolved.Nodes))
}