package thar_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"the-dev-tools/server/pkg/depfinder"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/translate/thar"
	"time"
)

func TestHarResvoledSimple(t *testing.T) {
	Entry := thar.Entry{}
	Entry.Request.Method = "GET"
	Entry.Request.URL = "http://example.com"
	Entry.Request.HTTPVersion = "HTTP/1.1"
	Entry.Request.Headers = []thar.Header{
		{Name: "Content-Type", Value: "application/json"},
	}
	testHar := thar.HAR{
		Log: thar.Log{
			Entries: []thar.Entry{Entry},
		},
	}
	id := idwrap.NewNow()
	workSpaceID := idwrap.NewNow()

	resolved, err := thar.ConvertHAR(&testHar, id, workSpaceID)
	if err != nil {
		t.Errorf("Error converting HAR: %v", err)
	}

	if len(resolved.Apis) != 1 {
		t.Errorf("Expected 1 API, got %d", len(resolved.Apis))
	}
}

func TestHarResvoledBodyRaw(t *testing.T) {
	Entry := thar.Entry{}
	Entry.Request.Method = "GET"
	Entry.Request.URL = "http://example.com"
	Entry.Request.HTTPVersion = "HTTP/1.1"
	Entry.Request.Headers = []thar.Header{
		{Name: "Content-Type", Value: "application/json"},
	}
	testHar := thar.HAR{
		Log: thar.Log{
			Entries: []thar.Entry{Entry, Entry},
		},
	}
	id := idwrap.NewNow()
	workSpaceID := idwrap.NewNow()

	resolved, err := thar.ConvertHAR(&testHar, id, workSpaceID)
	if err != nil {
		t.Errorf("Error converting HAR: %v", err)
	}

	if len(resolved.Apis) != 1 {
		t.Errorf("Expected 1 API, got %d", len(resolved.Apis))
	}
	if len(resolved.RawBodies) != 3 {
		t.Errorf("Expected 3 Raw Body, got %d", len(resolved.RawBodies))
	}

	for i, rawBody := range resolved.RawBodies {
		if !bytes.Equal(rawBody.Data, []byte{}) {
			t.Errorf("Expected empty body, got %s", rawBody.Data)
		}

		var found bool
		for examples := range resolved.Examples {
			if rawBody.ExampleID != resolved.Examples[examples].ID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected ExampleID to be %s, got %s", resolved.Examples[i].ID, rawBody.ExampleID)
		}
	}
}

func TestHarResvoledBodyForm(t *testing.T) {
	Entry := thar.Entry{}
	Entry.Request.Method = "GET"
	Entry.Request.URL = "http://example.com"
	Entry.Request.HTTPVersion = "HTTP/1.1"
	Entry.Request.Headers = []thar.Header{
		{Name: "Content-Type", Value: "application/json"},
	}
	Entry.Request.PostData = &thar.PostData{}
	Entry.Request.PostData.MimeType = thar.FormBodyCheck
	Entry.Request.PostData.Params = []thar.Param{
		{Name: "name", Value: "value"},
	}

	testHar := thar.HAR{
		Log: thar.Log{
			Entries: []thar.Entry{Entry, Entry},
		},
	}
	id := idwrap.NewNow()
	workSpaceID := idwrap.NewNow()

	resolved, err := thar.ConvertHAR(&testHar, id, workSpaceID)
	if err != nil {
		t.Errorf("Error converting HAR: %v", err)
	}

	if len(resolved.Apis) != 1 {
		t.Errorf("Expected 1 API, got %d", len(resolved.Apis))
	}
	if len(resolved.RawBodies) != 3 {
		t.Errorf("Expected 3 Raw Body, got %d", len(resolved.RawBodies))
	}

	for i, rawBody := range resolved.RawBodies {
		if !bytes.Equal(rawBody.Data, []byte{}) {
			t.Errorf("Expected empty body, got %s", rawBody.Data)
		}

		var found bool
		for examples := range resolved.Examples {
			if rawBody.ExampleID != resolved.Examples[examples].ID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected ExampleID to be %s, got %s", resolved.Examples[i].ID, rawBody.ExampleID)
		}
	}

	if len(resolved.FormBodies) != 2 {
		t.Errorf("Expected 4 Form Body, got %d", len(resolved.FormBodies))
	}
}

func TestHarResvoledBodyUrlEncoded(t *testing.T) {
	Entry := thar.Entry{}
	Entry.Request.Method = "GET"
	Entry.Request.URL = "http://example.com"
	Entry.Request.HTTPVersion = "HTTP/1.1"
	Entry.Request.Headers = []thar.Header{
		{Name: "Content-Type", Value: "application/json"},
	}
	Entry.Request.PostData = &thar.PostData{}
	Entry.Request.PostData.MimeType = thar.UrlEncodedBodyCheck
	Entry.Request.PostData.Params = []thar.Param{
		{Name: "name", Value: "value"},
	}

	testHar := thar.HAR{
		Log: thar.Log{
			Entries: []thar.Entry{Entry, Entry},
		},
	}
	id := idwrap.NewNow()
	workSpaceID := idwrap.NewNow()

	resolved, err := thar.ConvertHAR(&testHar, id, workSpaceID)
	if err != nil {
		t.Errorf("Error converting HAR: %v", err)
	}

	if len(resolved.Apis) != 1 {
		t.Errorf("Expected 1 API, got %d", len(resolved.Apis))
	}
	if len(resolved.RawBodies) != 3 {
		t.Errorf("Expected 3 Raw Body, got %d", len(resolved.RawBodies))
	}

	for i, rawBody := range resolved.RawBodies {
		if !bytes.Equal(rawBody.Data, []byte{}) {
			t.Errorf("Expected empty body, got %s", rawBody.Data)
		}

		var found bool
		for examples := range resolved.Examples {
			if rawBody.ExampleID != resolved.Examples[examples].ID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected ExampleID to be %s, got %s", resolved.Examples[i].ID, rawBody.ExampleID)
		}
	}

	if len(resolved.UrlEncodedBodies) != 2 {
		t.Errorf("Expected 4 Form Body, got %d", len(resolved.FormBodies))
	}
}

func TestHarEmptyLog(t *testing.T) {
	testHar := thar.HAR{
		Log: thar.Log{
			Entries: []thar.Entry{},
		},
	}
	id := idwrap.NewNow()
	workSpaceID := idwrap.NewNow()

	resolved, err := thar.ConvertHAR(&testHar, id, workSpaceID)
	if err == nil {
		t.Errorf("Expected error converting HAR")
	}

	if len(resolved.Apis) != 0 {
		t.Errorf("Expected 0 APIs, got %d", len(resolved.Apis))
	}

	if len(resolved.Examples) != 0 {
		t.Errorf("Expected 0 Examples, got %d", len(resolved.Examples))
	}
}

func TestHarUnknownMimeType(t *testing.T) {
	entry := thar.Entry{}
	entry.Request.Method = "POST"
	entry.Request.URL = "http://example.com/api"
	entry.Request.HTTPVersion = "HTTP/1.1"
	entry.Request.Headers = []thar.Header{
		{Name: "Content-Type", Value: "application/json"},
	}
	entry.Request.PostData = &thar.PostData{
		MimeType: "unknown/type",
		Params: []thar.Param{
			{Name: "param1", Value: "test"},
		},
	}

	testHar := thar.HAR{
		Log: thar.Log{
			Entries: []thar.Entry{entry},
		},
	}
	id := idwrap.NewNow()
	workSpaceID := idwrap.NewNow()

	resolved, err := thar.ConvertHAR(&testHar, id, workSpaceID)
	if err != nil {
		t.Errorf("Error converting HAR: %v", err)
	}

	// Assuming that an unknown MIME type is treated as a raw body.
	// Given previous tests, one entry produces 2 raw bodies.
	if len(resolved.RawBodies) != 3 {
		t.Errorf("Expected 3 Raw Bodies, got %d", len(resolved.RawBodies))
	}

	// Verify that the bodies are empty.
	for i, rawBody := range resolved.RawBodies {
		if !bytes.Equal(rawBody.Data, []byte{}) {
			t.Errorf("Expected empty body, got %s", rawBody.Data)
		}

		var found bool
		for examples := range resolved.Examples {
			if rawBody.ExampleID != resolved.Examples[examples].ID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected ExampleID to be %s, got %s", resolved.Examples[i].ID, rawBody.ExampleID)
		}
	}
}

func TestHarDiverseEntries(t *testing.T) {
	// Entry 1: GET without post data.
	entry1 := thar.Entry{}
	entry1.Request.Method = "GET"
	entry1.Request.URL = "http://example.com"
	entry1.Request.HTTPVersion = "HTTP/1.1"
	entry1.Request.Headers = []thar.Header{
		{Name: "Content-Type", Value: "application/json"},
	}

	// Entry 2: POST with form body.
	entry2 := thar.Entry{}
	entry2.Request.Method = "POST"
	entry2.Request.URL = "http://example.com/submit"
	entry2.Request.HTTPVersion = "HTTP/1.1"
	entry2.Request.Headers = []thar.Header{
		{Name: "Content-Type", Value: "application/json"},
	}
	entry2.Request.PostData = &thar.PostData{
		MimeType: thar.FormBodyCheck,
		Params:   []thar.Param{{Name: "username", Value: "admin"}},
	}

	// Entry 3: POST with urlencoded body.
	entry3 := thar.Entry{}
	entry3.Request.Method = "POST"
	entry3.Request.URL = "http://example.com/login"
	entry3.Request.HTTPVersion = "HTTP/1.1"
	entry3.Request.Headers = []thar.Header{
		{Name: "Content-Type", Value: "application/json"},
	}
	entry3.Request.PostData = &thar.PostData{
		MimeType: thar.UrlEncodedBodyCheck,
		Params:   []thar.Param{{Name: "user", Value: "admin"}},
	}

	testHar := thar.HAR{
		Log: thar.Log{
			Entries: []thar.Entry{entry1, entry2, entry3},
		},
	}
	id := idwrap.NewNow()
	workSpaceID := idwrap.NewNow()

	resolved, err := thar.ConvertHAR(&testHar, id, workSpaceID)
	if err != nil {
		t.Errorf("Error converting HAR: %v", err)
	}

	// Expect one API per entry.
	if len(resolved.Apis) != 3 {
		t.Errorf("Expected 3 APIs, got %d", len(resolved.Apis))
	}

	// According to previous tests each entry creates 2 raw bodies.
	if len(resolved.RawBodies) != 9 {
		t.Errorf("Expected 9 Raw Bodies, got %d", len(resolved.RawBodies))
	}

	// Verify that GET (entry1) did not produce form or URL encoded bodies.
	// Adjust counts based on your conversion logic; here we assume each POST produces 2 bodies
	// specific to their MIME type.
	if len(resolved.FormBodies) != 2 {
		t.Errorf("Expected 2 Form Bodies, got %d", len(resolved.FormBodies))
	}

	if len(resolved.UrlEncodedBodies) != 2 {
		t.Errorf("Expected 2 UrlEncoded Bodies, got %d", len(resolved.FormBodies))
	}
}

func TestHarResolvedNewFields(t *testing.T) {
	entry := thar.Entry{}
	entry.Request.Method = "GET"
	entry.Request.URL = "http://example.com/flow"
	entry.Request.HTTPVersion = "HTTP/1.1"
	entry.Request.Headers = []thar.Header{
		{Name: "Content-Type", Value: "application/json"},
	}

	testHar := thar.HAR{
		Log: thar.Log{
			Entries: []thar.Entry{entry},
		},
	}

	id := idwrap.NewNow()
	workSpaceID := idwrap.NewNow()

	resolved, err := thar.ConvertHAR(&testHar, id, workSpaceID)
	if err != nil {
		t.Errorf("Error converting HAR: %v", err)
	}

	// Check that the Flow field is populated.
	// TODO: change the check if mflow.Flow uses a different zero value.
	if resolved.Flow == (mflow.Flow{}) {
		t.Errorf("Expected Flow to be populated")
	}

	if len(resolved.Nodes) != 2 {
		t.Errorf("Expected 2 Node, got %d", len(resolved.Nodes))
	}

	if len(resolved.RequestNodes) != 1 {
		t.Errorf("Expected 1 Request, got %d", len(resolved.RequestNodes))
	}
}

func TestHarResolvedDeepFields(t *testing.T) {
	// Prepare a basic HAR entry.
	entry := thar.Entry{}
	entry.Request.Method = "GET"
	entry.Request.URL = "http://example.com/flow"
	entry.Request.HTTPVersion = "HTTP/1.1"
	entry.Request.Headers = []thar.Header{
		{Name: "Content-Type", Value: "application/json"},
	}

	testHar := thar.HAR{
		Log: thar.Log{
			Entries: []thar.Entry{entry},
		},
	}

	// Create IDs.
	id := idwrap.NewNow()
	workSpaceID := idwrap.NewNow()

	// Convert HAR.
	resolved, err := thar.ConvertHAR(&testHar, id, workSpaceID)
	if err != nil {
		t.Fatalf("Error converting HAR: %v", err)
	}

	// Verify Flow is not zero.
	if resolved.Flow == (mflow.Flow{}) {
		t.Error("Expected Flow to be populated")
	}

	// Verify we have a single node and request.
	if len(resolved.Nodes) != 2 {
		t.Fatalf("Expected 2 Node, got %d", len(resolved.Nodes))
	}
	if len(resolved.RequestNodes) != 1 {
		t.Fatalf("Expected 1 Request, got %d", len(resolved.RequestNodes))
	}

	// TODO: refactor this test

	apiID := resolved.Apis[0].ID
	requestNode := resolved.RequestNodes[0]
	if requestNode.EndpointID == nil {
		t.Fatalf("Expected Request Node to be populated")
	}
	if requestNode.ExampleID == nil {
		t.Fatalf("Expected Request Node to be populated")
	}

	// Deep checks on the Request.
	if *requestNode.EndpointID != apiID {
		t.Errorf("Expected Request APIID to be %v, got %v", apiID, resolved.RequestNodes[0].EndpointID)
	}

	// Verify that the bodies are empty.
	for i, rawBody := range resolved.RawBodies {
		if !bytes.Equal(rawBody.Data, []byte{}) {
			t.Errorf("Expected empty body, got %s", rawBody.Data)
		}

		var found bool
		for examples := range resolved.Examples {
			if rawBody.ExampleID != resolved.Examples[examples].ID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected ExampleID to be %s, got %s", resolved.Examples[i].ID, rawBody.ExampleID)
		}
	}
}

func TestHarSortEntriesByStartedTime(t *testing.T) {
	// Create two entries with different start times.
	// entry1 starts later.
	entry1 := thar.Entry{
		StartedDateTime: time.Date(2023, 10, 12, 12, 0, 0, 0, time.UTC),
		Request: thar.Request{
			Method:      "GET",
			URL:         "http://example.com/second",
			HTTPVersion: "HTTP/1.1",
			Headers:     []thar.Header{},
		},
	}
	// entry2 starts earlier.
	entry2 := thar.Entry{
		StartedDateTime: time.Date(2023, 10, 12, 10, 0, 0, 0, time.UTC),
		Request: thar.Request{
			Method:      "GET",
			URL:         "http://example.com/first",
			HTTPVersion: "HTTP/1.1",
			Headers:     []thar.Header{},
		},
	}

	testHar := thar.HAR{
		Log: thar.Log{
			Entries: []thar.Entry{entry1, entry2},
		},
	}
	id := idwrap.NewNow()
	workSpaceID := idwrap.NewNow()

	resolved, err := thar.ConvertHAR(&testHar, id, workSpaceID)
	if err != nil {
		t.Errorf("Error converting HAR: %v", err)
	}

	expectedFlowName := "http://example.com/first"
	if resolved.Flow.Name != expectedFlowName {
		t.Errorf("Expected Flow.Name %s, got %s", expectedFlowName, resolved.Flow.Name)
	}
}

func TestHarItemApiExampleRelationship(t *testing.T) {
	// Create a test HAR with multiple entries
	entries := []thar.Entry{
		{
			StartedDateTime: time.Now(),
			Request: thar.Request{
				Method:      "GET",
				URL:         "http://example.com/api1",
				HTTPVersion: "HTTP/1.1",
				Headers: []thar.Header{
					{Name: "Content-Type", Value: "application/json"},
				},
			},
		},
		{
			StartedDateTime: time.Now().Add(time.Second),
			Request: thar.Request{
				Method:      "POST",
				URL:         "http://example.com/api2",
				HTTPVersion: "HTTP/1.1",
				Headers: []thar.Header{
					{Name: "Content-Type", Value: "application/json"},
				},
			},
		},
	}

	testHar := thar.HAR{
		Log: thar.Log{
			Entries: entries,
		},
	}

	// Convert HAR
	id := idwrap.NewNow()
	workSpaceID := idwrap.NewNow()
	resolved, err := thar.ConvertHAR(&testHar, id, workSpaceID)
	if err != nil {
		t.Fatalf("Error converting HAR: %v", err)
	}

	// Create a map of API IDs to APIs for easy lookup
	apiMap := make(map[string]mitemapi.ItemApi)
	for _, api := range resolved.Apis {
		key := api.Method + " " + api.Url
		apiMap[key] = api
	}

	// Verify each example has correct API ID
	for _, example := range resolved.Examples {
		// Find the corresponding API
		var foundAPI *mitemapi.ItemApi
		for _, api := range resolved.Apis {
			if api.ID == example.ItemApiID {
				foundAPI = &api
				break
			}
		}

		// Check if we found a matching API
		if foundAPI == nil {
			t.Errorf("Example %s has ItemApiID %s which doesn't match any API",
				example.Name, example.ItemApiID)
			continue
		}

		// TODO: add case for delta name check
		// Verify the relationship is correct
		/*
			if foundAPI.Name != example.Name {
				t.Errorf("Example name mismatch: expected %s, got %s",
					foundAPI.Name, example.Name)
			}
		*/

		// Verify that each API has exactly two examples (default and non-default)
		examplesForAPI := 0
		var hasDefault, hasNonDefault bool
		for _, ex := range resolved.Examples {
			if ex.ItemApiID == foundAPI.ID {
				examplesForAPI++
				if ex.IsDefault {
					hasDefault = true
				} else {
					hasNonDefault = true
				}
			}
		}

		if examplesForAPI != 3 {
			t.Errorf("API %s should have exactly 2 examples, got %d",
				foundAPI.Name, examplesForAPI)
		}

		if !hasDefault {
			t.Errorf("API %s is missing default example", foundAPI.Name)
		}

		if !hasNonDefault {
			t.Errorf("API %s is missing non-default example", foundAPI.Name)
		}
	}

	// Verify the total number of examples is twice the number of APIs
	expectedExampleCount := len(resolved.Apis) * 3 // Each API has default and non-default example
	if len(resolved.Examples) != expectedExampleCount {
		t.Errorf("Expected %d total examples, got %d",
			expectedExampleCount, len(resolved.Examples))
	}
}

func TestHarUniqueIDs(t *testing.T) {
	// Create test HAR entries
	entries := []thar.Entry{
		{
			StartedDateTime: time.Now(),
			Request: thar.Request{
				Method:      "GET",
				URL:         "http://example.com/api1",
				HTTPVersion: "HTTP/1.1",
				Headers: []thar.Header{
					{Name: "Content-Type", Value: "application/json"},
				},
			},
		},
		{
			StartedDateTime: time.Now().Add(time.Second),
			Request: thar.Request{
				Method:      "POST",
				URL:         "http://example.com/api2",
				HTTPVersion: "HTTP/1.1",
				Headers: []thar.Header{
					{Name: "Content-Type", Value: "application/json"},
				},
			},
		},
	}

	testHar := thar.HAR{
		Log: thar.Log{
			Entries: entries,
		},
	}

	// Convert HAR
	id := idwrap.NewNow()
	workSpaceID := idwrap.NewNow()
	resolved, err := thar.ConvertHAR(&testHar, id, workSpaceID)
	if err != nil {
		t.Fatalf("Error converting HAR: %v", err)
	}

	// Create maps to track unique IDs
	apiIDs := make(map[string]bool)
	exampleIDs := make(map[string]bool)

	// Check for unique API IDs
	for _, api := range resolved.Apis {
		// Check if API ID already exists
		if apiIDs[api.ID.String()] {
			t.Errorf("Duplicate API ID found: %s", api.ID)
		}
		apiIDs[api.ID.String()] = true

		// Verify API ID is not used in any example
		for _, example := range resolved.Examples {
			if api.ID == example.ID {
				t.Errorf("API ID %s is also used as Example ID", api.ID)
			}
		}
	}

	// Check for unique Example IDs
	for _, example := range resolved.Examples {
		// Check if Example ID already exists
		if exampleIDs[example.ID.String()] {
			t.Errorf("Duplicate Example ID found: %s", example.ID)
		}
		exampleIDs[example.ID.String()] = true

		// Verify Example ID is not used in any API
		for _, api := range resolved.Apis {
			if example.ID == api.ID {
				t.Errorf("Example ID %s is also used as API ID", example.ID)
			}
		}

		// Verify Example's ItemApiID exists in APIs
		var found bool
		for _, api := range resolved.Apis {
			if example.ItemApiID == api.ID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Example %s references non-existent API ID %s",
				example.ID, example.ItemApiID)
		}
	}

	// Print all IDs for debugging
	if t.Failed() {
		t.Log("API IDs:")
		for _, api := range resolved.Apis {
			t.Logf("API: %s - %s", api.ID, api.Name)
		}
		t.Log("Example IDs:")
		for _, example := range resolved.Examples {
			t.Logf("Example: %s - %s (API: %s)",
				example.ID, example.Name, example.ItemApiID)
		}
	}
}

func TestNodePositioning(t *testing.T) {
	// Create a test flow with a branching structure:
	//
	// Start
	//   |
	//   +-------+--------+
	//   |       |        |
	// Branch1 Branch2  Branch3
	//   |       |        |
	//   |     Child1     |
	//   |       |        |
	//   |     Child2     |
	//   |                |
	//   +-------+
	//           |
	//          (Implicit End)

	// Create a HarResvoled with a simple flow structure
	result := thar.HarResvoled{
		Flow: mflow.Flow{
			ID:   idwrap.NewNow(),
			Name: "Test Flow",
		},
		Nodes:     []mnnode.MNode{},
		NoopNodes: []mnnoop.NoopNode{},
		Edges:     []edge.Edge{},
	}

	// Create node IDs
	startID := idwrap.NewNow()
	branch1ID := idwrap.NewNow()
	branch2ID := idwrap.NewNow()
	branch3ID := idwrap.NewNow()
	child1ID := idwrap.NewNow()
	child2ID := idwrap.NewNow()

	// Add nodes
	result.Nodes = append(result.Nodes,
		mnnode.MNode{ID: startID, Name: "Start"},
		mnnode.MNode{ID: branch1ID, Name: "Branch1"},
		mnnode.MNode{ID: branch2ID, Name: "Branch2"},
		mnnode.MNode{ID: branch3ID, Name: "Branch3"},
		mnnode.MNode{ID: child1ID, Name: "Child1"},
		mnnode.MNode{ID: child2ID, Name: "Child2"},
	)

	// Add start noop node
	result.NoopNodes = append(result.NoopNodes,
		mnnoop.NoopNode{
			Type:       mnnoop.NODE_NO_OP_KIND_START,
			FlowNodeID: startID,
		},
	)

	// Add edges
	result.Edges = append(result.Edges,
		edge.Edge{SourceID: startID, TargetID: branch1ID},
		edge.Edge{SourceID: startID, TargetID: branch2ID},
		edge.Edge{SourceID: startID, TargetID: branch3ID},
		edge.Edge{SourceID: branch2ID, TargetID: child1ID},
		edge.Edge{SourceID: child1ID, TargetID: child2ID},
	)

	// Run the positioning function
	err := thar.ReorganizeNodePositions(&result)
	if err != nil {
		t.Fatal(err)
	}

	// Create a node map for easy lookup
	nodeMap := make(map[string]*mnnode.MNode)
	for i := range result.Nodes {
		nodeMap[result.Nodes[i].Name] = &result.Nodes[i]
	}

	// Verify start node is at origin
	if nodeMap["Start"].PositionX != 0 || nodeMap["Start"].PositionY != 0 {
		t.Errorf("Start node not at origin: (%f, %f)", nodeMap["Start"].PositionX, nodeMap["Start"].PositionY)
	}

	// Verify branch nodes are at the same Y level
	branchY := nodeMap["Branch1"].PositionY
	if nodeMap["Branch2"].PositionY != branchY || nodeMap["Branch3"].PositionY != branchY {
		t.Errorf("Branch nodes not at same Y level: Branch1=%f, Branch2=%f, Branch3=%f",
			nodeMap["Branch1"].PositionY, nodeMap["Branch2"].PositionY, nodeMap["Branch3"].PositionY)
	}

	// Verify that branches are horizontally spread out
	if !(nodeMap["Branch1"].PositionX < nodeMap["Branch2"].PositionX &&
		nodeMap["Branch2"].PositionX < nodeMap["Branch3"].PositionX) {
		t.Errorf("Branch nodes not properly spaced horizontally: Branch1=%.1f, Branch2=%.1f, Branch3=%.1f",
			nodeMap["Branch1"].PositionX, nodeMap["Branch2"].PositionX, nodeMap["Branch3"].PositionX)
	}

	// Verify child nodes form a vertical chain
	if nodeMap["Child1"].PositionY <= nodeMap["Branch2"].PositionY ||
		nodeMap["Child2"].PositionY <= nodeMap["Child1"].PositionY {
		t.Errorf("Child nodes not in vertical chain: Branch2=%f, Child1=%f, Child2=%f",
			nodeMap["Branch2"].PositionY, nodeMap["Child1"].PositionY, nodeMap["Child2"].PositionY)
	}

	// Verify child nodes are aligned with their parent
	if nodeMap["Child1"].PositionX != nodeMap["Branch2"].PositionX ||
		nodeMap["Child2"].PositionX != nodeMap["Child1"].PositionX {
		t.Errorf("Child nodes not aligned with parent: Branch2=%.1f, Child1=%.1f, Child2=%.1f",
			nodeMap["Branch2"].PositionX, nodeMap["Child1"].PositionX, nodeMap["Child2"].PositionX)
	}
}

func TestPositionNodesWithDifferentTopologies(t *testing.T) {
	t.Run("linear chain", func(t *testing.T) {
		// Create a linear chain: Start -> Node1 -> Node2 -> Node3
		result := thar.HarResvoled{
			Flow:      mflow.Flow{ID: idwrap.NewNow(), Name: "Linear Chain"},
			Nodes:     []mnnode.MNode{},
			NoopNodes: []mnnoop.NoopNode{},
			Edges:     []edge.Edge{},
		}

		startID := idwrap.NewNow()
		node1ID := idwrap.NewNow()
		node2ID := idwrap.NewNow()
		node3ID := idwrap.NewNow()

		// Add nodes
		result.Nodes = append(result.Nodes,
			mnnode.MNode{ID: startID, Name: "Start"},
			mnnode.MNode{ID: node1ID, Name: "Node1"},
			mnnode.MNode{ID: node2ID, Name: "Node2"},
			mnnode.MNode{ID: node3ID, Name: "Node3"},
		)

		// Add start noop node
		result.NoopNodes = append(result.NoopNodes,
			mnnoop.NoopNode{Type: mnnoop.NODE_NO_OP_KIND_START, FlowNodeID: startID},
		)

		// Add edges for linear chain
		result.Edges = append(result.Edges,
			edge.Edge{SourceID: startID, TargetID: node1ID},
			edge.Edge{SourceID: node1ID, TargetID: node2ID},
			edge.Edge{SourceID: node2ID, TargetID: node3ID},
		)

		// Run the positioning function
		err := thar.ReorganizeNodePositions(&result)
		if err != nil {
			t.Fatal(err)
		}

		// Create a node map for lookup
		nodeMap := make(map[string]*mnnode.MNode)
		for i := range result.Nodes {
			nodeMap[result.Nodes[i].Name] = &result.Nodes[i]
		}

		// Verify start node is at origin
		if nodeMap["Start"].PositionX != 0 || nodeMap["Start"].PositionY != 0 {
			t.Errorf("Start node not at origin: (%f, %f)", nodeMap["Start"].PositionX, nodeMap["Start"].PositionY)
		}

		// Verify nodes form a vertical chain with constant X
		if nodeMap["Node1"].PositionY <= nodeMap["Start"].PositionY ||
			nodeMap["Node2"].PositionY <= nodeMap["Node1"].PositionY ||
			nodeMap["Node3"].PositionY <= nodeMap["Node2"].PositionY {
			t.Errorf("Nodes not in vertical chain: Start=%f, Node1=%f, Node2=%f, Node3=%f",
				nodeMap["Start"].PositionY, nodeMap["Node1"].PositionY, nodeMap["Node2"].PositionY, nodeMap["Node3"].PositionY)
		}

		// Verify X positions are the same (vertical alignment)
		if nodeMap["Node1"].PositionX != nodeMap["Start"].PositionX ||
			nodeMap["Node2"].PositionX != nodeMap["Start"].PositionX ||
			nodeMap["Node3"].PositionX != nodeMap["Start"].PositionX {
			t.Errorf("Nodes not aligned vertically: Start=%.1f, Node1=%.1f, Node2=%.1f, Node3=%.1f",
				nodeMap["Start"].PositionX, nodeMap["Node1"].PositionX, nodeMap["Node2"].PositionX, nodeMap["Node3"].PositionX)
		}
	})
}

func TestHarTokenMatching(t *testing.T) {
	// Create a test HAR with entries containing tokens
	entries := []thar.Entry{
		{
			StartedDateTime: time.Now(),
			Request: thar.Request{
				Method:      "GET",
				URL:         "http://example.com/api1",
				HTTPVersion: "HTTP/1.1",
				Headers: []thar.Header{
					{Name: "Authorization", Value: "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6MSwiaWF0IjoxNzQ4MTg1MDkwLCJleHAiOjE3NDgyNzE0OTB9.TG4reOVX09bjGnB04xuYH0HrdfMcKn9vq03mG2aGa7Q"},
				},
			},
		},
		{
			StartedDateTime: time.Now().Add(time.Second),
			Request: thar.Request{
				Method:      "POST",
				URL:         "http://example.com/api2",
				HTTPVersion: "HTTP/1.1",
				Headers: []thar.Header{
					{Name: "Authorization", Value: "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6MSwiaWF0IjoxNzQ4MTg1MDkwLCJleHAiOjE3NDgyNzE0OTB9.TG4reOVX09bjGnB04xuYH0HrdfMcKn9vq03mG2aGa7Q"},
				},
				PostData: &thar.PostData{
					MimeType: "application/json",
					Text:     `{"token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6MSwiaWF0IjoxNzQ4MTg1MDkwLCJleHAiOjE3NDgyNzE0OTB9.TG4reOVX09bjGnB04xuYH0HrdfMcKn9vq03mG2aGa7Q"}`,
				},
			},
		},
	}

	testHar := thar.HAR{
		Log: thar.Log{
			Entries: entries,
		},
	}

	// Convert HAR
	id := idwrap.NewNow()
	workSpaceID := idwrap.NewNow()
	resolved, err := thar.ConvertHAR(&testHar, id, workSpaceID)
	if err != nil {
		t.Fatalf("Error converting HAR: %v", err)
	}

	// Verify headers were processed correctly
	for _, example := range resolved.Examples {
		var authHeader *mexampleheader.Header
		for _, header := range resolved.Headers {
			if header.ExampleID == example.ID && header.HeaderKey == "Authorization" {
				authHeader = &header
				break
			}
		}

		if authHeader == nil {
			t.Errorf("Authorization header not found for example %s", example.ID)
			continue
		}

		// Verify the header value contains the template variable
		if !strings.Contains(authHeader.Value, "{{") || !strings.Contains(authHeader.Value, "}}") {
			t.Errorf("Authorization header value does not contain template variable: %s", authHeader.Value)
		}

		// Verify Bearer prefix is preserved
		if !strings.HasPrefix(authHeader.Value, "Bearer {{") {
			t.Errorf("Authorization header value does not preserve Bearer prefix: %s", authHeader.Value)
		}
	}

	// Verify request body was processed correctly
	for _, example := range resolved.Examples {
		var rawBody *mbodyraw.ExampleBodyRaw
		for _, body := range resolved.RawBodies {
			if body.ExampleID == example.ID {
				rawBody = &body
				break
			}
		}

		if rawBody == nil {
			t.Errorf("Raw body not found for example %s", example.ID)
			continue
		}

		// Convert raw body to string for checking
		bodyStr := string(rawBody.Data)
		if strings.Contains(bodyStr, "token") {
			// Verify the token value contains the template variable
			if !strings.Contains(bodyStr, "{{") || !strings.Contains(bodyStr, "}}") {
				t.Errorf("Request body does not contain template variable: %s", bodyStr)
			}
		}
	}
}

func TestHarTokenReplacementFromRealHar(t *testing.T) {
	// Use a dummy HAR JSON for testing
	dummyHAR := `{
		"log": {
			"entries": [
				{
					"startedDateTime": "2023-01-01T00:00:00.000Z",
					"request": {
						"method": "POST",
						"url": "http://example.com/login",
						"httpVersion": "HTTP/1.1",
						"headers": [
							{ "name": "Content-Type", "value": "application/json" }
						],
						"postData": {
							"mimeType": "application/json",
							"text": "{\"username\":\"user\",\"password\":\"pass\"}"
						}
					},
					"response": {
						"status": 200,
						"statusText": "OK",
						"httpVersion": "HTTP/1.1",
						"headers": [
							{ "name": "Content-Type", "value": "application/json" }
						],
						"content": {
							"size": 100,
							"mimeType": "application/json",
							"text": "{\"token\":\"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6MSwiaWF0IjoxNzQ4MTg1MDkwLCJleHAiOjE3NDgyNzE0OTB9.TG4reOVX09bjGnB04xuYH0HrdfMcKn9vq03mG2aGa7Q\"}"
						}
					}
				},
				{
					"startedDateTime": "2023-01-01T00:00:01.000Z",
					"request": {
						"method": "GET",
						"url": "http://example.com/api",
						"httpVersion": "HTTP/1.1",
						"headers": [
							{ "name": "Authorization", "value": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6MSwiaWF0IjoxNzQ4MTg1MDkwLCJleHAiOjE3NDgyNzE0OTB9.TG4reOVX09bjGnB04xuYH0HrdfMcKn9vq03mG2aGa7Q" }
						]
					},
					"response": {
						"status": 200,
						"statusText": "OK",
						"httpVersion": "HTTP/1.1",
						"headers": [
							{ "name": "Content-Type", "value": "application/json" }
						],
						"content": {
							"size": 50,
							"mimeType": "application/json",
							"text": "{\"data\":\"success\"}"
						}
					}
				}
			]
		}
	}`

	var har thar.HAR
	if err := json.Unmarshal([]byte(dummyHAR), &har); err != nil {
		t.Fatalf("Failed to unmarshal dummy HAR: %v", err)
	}

	// Add the known token to depFinder
	depFinder := depfinder.NewDepFinder()
	token := "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6MSwiaWF0IjoxNzQ4MTg1MDkwLCJleHAiOjE3NDgyNzE0OTB9.TG4reOVX09bjGnB04xuYH0HrdfMcKn9vq03mG2aGa7Q"
	path := "auth.token"
	nodeID := idwrap.NewNow()
	couple := depfinder.VarCouple{Path: path, NodeID: nodeID}
	depFinder.AddVar(token, couple)
	// Also add the raw token without 'Bearer ' prefix
	rawToken := strings.TrimPrefix(token, "Bearer ")
	rawToken = strings.TrimSpace(rawToken)
	depFinder.AddVar(rawToken, couple)

	// Convert HAR with token replacement
	result, err := thar.ConvertHARWithDepFinder(&har, idwrap.NewNow(), idwrap.NewNow(), &depFinder)
	if err != nil {
		t.Fatalf("Failed to convert HAR: %v", err)
	}

	// Check all headers for token replacement
	for _, header := range result.Headers {
		if strings.ToLower(header.HeaderKey) == "authorization" {
			if !strings.Contains(header.Value, "{{ auth.token }}") {
				t.Errorf("Token not replaced in Authorization header: %s", header.Value)
			}
		}
	}

	// Check all response bodies for token replacement
	for _, body := range result.RawBodies {
		var bodyObj map[string]interface{}
		if err := json.Unmarshal(body.Data, &bodyObj); err == nil {
			if tokenVal, ok := bodyObj["token"].(string); ok && !strings.Contains(tokenVal, "{{ auth.token }}") {
				t.Errorf("Token not replaced in response body: %s", tokenVal)
			}
		}
	}
}
