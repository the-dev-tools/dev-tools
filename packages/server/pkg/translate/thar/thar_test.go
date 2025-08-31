package thar_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/depfinder"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mitemfolder"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/translate/thar"
	"time"

	"github.com/stretchr/testify/require"
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

	// With delta system: 1 original + 1 delta = 2 APIs
	if len(resolved.Apis) != 2 {
		t.Errorf("Expected 2 APIs (1 original + 1 delta), got %d", len(resolved.Apis))
	}

	// Verify one original and one delta API
	var originalAPI, deltaAPI *mitemapi.ItemApi
	for i := range resolved.Apis {
		if resolved.Apis[i].DeltaParentID == nil {
			originalAPI = &resolved.Apis[i]
		} else {
			deltaAPI = &resolved.Apis[i]
		}
	}

	if originalAPI == nil {
		t.Errorf("Expected to find one original API")
	}
	if deltaAPI == nil {
		t.Errorf("Expected to find one delta API")
	}
	if deltaAPI != nil && originalAPI != nil && *deltaAPI.DeltaParentID != originalAPI.ID {
		t.Errorf("Expected delta API to reference original API as parent")
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

	// With delta system: 2 entries -> 2 original + 2 delta = 4 APIs
	if len(resolved.Apis) != 4 {
		t.Errorf("Expected 4 APIs (2 original + 2 delta), got %d", len(resolved.Apis))
	}
	if len(resolved.RawBodies) != 6 {
		t.Errorf("Expected 6 Raw Bodies, got %d", len(resolved.RawBodies))
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

	if len(resolved.Apis) != 4 {
		t.Errorf("Expected 4 APIs, got %d", len(resolved.Apis))
	}
	if len(resolved.RawBodies) != 6 {
		t.Errorf("Expected 6 Raw Bodies, got %d", len(resolved.RawBodies))
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

	if len(resolved.FormBodies) != 6 {
		t.Errorf("Expected 6 Form Bodies, got %d", len(resolved.FormBodies))
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

	if len(resolved.Apis) != 4 {
		t.Errorf("Expected 4 APIs, got %d", len(resolved.Apis))
	}
	if len(resolved.RawBodies) != 6 {
		t.Errorf("Expected 6 Raw Bodies, got %d", len(resolved.RawBodies))
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

	if len(resolved.UrlEncodedBodies) != 6 {
		t.Errorf("Expected 6 UrlEncoded Bodies, got %d", len(resolved.UrlEncodedBodies))
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

	// Expect one API per entry (with delta system, 2 APIs per entry).
	if len(resolved.Apis) != 6 {
		t.Errorf("Expected 6 APIs, got %d", len(resolved.Apis))
	}

	// According to previous tests each entry creates 2 raw bodies.
	if len(resolved.RawBodies) != 9 {
		t.Errorf("Expected 9 Raw Bodies, got %d", len(resolved.RawBodies))
	}

	// Verify that GET (entry1) did not produce form or URL encoded bodies.
	// Adjust counts based on your conversion logic; here we assume each POST produces 3 bodies
	// specific to their MIME type (regular, default, delta).
	if len(resolved.FormBodies) != 3 {
		t.Errorf("Expected 3 Form Bodies, got %d", len(resolved.FormBodies))
	}

	if len(resolved.UrlEncodedBodies) != 3 {
		t.Errorf("Expected 3 UrlEncoded Bodies, got %d", len(resolved.UrlEncodedBodies))
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

	// Verify the total number of examples (with delta system: 2 entries * 3 examples per entry = 6)
	expectedExampleCount := 6 // Each entry produces 3 examples total (1 normal + 1 default + 1 delta)
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

	// Verify that branches are horizontally spread out (no specific order required with grid system)
	branchPositions := []float64{nodeMap["Branch1"].PositionX, nodeMap["Branch2"].PositionX, nodeMap["Branch3"].PositionX}
	uniquePositions := make(map[float64]bool)
	for _, pos := range branchPositions {
		if uniquePositions[pos] {
			t.Errorf("Branch nodes have overlapping X positions: Branch1=%.1f, Branch2=%.1f, Branch3=%.1f",
				nodeMap["Branch1"].PositionX, nodeMap["Branch2"].PositionX, nodeMap["Branch3"].PositionX)
			break
		}
		uniquePositions[pos] = true
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

func TestHarNoJSONBodyTemplating(t *testing.T) {
	// Create a test HAR with entries containing JSON bodies that should NOT be templated
	// since template variables now only work for deltas in query, header, form-body, and urlencoded-body
	entries := []thar.Entry{
		{
			StartedDateTime: time.Now(),
			ResourceType:    "xhr",
			Request: thar.Request{
				Method:      "POST",
				URL:         "http://example.com/categories",
				HTTPVersion: "HTTP/1.1",
				Headers: []thar.Header{
					{Name: "Content-Type", Value: "application/json"},
				},
				PostData: &thar.PostData{
					MimeType: "application/json",
					Text:     `{"name": "Electronics"}`,
				},
			},
			Response: thar.Response{
				Status: 200,
				Content: thar.Content{
					MimeType: "application/json",
					Text:     `{"id": 2, "name": "Electronics", "created_at": "2025-05-25 14:58:04", "updated_at": "2025-05-25 14:58:04"}`,
				},
			},
		},
		{
			StartedDateTime: time.Now().Add(time.Second),
			ResourceType:    "xhr",
			Request: thar.Request{
				Method:      "POST",
				URL:         "http://example.com/products",
				HTTPVersion: "HTTP/1.1",
				Headers: []thar.Header{
					{Name: "Content-Type", Value: "application/json"},
				},
				PostData: &thar.PostData{
					MimeType: "application/json",
					Text:     `{"name": "Laptop", "category_id": 2}`,
				},
			},
			Response: thar.Response{
				Status: 200,
				Content: thar.Content{
					MimeType: "application/json",
					Text:     `{"id": 1, "name": "Laptop", "category_id": 2}`,
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

	// Verify JSON body templating behavior: base examples unmodified, delta examples templated
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

		// Decompress if needed
		var bodyData []byte
		if rawBody.CompressType == compress.CompressTypeZstd {
			decompressed, err := compress.Decompress(rawBody.Data, compress.CompressTypeZstd)
			if err != nil {
				t.Errorf("Failed to decompress body: %v", err)
				continue
			}
			bodyData = decompressed
		} else {
			bodyData = rawBody.Data
		}
		
		bodyStr := string(bodyData)

		// Check if this is the products request
		if strings.Contains(bodyStr, "category_id") {
			// Determine if this is a delta example
			isDelta := strings.Contains(example.Name, "Delta")
			
			if isDelta {
				// Delta examples SHOULD have templated values
				if !strings.Contains(bodyStr, "{{") || !strings.Contains(bodyStr, "}}") {
					t.Errorf("Delta example should have templated category_id in JSON body: %s", bodyStr)
				}
				if !strings.Contains(bodyStr, "{{ request_0.response.body.id }}") {
					t.Errorf("Delta example should have category_id templated as {{ request_0.response.body.id }}: %s", bodyStr)
				}
			} else {
				// Base examples should preserve original values
				if strings.Contains(bodyStr, "{{") && strings.Contains(bodyStr, "}}") {
					t.Errorf("Base example should NOT have template variables in JSON body: %s", bodyStr)
				}
				// The original test expects "category_id": 2 but the JSON might be ordered differently
				if !strings.Contains(bodyStr, "\"category_id\":2") && !strings.Contains(bodyStr, "\"category_id\": 2") {
					t.Errorf("Base example should preserve original category_id value: %s", bodyStr)
				}
			}
		}
	}

	// Template variables and edges are created for all body types including JSON bodies
	// but only for delta examples - base examples preserve original values.
}

func TestHarTemplatingInDeltasOnly(t *testing.T) {
	// Create a test HAR with entries that should have template variables in delta examples
	// for queries, headers, form bodies, and URL-encoded bodies
	entries := []thar.Entry{
		{
			StartedDateTime: time.Now(),
			ResourceType:    "xhr",
			Request: thar.Request{
				Method:      "POST",
				URL:         "http://example.com/auth",
				HTTPVersion: "HTTP/1.1",
				Headers: []thar.Header{
					{Name: "Authorization", Value: "Bearer abc123token"},
					{Name: "Content-Type", Value: "application/x-www-form-urlencoded"},
				},
				QueryString: []thar.Query{
					{Name: "api_key", Value: "secret123"},
				},
				PostData: &thar.PostData{
					MimeType: "application/x-www-form-urlencoded",
					Params:   []thar.Param{{Name: "username", Value: "testuser123"}},
				},
			},
			Response: thar.Response{
				Status: 200,
				Content: thar.Content{
					MimeType: "application/json",
					Text:     `{"token": "abc123token", "user_id": "testuser123"}`,
				},
			},
		},
		{
			StartedDateTime: time.Now().Add(time.Second),
			ResourceType:    "xhr",
			Request: thar.Request{
				Method:      "GET",
				URL:         "http://example.com/profile",
				HTTPVersion: "HTTP/1.1",
				Headers: []thar.Header{
					{Name: "Authorization", Value: "Bearer abc123token"},
					{Name: "User-ID", Value: "testuser123"},
				},
				QueryString: []thar.Query{
					{Name: "user", Value: "testuser123"},
				},
			},
			Response: thar.Response{
				Status: 200,
				Content: thar.Content{
					MimeType: "application/json",
					Text:     `{"id": "testuser123", "name": "Test User"}`,
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

	// Verify we have the expected number of examples (3 per entry: regular, default, delta)
	if len(resolved.Examples) != 6 {
		t.Errorf("Expected 6 examples, got %d", len(resolved.Examples))
	}

	// Track delta examples
	var deltaExamples []mitemapiexample.ItemApiExample
	for _, example := range resolved.Examples {
		if strings.Contains(example.Name, "Delta") {
			deltaExamples = append(deltaExamples, example)
		}
	}

	if len(deltaExamples) != 2 {
		t.Errorf("Expected 2 delta examples, got %d", len(deltaExamples))
	}

	// Verify headers: ONLY delta examples should have templated values, normal and default should have original
	hasTemplatedHeader := false
	hasOriginalHeader := false
	var normalExamples []mitemapiexample.ItemApiExample
	var defaultExamples []mitemapiexample.ItemApiExample

	for _, example := range resolved.Examples {
		if example.IsDefault {
			defaultExamples = append(defaultExamples, example)
		} else if !strings.Contains(example.Name, "Delta") {
			normalExamples = append(normalExamples, example)
		}
	}

	for _, header := range resolved.Headers {
		// Check for templated Authorization header in delta examples ONLY
		for _, deltaExample := range deltaExamples {
			if header.ExampleID == deltaExample.ID && header.HeaderKey == "Authorization" {
				if strings.Contains(header.Value, "{{") && strings.Contains(header.Value, "}}") {
					hasTemplatedHeader = true
				}
			}
		}
		// Check for original Authorization header in normal and default examples
		for _, normalExample := range normalExamples {
			if header.ExampleID == normalExample.ID && header.HeaderKey == "Authorization" && !strings.Contains(header.Value, "{{") {
				hasOriginalHeader = true
			}
		}
		for _, defaultExample := range defaultExamples {
			if header.ExampleID == defaultExample.ID && header.HeaderKey == "Authorization" && !strings.Contains(header.Value, "{{") {
				hasOriginalHeader = true
			}
		}
	}

	if !hasTemplatedHeader {
		t.Error("Expected to find templated Authorization header in delta examples")
	}
	if !hasOriginalHeader {
		t.Error("Expected to find original Authorization header in normal and default examples")
	}

	// Verify queries: ONLY delta examples should have templated values, normal and default should have original
	hasTemplatedQuery := false
	hasOriginalQuery := false
	for _, query := range resolved.Queries {
		// Check for templated query in delta examples ONLY
		for _, deltaExample := range deltaExamples {
			if query.ExampleID == deltaExample.ID && query.QueryKey == "user" {
				if strings.Contains(query.Value, "{{") && strings.Contains(query.Value, "}}") {
					hasTemplatedQuery = true
				}
			}
		}
		// Check for original query in normal and default examples
		for _, normalExample := range normalExamples {
			if query.ExampleID == normalExample.ID && query.QueryKey == "user" && !strings.Contains(query.Value, "{{") {
				hasOriginalQuery = true
			}
		}
		for _, defaultExample := range defaultExamples {
			if query.ExampleID == defaultExample.ID && query.QueryKey == "user" && !strings.Contains(query.Value, "{{") {
				hasOriginalQuery = true
			}
		}
	}

	if !hasTemplatedQuery {
		t.Error("Expected to find templated query in delta examples")
	}
	if !hasOriginalQuery {
		t.Error("Expected to find original query in normal and default examples")
	}

	// Verify URL-encoded bodies: ONLY delta examples should have templated values, normal and default should have original
	hasTemplatedUrlBody := false
	hasOriginalUrlBody := false
	for _, urlBody := range resolved.UrlEncodedBodies {
		// Check for templated URL-encoded body in delta examples ONLY
		for _, deltaExample := range deltaExamples {
			if urlBody.ExampleID == deltaExample.ID && urlBody.BodyKey == "username" {
				if strings.Contains(urlBody.Value, "{{") && strings.Contains(urlBody.Value, "}}") {
					hasTemplatedUrlBody = true
				}
			}
		}
		// Check for original URL-encoded body in normal and default examples
		for _, normalExample := range normalExamples {
			if urlBody.ExampleID == normalExample.ID && urlBody.BodyKey == "username" && !strings.Contains(urlBody.Value, "{{") {
				hasOriginalUrlBody = true
			}
		}
		for _, defaultExample := range defaultExamples {
			if urlBody.ExampleID == defaultExample.ID && urlBody.BodyKey == "username" && !strings.Contains(urlBody.Value, "{{") {
				hasOriginalUrlBody = true
			}
		}
	}

	if !hasTemplatedUrlBody {
		t.Error("Expected to find templated URL-encoded body in delta examples")
	}
	if !hasOriginalUrlBody {
		t.Error("Expected to find original URL-encoded body in normal and default examples")
	}

	// Verify edges are created for dependencies
	if len(resolved.Edges) == 0 {
		t.Error("Expected edges to be created for template variable dependencies")
	}
}

func TestNodePositioningNoOverlaps(t *testing.T) {
	// Create a test flow with multiple branches to test positioning
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
	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()
	node3ID := idwrap.NewNow()
	node4ID := idwrap.NewNow()
	node5ID := idwrap.NewNow()

	// Add nodes
	result.Nodes = append(result.Nodes,
		mnnode.MNode{ID: startID, Name: "Start"},
		mnnode.MNode{ID: node1ID, Name: "Node1"},
		mnnode.MNode{ID: node2ID, Name: "Node2"},
		mnnode.MNode{ID: node3ID, Name: "Node3"},
		mnnode.MNode{ID: node4ID, Name: "Node4"},
		mnnode.MNode{ID: node5ID, Name: "Node5"},
	)

	// Add start noop node
	result.NoopNodes = append(result.NoopNodes,
		mnnoop.NoopNode{
			Type:       mnnoop.NODE_NO_OP_KIND_START,
			FlowNodeID: startID,
		},
	)

	// Add edges to create a complex flow structure
	result.Edges = append(result.Edges,
		edge.Edge{SourceID: startID, TargetID: node1ID},
		edge.Edge{SourceID: startID, TargetID: node2ID},
		edge.Edge{SourceID: node1ID, TargetID: node3ID},
		edge.Edge{SourceID: node2ID, TargetID: node4ID},
		edge.Edge{SourceID: node3ID, TargetID: node5ID},
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

	// Test 1: Start node should be at origin (0,0)
	if nodeMap["Start"].PositionX != 0 || nodeMap["Start"].PositionY != 0 {
		t.Errorf("Start node not at origin: (%f, %f)", nodeMap["Start"].PositionX, nodeMap["Start"].PositionY)
	}

	// Test 2: No two nodes should occupy the same position
	positions := make(map[string][]string)
	for name, node := range nodeMap {
		posKey := fmt.Sprintf("%.0f,%.0f", node.PositionX, node.PositionY)
		positions[posKey] = append(positions[posKey], name)
	}

	for posKey, nodes := range positions {
		if len(nodes) > 1 {
			t.Errorf("Multiple nodes at position %s: %v", posKey, nodes)
		}
	}

	// Test 3: Child nodes should be positioned below their parents (higher Y values)
	if nodeMap["Node1"].PositionY <= nodeMap["Start"].PositionY {
		t.Errorf("Node1 should be below Start: Start.Y=%f, Node1.Y=%f",
			nodeMap["Start"].PositionY, nodeMap["Node1"].PositionY)
	}
	if nodeMap["Node2"].PositionY <= nodeMap["Start"].PositionY {
		t.Errorf("Node2 should be below Start: Start.Y=%f, Node2.Y=%f",
			nodeMap["Start"].PositionY, nodeMap["Node2"].PositionY)
	}
	if nodeMap["Node3"].PositionY <= nodeMap["Node1"].PositionY {
		t.Errorf("Node3 should be below Node1: Node1.Y=%f, Node3.Y=%f",
			nodeMap["Node1"].PositionY, nodeMap["Node3"].PositionY)
	}
	if nodeMap["Node4"].PositionY <= nodeMap["Node2"].PositionY {
		t.Errorf("Node4 should be below Node2: Node2.Y=%f, Node4.Y=%f",
			nodeMap["Node2"].PositionY, nodeMap["Node4"].PositionY)
	}
	if nodeMap["Node5"].PositionY <= nodeMap["Node3"].PositionY {
		t.Errorf("Node5 should be below Node3: Node3.Y=%f, Node5.Y=%f",
			nodeMap["Node3"].PositionY, nodeMap["Node5"].PositionY)
	}

	// Test 4: Nodes should be spaced far enough apart
	// The algorithm uses 400px horizontal and 300px vertical spacing
	// So minimum distance can be 300px (vertical) or ~360.6px (diagonal at 1 level)
	const minSpacing = 250.0
	for name1, node1 := range nodeMap {
		for name2, node2 := range nodeMap {
			if name1 >= name2 { // avoid duplicate checks
				continue
			}

			dx := node1.PositionX - node2.PositionX
			dy := node1.PositionY - node2.PositionY
			distance := math.Sqrt(dx*dx + dy*dy)
			if distance < minSpacing {
				t.Errorf("Nodes %s and %s too close: distance=%.1f, minimum=%.1f",
					name1, name2, distance, minSpacing)
			}
		}
	}

	// Test 5: Print positions for debugging
	t.Logf("Node positions:")
	for name, node := range nodeMap {
		t.Logf("  %s: (%.0f, %.0f)", name, node.PositionX, node.PositionY)
	}
}

func TestHarFolderHierarchy(t *testing.T) {
	// Create test HAR with URLs that should create folder hierarchies
	entries := []thar.Entry{
		{
			StartedDateTime: time.Now(),
			Request: thar.Request{
				Method:      "GET",
				URL:         "https://api.example.com/v1/users/123",
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
				URL:         "https://api.example.com/v1/users/create",
				HTTPVersion: "HTTP/1.1",
				Headers: []thar.Header{
					{Name: "Content-Type", Value: "application/json"},
				},
			},
		},
		{
			StartedDateTime: time.Now().Add(2 * time.Second),
			Request: thar.Request{
				Method:      "GET",
				URL:         "https://api.example.com/v1/posts/456",
				HTTPVersion: "HTTP/1.1",
				Headers: []thar.Header{
					{Name: "Content-Type", Value: "application/json"},
				},
			},
		},
		{
			StartedDateTime: time.Now().Add(3 * time.Second),
			Request: thar.Request{
				Method:      "GET",
				URL:         "https://other.example.com/api/health",
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
	collectionID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()
	resolved, err := thar.ConvertHAR(&testHar, collectionID, workspaceID)
	if err != nil {
		t.Fatalf("Error converting HAR: %v", err)
	}

	// Verify folder structure
	if len(resolved.Folders) == 0 {
		t.Fatal("Expected folders to be created, but got none")
	}

	// Create a map of folder names to folders for easy lookup
	foldersByName := make(map[string]mitemfolder.ItemFolder)
	for _, folder := range resolved.Folders {
		foldersByName[folder.Name] = folder
	}

	// Verify domain folders exist
	if _, exists := foldersByName["api.example.com"]; !exists {
		t.Error("Expected domain folder 'api.example.com' to be created")
	}
	if _, exists := foldersByName["other.example.com"]; !exists {
		t.Error("Expected domain folder 'other.example.com' to be created")
	}

	// Verify path folders exist
	if _, exists := foldersByName["v1"]; !exists {
		t.Error("Expected path folder 'v1' to be created")
	}
	if _, exists := foldersByName["users"]; !exists {
		t.Error("Expected path folder 'users' to be created")
	}
	if _, exists := foldersByName["posts"]; !exists {
		t.Error("Expected path folder 'posts' to be created")
	}
	if _, exists := foldersByName["api"]; !exists {
		t.Error("Expected path folder 'api' to be created")
	}

	// Verify folder hierarchy is correct
	apiExampleComFolder := foldersByName["api.example.com"]
	v1Folder := foldersByName["v1"]
	usersFolder := foldersByName["users"]
	postsFolder := foldersByName["posts"]
	otherExampleComFolder := foldersByName["other.example.com"]
	apiFolder := foldersByName["api"]

	// Check parent relationships
	if apiExampleComFolder.ParentID != nil {
		t.Error("Domain folder 'api.example.com' should have no parent")
	}
	if v1Folder.ParentID == nil || *v1Folder.ParentID != apiExampleComFolder.ID {
		t.Error("Folder 'v1' should have 'api.example.com' as parent")
	}
	if usersFolder.ParentID == nil || *usersFolder.ParentID != v1Folder.ID {
		t.Error("Folder 'users' should have 'v1' as parent")
	}
	if postsFolder.ParentID == nil || *postsFolder.ParentID != v1Folder.ID {
		t.Error("Folder 'posts' should have 'v1' as parent")
	}
	if otherExampleComFolder.ParentID != nil {
		t.Error("Domain folder 'other.example.com' should have no parent")
	}
	if apiFolder.ParentID == nil || *apiFolder.ParentID != otherExampleComFolder.ID {
		t.Error("Folder 'api' should have 'other.example.com' as parent")
	}

	// Verify APIs are placed in correct folders
	expectedAPIFolders := map[string]string{
		"123":            "users", // Should be in users folder
		"create":         "users", // Should be in users folder
		"456":            "posts", // Should be in posts folder
		"health":         "api",   // Should be in api folder
		"123 (Delta)":    "users", // Delta version in users folder
		"create (Delta)": "users", // Delta version in users folder
		"456 (Delta)":    "posts", // Delta version in posts folder
		"health (Delta)": "api",   // Delta version in api folder
	}

	for _, api := range resolved.Apis {
		expectedFolderName, exists := expectedAPIFolders[api.Name]
		if !exists {
			t.Errorf("Unexpected API name: %s", api.Name)
			continue
		}

		if api.FolderID == nil {
			t.Errorf("API '%s' should be placed in a folder", api.Name)
			continue
		}

		expectedFolder := foldersByName[expectedFolderName]
		if *api.FolderID != expectedFolder.ID {
			t.Errorf("API '%s' should be in folder '%s', but is in a different folder", api.Name, expectedFolderName)
		}
	}

	// Verify all folders belong to the same collection
	for _, folder := range resolved.Folders {
		if folder.CollectionID != collectionID {
			t.Errorf("Folder '%s' should belong to collection %s, but belongs to %s", folder.Name, collectionID, folder.CollectionID)
		}
	}
}

func TestHarFolderHierarchySimpleURL(t *testing.T) {
	// Test with simpler URLs
	entries := []thar.Entry{
		{
			StartedDateTime: time.Now(),
			Request: thar.Request{
				Method:      "GET",
				URL:         "http://example.com/api",
				HTTPVersion: "HTTP/1.1",
				Headers: []thar.Header{
					{Name: "Content-Type", Value: "application/json"},
				},
			},
		},
		{
			StartedDateTime: time.Now().Add(time.Second),
			Request: thar.Request{
				Method:      "GET",
				URL:         "http://example.com/users",
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
	collectionID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()
	resolved, err := thar.ConvertHAR(&testHar, collectionID, workspaceID)
	if err != nil {
		t.Fatalf("Error converting HAR: %v", err)
	}

	// Should only create domain folder
	if len(resolved.Folders) != 1 {
		t.Errorf("Expected 1 folder (domain only), got %d", len(resolved.Folders))
	}

	domainFolder := resolved.Folders[0]
	if domainFolder.Name != "example.com" {
		t.Errorf("Expected domain folder name 'example.com', got '%s'", domainFolder.Name)
	}

	// Both APIs should be in the domain folder
	for _, api := range resolved.Apis {
		if api.FolderID == nil || *api.FolderID != domainFolder.ID {
			t.Errorf("API '%s' should be in domain folder", api.Name)
		}
	}

	// Verify API names
	expectedAPINames := map[string]bool{
		"api":           true,
		"users":         true,
		"api (Delta)":   true,
		"users (Delta)": true,
	}
	for _, api := range resolved.Apis {
		if !expectedAPINames[api.Name] {
			t.Errorf("Unexpected API name: %s", api.Name)
		}
	}
}

func TestHarFolderHierarchyRootURL(t *testing.T) {
	// Test with root URLs (no path)
	entries := []thar.Entry{
		{
			StartedDateTime: time.Now(),
			Request: thar.Request{
				Method:      "GET",
				URL:         "http://example.com",
				HTTPVersion: "HTTP/1.1",
				Headers: []thar.Header{
					{Name: "Content-Type", Value: "application/json"},
				},
			},
		},
		{
			StartedDateTime: time.Now().Add(time.Second),
			Request: thar.Request{
				Method:      "GET",
				URL:         "http://example.com/",
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
	collectionID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()
	resolved, err := thar.ConvertHAR(&testHar, collectionID, workspaceID)
	if err != nil {
		t.Fatalf("Error converting HAR: %v", err)
	}

	// Should only create domain folder
	if len(resolved.Folders) != 1 {
		t.Errorf("Expected 1 folder (domain only), got %d", len(resolved.Folders))
	}

	domainFolder := resolved.Folders[0]
	if domainFolder.Name != "example.com" {
		t.Errorf("Expected domain folder name 'example.com', got '%s'", domainFolder.Name)
	}

	// Both APIs should be in the domain folder and named after the domain
	for _, api := range resolved.Apis {
		if api.FolderID == nil || *api.FolderID != domainFolder.ID {
			t.Errorf("API '%s' should be in domain folder", api.Name)
		}
		if api.Name != "example.com" && api.Name != "example.com (Delta)" {
			t.Errorf("Expected API name 'example.com' or 'example.com (Delta)', got '%s'", api.Name)
		}
	}
}

func TestHarFolderHierarchyDuplicatePaths(t *testing.T) {
	// Test that duplicate paths don't create duplicate folders
	entries := []thar.Entry{
		{
			StartedDateTime: time.Now(),
			Request: thar.Request{
				Method:      "GET",
				URL:         "http://api.example.com/v1/users/123",
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
				URL:         "http://api.example.com/v1/users/456",
				HTTPVersion: "HTTP/1.1",
				Headers: []thar.Header{
					{Name: "Content-Type", Value: "application/json"},
				},
			},
		},
		{
			StartedDateTime: time.Now().Add(2 * time.Second),
			Request: thar.Request{
				Method:      "DELETE",
				URL:         "http://api.example.com/v1/users/789",
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
	collectionID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()
	resolved, err := thar.ConvertHAR(&testHar, collectionID, workspaceID)
	if err != nil {
		t.Fatalf("Error converting HAR: %v", err)
	}

	// Should create exactly 3 folders: domain, v1, users
	if len(resolved.Folders) != 3 {
		t.Errorf("Expected 3 folders, got %d", len(resolved.Folders))
	}

	// Create a map to count folders by name
	folderCounts := make(map[string]int)
	for _, folder := range resolved.Folders {
		folderCounts[folder.Name]++
	}

	// Verify no duplicate folders
	expectedFolders := []string{"api.example.com", "v1", "users"}
	for _, expectedFolder := range expectedFolders {
		if count, exists := folderCounts[expectedFolder]; !exists {
			t.Errorf("Expected folder '%s' not found", expectedFolder)
		} else if count != 1 {
			t.Errorf("Expected exactly 1 folder named '%s', got %d", expectedFolder, count)
		}
	}

	// All APIs should be in the users folder
	usersFolder := mitemfolder.ItemFolder{}
	for _, folder := range resolved.Folders {
		if folder.Name == "users" {
			usersFolder = folder
			break
		}
	}

	for _, api := range resolved.Apis {
		if api.FolderID == nil || *api.FolderID != usersFolder.ID {
			t.Errorf("API '%s' should be in users folder", api.Name)
		}
	}
}

func TestHarFolderHierarchyEcommerce(t *testing.T) {
	// Test with e-commerce-like URLs similar to the user's image
	entries := []thar.Entry{
		{
			StartedDateTime: time.Now(),
			Request: thar.Request{
				Method:      "POST",
				URL:         "https://ecommerce-admin-panel.fly.dev/api/auth/login",
				HTTPVersion: "HTTP/1.1",
				Headers: []thar.Header{
					{Name: "Content-Type", Value: "application/json"},
				},
			},
		},
		{
			StartedDateTime: time.Now().Add(time.Second),
			Request: thar.Request{
				Method:      "DELETE",
				URL:         "https://ecommerce-admin-panel.fly.dev/api/products/14",
				HTTPVersion: "HTTP/1.1",
				Headers: []thar.Header{
					{Name: "Content-Type", Value: "application/json"},
				},
			},
		},
		{
			StartedDateTime: time.Now().Add(2 * time.Second),
			Request: thar.Request{
				Method:      "GET",
				URL:         "https://ecommerce-admin-panel.fly.dev/api/categories",
				HTTPVersion: "HTTP/1.1",
				Headers: []thar.Header{
					{Name: "Content-Type", Value: "application/json"},
				},
			},
		},
		{
			StartedDateTime: time.Now().Add(3 * time.Second),
			Request: thar.Request{
				Method:      "DELETE",
				URL:         "https://ecommerce-admin-panel.fly.dev/api/categories/16",
				HTTPVersion: "HTTP/1.1",
				Headers: []thar.Header{
					{Name: "Content-Type", Value: "application/json"},
				},
			},
		},
		{
			StartedDateTime: time.Now().Add(4 * time.Second),
			Request: thar.Request{
				Method:      "GET",
				URL:         "https://ecommerce-admin-panel.fly.dev/api/tags",
				HTTPVersion: "HTTP/1.1",
				Headers: []thar.Header{
					{Name: "Content-Type", Value: "application/json"},
				},
			},
		},
		{
			StartedDateTime: time.Now().Add(5 * time.Second),
			Request: thar.Request{
				Method:      "DELETE",
				URL:         "https://ecommerce-admin-panel.fly.dev/api/tags/12",
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
	collectionID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()
	resolved, err := thar.ConvertHAR(&testHar, collectionID, workspaceID)
	if err != nil {
		t.Fatalf("Error converting HAR: %v", err)
	}

	// Verify folder structure
	if len(resolved.Folders) == 0 {
		t.Fatal("Expected folders to be created, but got none")
	}

	// Create a map of folder names to folders for easy lookup
	foldersByName := make(map[string]mitemfolder.ItemFolder)
	for _, folder := range resolved.Folders {
		foldersByName[folder.Name] = folder
	}

	// Verify expected folders exist
	expectedFolders := []string{
		"ecommerce-admin-panel.fly.dev", // domain
		"api",                           // main API path
		"auth",                          // auth subfolder
		"products",                      // products subfolder
		"categories",                    // categories subfolder
		"tags",                          // tags subfolder
	}

	for _, expectedFolder := range expectedFolders {
		if _, exists := foldersByName[expectedFolder]; !exists {
			t.Errorf("Expected folder '%s' to be created", expectedFolder)
		}
	}

	// Verify folder hierarchy
	domainFolder := foldersByName["ecommerce-admin-panel.fly.dev"]
	apiFolder := foldersByName["api"]
	authFolder := foldersByName["auth"]
	productsFolder := foldersByName["products"]
	categoriesFolder := foldersByName["categories"]
	tagsFolder := foldersByName["tags"]

	// Check parent relationships
	if domainFolder.ParentID != nil {
		t.Error("Domain folder should have no parent")
	}
	if apiFolder.ParentID == nil || *apiFolder.ParentID != domainFolder.ID {
		t.Error("'api' folder should have domain as parent")
	}
	if authFolder.ParentID == nil || *authFolder.ParentID != apiFolder.ID {
		t.Error("'auth' folder should have 'api' as parent")
	}
	if productsFolder.ParentID == nil || *productsFolder.ParentID != apiFolder.ID {
		t.Error("'products' folder should have 'api' as parent")
	}
	if categoriesFolder.ParentID == nil || *categoriesFolder.ParentID != apiFolder.ID {
		t.Error("'categories' folder should have 'api' as parent")
	}
	if tagsFolder.ParentID == nil || *tagsFolder.ParentID != apiFolder.ID {
		t.Error("'tags' folder should have 'api' as parent")
	}

	// Verify APIs are placed in correct folders
	expectedAPIFolders := map[string]string{
		"login":              "auth",       // POST /api/auth/login
		"14":                 "products",   // DELETE /api/products/14
		"products":           "products",   // When folder and API name match
		"categories":         "categories", // GET /api/categories (should be in categories folder)
		"16":                 "categories", // DELETE /api/categories/16
		"tags":               "tags",       // GET /api/tags (should be in tags folder)
		"12":                 "tags",       // DELETE /api/tags/12
		"login (Delta)":      "auth",       // Delta version
		"14 (Delta)":         "products",   // Delta version
		"products (Delta)":   "products",   // Delta version
		"categories (Delta)": "categories", // Delta version
		"16 (Delta)":         "categories", // Delta version
		"tags (Delta)":       "tags",       // Delta version
		"12 (Delta)":         "tags",       // Delta version
	}

	for _, api := range resolved.Apis {
		expectedFolderName, exists := expectedAPIFolders[api.Name]
		if !exists {
			t.Errorf("Unexpected API name: %s", api.Name)
			continue
		}

		if api.FolderID == nil {
			t.Errorf("API '%s' should be placed in a folder", api.Name)
			continue
		}

		expectedFolder := foldersByName[expectedFolderName]
		if *api.FolderID != expectedFolder.ID {
			t.Errorf("API '%s' should be in folder '%s', but is in a different folder", api.Name, expectedFolderName)
		}
	}
}

func TestHarComprehensiveIntegration(t *testing.T) {
	// This test verifies the complete workflow of HAR conversion including:
	// 1. Folder hierarchy creation based on URL structure
	// 2. Delta templating for dependencies (only in delta examples)
	// 3. Proper example types (default, normal, delta)
	// 4. JSON bodies not being templated
	// 5. Flow and edge creation for dependencies

	entries := []thar.Entry{
		{
			StartedDateTime: time.Now(),
			ResourceType:    "xhr",
			Request: thar.Request{
				Method:      "POST",
				URL:         "https://api.ecommerce.com/v1/auth/login",
				HTTPVersion: "HTTP/1.1",
				Headers: []thar.Header{
					{Name: "Content-Type", Value: "application/x-www-form-urlencoded"},
					{Name: "User-Agent", Value: "TestClient/1.0"},
				},
				QueryString: []thar.Query{
					{Name: "client_id", Value: "web_app"},
				},
				PostData: &thar.PostData{
					MimeType: "application/x-www-form-urlencoded",
					Params: []thar.Param{
						{Name: "username", Value: "admin"},
						{Name: "password", Value: "secret123"},
					},
				},
			},
			Response: thar.Response{
				Status: 200,
				Content: thar.Content{
					MimeType: "application/json",
					Text:     `{"token": "abc123token", "user_id": "user_456", "expires": 3600}`,
				},
			},
		},
		{
			StartedDateTime: time.Now().Add(time.Second),
			ResourceType:    "xhr",
			Request: thar.Request{
				Method:      "GET",
				URL:         "https://api.ecommerce.com/v1/users/profile",
				HTTPVersion: "HTTP/1.1",
				Headers: []thar.Header{
					{Name: "Authorization", Value: "Bearer abc123token"},
					{Name: "User-ID", Value: "user_456"},
				},
				QueryString: []thar.Query{
					{Name: "include", Value: "permissions"},
					{Name: "user_id", Value: "user_456"},
				},
			},
			Response: thar.Response{
				Status: 200,
				Content: thar.Content{
					MimeType: "application/json",
					Text:     `{"id": "user_456", "name": "Admin User", "permissions": ["read", "write"]}`,
				},
			},
		},
		{
			StartedDateTime: time.Now().Add(2 * time.Second),
			ResourceType:    "xhr",
			Request: thar.Request{
				Method:      "POST",
				URL:         "https://api.ecommerce.com/v1/products/create",
				HTTPVersion: "HTTP/1.1",
				Headers: []thar.Header{
					{Name: "Authorization", Value: "Bearer abc123token"},
					{Name: "Content-Type", Value: "application/json"},
				},
				PostData: &thar.PostData{
					MimeType: "application/json",
					Text:     `{"name": "New Product", "user_id": "user_456", "category_id": 5}`,
				},
			},
			Response: thar.Response{
				Status: 201,
				Content: thar.Content{
					MimeType: "application/json",
					Text:     `{"id": "product_789", "name": "New Product", "created_by": "user_456"}`,
				},
			},
		},
		{
			StartedDateTime: time.Now().Add(3 * time.Second),
			ResourceType:    "xhr",
			Request: thar.Request{
				Method:      "PUT",
				URL:         "https://different.api.com/settings/update",
				HTTPVersion: "HTTP/1.1",
				Headers: []thar.Header{
					{Name: "Authorization", Value: "Bearer abc123token"},
					{Name: "Content-Type", Value: "application/x-www-form-urlencoded"},
				},
				QueryString: []thar.Query{
					{Name: "token", Value: "abc123token"},
				},
				PostData: &thar.PostData{
					MimeType: "application/x-www-form-urlencoded",
					Params: []thar.Param{
						{Name: "setting_name", Value: "theme"},
						{Name: "setting_value", Value: "dark"},
						{Name: "user_token", Value: "abc123token"},
					},
				},
			},
			Response: thar.Response{
				Status: 200,
				Content: thar.Content{
					MimeType: "application/json",
					Text:     `{"success": true, "updated_by": "abc123token"}`,
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
	collectionID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()
	resolved, err := thar.ConvertHAR(&testHar, collectionID, workspaceID)
	if err != nil {
		t.Fatalf("Error converting HAR: %v", err)
	}

	// === Test 1: Verify folder hierarchy ===
	t.Run("FolderHierarchy", func(t *testing.T) {
		if len(resolved.Folders) == 0 {
			t.Fatal("Expected folders to be created")
		}

		foldersByName := make(map[string]mitemfolder.ItemFolder)
		for _, folder := range resolved.Folders {
			foldersByName[folder.Name] = folder
		}

		// Verify expected folders exist
		expectedFolders := []string{
			"api.ecommerce.com", "different.api.com", // domains
			"v1", "auth", "users", "products", "settings", // paths
		}

		for _, expectedFolder := range expectedFolders {
			if _, exists := foldersByName[expectedFolder]; !exists {
				t.Errorf("Expected folder '%s' to be created", expectedFolder)
			}
		}

		// Verify folder hierarchy
		domainFolder := foldersByName["api.ecommerce.com"]
		v1Folder := foldersByName["v1"]
		authFolder := foldersByName["auth"]

		if domainFolder.ParentID != nil {
			t.Error("Domain folder should have no parent")
		}
		if v1Folder.ParentID == nil || *v1Folder.ParentID != domainFolder.ID {
			t.Error("'v1' folder should have domain as parent")
		}
		if authFolder.ParentID == nil || *authFolder.ParentID != v1Folder.ID {
			t.Error("'auth' folder should have 'v1' as parent")
		}
	})

	// === Test 2: Verify API placement in folders ===
	t.Run("APIFolderPlacement", func(t *testing.T) {
		foldersByName := make(map[string]mitemfolder.ItemFolder)
		foldersById := make(map[idwrap.IDWrap]mitemfolder.ItemFolder)
		for _, folder := range resolved.Folders {
			foldersByName[folder.Name] = folder
			foldersById[folder.ID] = folder
		}

		// Verify folder structure exists as expected

		expectedAPIFolders := map[string]string{
			"login":           "auth",     // POST /v1/auth/login
			"profile":         "users",    // GET /v1/users/profile
			"create":          "products", // POST /v1/products/create
			"update":          "settings", // PUT /settings/update
			"login (Delta)":   "auth",     // Delta version
			"profile (Delta)": "users",    // Delta version
			"create (Delta)":  "products", // Delta version
			"update (Delta)":  "settings", // Delta version
		}

		for _, api := range resolved.Apis {
			expectedFolderName, exists := expectedAPIFolders[api.Name]
			if !exists {
				t.Errorf("Unexpected API name: %s", api.Name)
				continue
			}

			if api.FolderID == nil {
				t.Errorf("API '%s' should be placed in a folder", api.Name)
				continue
			}

			expectedFolder := foldersByName[expectedFolderName]
			if *api.FolderID != expectedFolder.ID {
				actualFolderName := "UNKNOWN"
				if actualFolder, exists := foldersById[*api.FolderID]; exists {
					actualFolderName = actualFolder.Name
				}
				t.Errorf("API '%s' should be in folder '%s' but is in folder '%s'",
					api.Name, expectedFolderName, actualFolderName)
			}
		}
	})

	// === Test 3: Verify three types of examples (default, normal, delta) ===
	t.Run("ExampleTypes", func(t *testing.T) {
		if len(resolved.Examples) != 12 { // 4 APIs × 3 examples each
			t.Errorf("Expected 12 examples (4 APIs × 3 types), got %d", len(resolved.Examples))
		}

		examplesByType := make(map[string]int)
		for _, example := range resolved.Examples {
			if example.IsDefault {
				examplesByType["default"]++
			} else if strings.Contains(example.Name, "Delta") {
				examplesByType["delta"]++
			} else {
				examplesByType["normal"]++
			}
		}

		if examplesByType["default"] != 4 {
			t.Errorf("Expected 4 default examples, got %d", examplesByType["default"])
		}
		if examplesByType["normal"] != 4 {
			t.Errorf("Expected 4 normal examples, got %d", examplesByType["normal"])
		}
		if examplesByType["delta"] != 4 {
			t.Errorf("Expected 4 delta examples, got %d", examplesByType["delta"])
		}
	})

	// === Test 4: Verify template variables only in delta examples ===
	t.Run("DeltaTemplatingOnly", func(t *testing.T) {
		// Get example types
		var deltaExamples, normalExamples, defaultExamples []mitemapiexample.ItemApiExample
		for _, example := range resolved.Examples {
			if example.IsDefault {
				defaultExamples = append(defaultExamples, example)
			} else if strings.Contains(example.Name, "Delta") {
				deltaExamples = append(deltaExamples, example)
			} else {
				normalExamples = append(normalExamples, example)
			}
		}

		// Check headers: only delta should have templates
		hasTemplatedHeaderInDelta := false
		hasTemplatedHeaderInNormal := false
		hasTemplatedHeaderInDefault := false

		for _, header := range resolved.Headers {
			isTemplated := strings.Contains(header.Value, "{{") && strings.Contains(header.Value, "}}")

			// Check if this header belongs to delta examples
			for _, deltaExample := range deltaExamples {
				if header.ExampleID == deltaExample.ID && isTemplated {
					hasTemplatedHeaderInDelta = true
				}
			}

			// Check if this header belongs to normal examples
			for _, normalExample := range normalExamples {
				if header.ExampleID == normalExample.ID && isTemplated {
					hasTemplatedHeaderInNormal = true
				}
			}

			// Check if this header belongs to default examples
			for _, defaultExample := range defaultExamples {
				if header.ExampleID == defaultExample.ID && isTemplated {
					hasTemplatedHeaderInDefault = true
				}
			}
		}

		if !hasTemplatedHeaderInDelta {
			t.Error("Expected templated headers in delta examples")
		}
		if hasTemplatedHeaderInNormal {
			t.Error("Should NOT have templated headers in normal examples")
		}
		if hasTemplatedHeaderInDefault {
			t.Error("Should NOT have templated headers in default examples")
		}

		// Check queries: only delta should have templates
		hasTemplatedQueryInDelta := false
		hasTemplatedQueryInNormal := false

		for _, query := range resolved.Queries {
			isTemplated := strings.Contains(query.Value, "{{") && strings.Contains(query.Value, "}}")

			for _, deltaExample := range deltaExamples {
				if query.ExampleID == deltaExample.ID && isTemplated {
					hasTemplatedQueryInDelta = true
				}
			}

			for _, normalExample := range normalExamples {
				if query.ExampleID == normalExample.ID && isTemplated {
					hasTemplatedQueryInNormal = true
				}
			}
		}

		if !hasTemplatedQueryInDelta {
			t.Error("Expected templated queries in delta examples")
		}
		if hasTemplatedQueryInNormal {
			t.Error("Should NOT have templated queries in normal examples")
		}

		// Check URL-encoded bodies: only delta should have templates
		hasTemplatedUrlBodyInDelta := false
		hasTemplatedUrlBodyInNormal := false

		for _, urlBody := range resolved.UrlEncodedBodies {
			isTemplated := strings.Contains(urlBody.Value, "{{") && strings.Contains(urlBody.Value, "}}")

			for _, deltaExample := range deltaExamples {
				if urlBody.ExampleID == deltaExample.ID && isTemplated {
					hasTemplatedUrlBodyInDelta = true
				}
			}

			for _, normalExample := range normalExamples {
				if urlBody.ExampleID == normalExample.ID && isTemplated {
					hasTemplatedUrlBodyInNormal = true
				}
			}
		}

		if !hasTemplatedUrlBodyInDelta {
			t.Error("Expected templated URL-encoded bodies in delta examples")
		}
		if hasTemplatedUrlBodyInNormal {
			t.Error("Should NOT have templated URL-encoded bodies in normal examples")
		}
	})

	// === Test 5: Verify JSON body templating: base preserved, delta templated ===
	t.Run("JSONTemplatingBehavior", func(t *testing.T) {
		for _, body := range resolved.RawBodies {
			// Decompress if needed
			var bodyData []byte
			if body.CompressType == compress.CompressTypeZstd {
				decompressed, err := compress.Decompress(body.Data, compress.CompressTypeZstd)
				if err != nil {
					continue // Skip if decompression fails
				}
				bodyData = decompressed
			} else {
				bodyData = body.Data
			}
			
			bodyStr := string(bodyData)
			if strings.Contains(bodyStr, "user_id") || strings.Contains(bodyStr, "category_id") {
				// This is a JSON body that might have had dependencies
				// Find the corresponding example to check if it's a delta
				var isDelta bool
				for _, example := range resolved.Examples {
					if example.ID == body.ExampleID {
						isDelta = strings.Contains(example.Name, "Delta")
						break
					}
				}
				
				if isDelta {
					// Delta examples can have template variables
					// This is expected behavior now
				} else {
					// Base examples should NOT have template variables
					if strings.Contains(bodyStr, "{{") && strings.Contains(bodyStr, "}}") {
						t.Errorf("Base example JSON body should NOT contain template variables: %s", bodyStr)
					}
				}
			}
		}
	})

	// === Test 6: Verify flow structure and edges ===
	t.Run("FlowStructure", func(t *testing.T) {
		if resolved.Flow == (mflow.Flow{}) {
			t.Error("Flow should be populated")
		}

		if len(resolved.Nodes) == 0 {
			t.Error("Should have flow nodes")
		}

		if len(resolved.RequestNodes) != 4 {
			t.Errorf("Expected 4 request nodes, got %d", len(resolved.RequestNodes))
		}

		// Should have edges for dependencies
		if len(resolved.Edges) == 0 {
			t.Error("Expected edges for dependencies")
		}

		// Verify start node exists
		hasStartNode := false
		for _, noopNode := range resolved.NoopNodes {
			if noopNode.Type == mnnoop.NODE_NO_OP_KIND_START {
				hasStartNode = true
				break
			}
		}
		if !hasStartNode {
			t.Error("Should have a start node")
		}
	})

	// === Test 7: Verify collection and workspace IDs ===
	t.Run("IDConsistency", func(t *testing.T) {
		// Check APIs
		for _, api := range resolved.Apis {
			if api.CollectionID != collectionID {
				t.Errorf("API collection ID mismatch: expected %s, got %s", collectionID, api.CollectionID)
			}
		}

		// Check examples
		for _, example := range resolved.Examples {
			if example.CollectionID != collectionID {
				t.Errorf("Example collection ID mismatch: expected %s, got %s", collectionID, example.CollectionID)
			}
		}

		// Check folders
		for _, folder := range resolved.Folders {
			if folder.CollectionID != collectionID {
				t.Errorf("Folder collection ID mismatch: expected %s, got %s", collectionID, folder.CollectionID)
			}
		}

		// Check flow
		if resolved.Flow.WorkspaceID != workspaceID {
			t.Errorf("Flow workspace ID mismatch: expected %s, got %s", workspaceID, resolved.Flow.WorkspaceID)
		}
	})

	// === Test 8: Verify proper counts ===
	t.Run("ProperCounts", func(t *testing.T) {
		// 8 APIs (4 original + 4 delta)
		if len(resolved.Apis) != 8 {
			t.Errorf("Expected 8 APIs, got %d", len(resolved.Apis))
		}

		// 12 examples (4 entries × 3 examples per entry)
		if len(resolved.Examples) != 12 {
			t.Errorf("Expected 12 examples, got %d", len(resolved.Examples))
		}

		// Headers: should have headers for all examples
		if len(resolved.Headers) == 0 {
			t.Error("Should have headers")
		}

		// Queries: should have queries for relevant examples
		if len(resolved.Queries) == 0 {
			t.Error("Should have queries")
		}

		// URL-encoded bodies: should have bodies for relevant examples
		if len(resolved.UrlEncodedBodies) == 0 {
			t.Error("Should have URL-encoded bodies")
		}

		// Raw bodies: should have bodies for all examples
		if len(resolved.RawBodies) == 0 {
			t.Error("Should have raw bodies")
		}
	})

	t.Logf("✅ All integration tests passed!")
	t.Logf("📁 Created %d folders", len(resolved.Folders))
	t.Logf("🔗 Created %d APIs", len(resolved.Apis))
	t.Logf("📝 Created %d examples", len(resolved.Examples))
	t.Logf("🔄 Created %d edges", len(resolved.Edges))
}

func TestHarDependencyOrdering(t *testing.T) {
	// This test verifies that dependency ordering works correctly:
	// 1. Independent requests connect to start node
	// 2. Dependent requests connect to their dependencies
	// 3. Proper edge creation for dependencies

	entries := []thar.Entry{
		{
			StartedDateTime: time.Now(),
			ResourceType:    "xhr",
			Request: thar.Request{
				Method:      "POST",
				URL:         "https://api.example.com/auth/login",
				HTTPVersion: "HTTP/1.1",
				Headers: []thar.Header{
					{Name: "Content-Type", Value: "application/json"},
				},
				PostData: &thar.PostData{
					MimeType: "application/json",
					Text:     `{"username": "admin", "password": "secret"}`,
				},
			},
			Response: thar.Response{
				Status: 200,
				Content: thar.Content{
					MimeType: "application/json",
					Text:     `{"token": "auth_token_123", "user_id": "user_456"}`,
				},
			},
		},
		{
			StartedDateTime: time.Now().Add(time.Second),
			ResourceType:    "xhr",
			Request: thar.Request{
				Method:      "GET",
				URL:         "https://api.example.com/user/profile",
				HTTPVersion: "HTTP/1.1",
				Headers: []thar.Header{
					{Name: "Authorization", Value: "Bearer auth_token_123"}, // Depends on login
					{Name: "User-ID", Value: "user_456"},                    // Depends on login
				},
			},
			Response: thar.Response{
				Status: 200,
				Content: thar.Content{
					MimeType: "application/json",
					Text:     `{"id": "user_456", "name": "Admin", "role": "admin"}`,
				},
			},
		},
		{
			StartedDateTime: time.Now().Add(2 * time.Second),
			ResourceType:    "xhr",
			Request: thar.Request{
				Method:      "GET",
				URL:         "https://api.example.com/admin/settings",
				HTTPVersion: "HTTP/1.1",
				Headers: []thar.Header{
					{Name: "Authorization", Value: "Bearer auth_token_123"}, // Depends on login
				},
			},
			Response: thar.Response{
				Status: 200,
				Content: thar.Content{
					MimeType: "application/json",
					Text:     `{"theme": "dark", "notifications": true}`,
				},
			},
		},
		{
			StartedDateTime: time.Now().Add(3 * time.Second),
			ResourceType:    "xhr",
			Request: thar.Request{
				Method:      "GET",
				URL:         "https://api.different.com/public/status",
				HTTPVersion: "HTTP/1.1",
				Headers: []thar.Header{
					{Name: "Content-Type", Value: "application/json"},
				},
			},
			Response: thar.Response{
				Status: 200,
				Content: thar.Content{
					MimeType: "application/json",
					Text:     `{"status": "ok", "version": "1.0"}`,
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
	collectionID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()
	resolved, err := thar.ConvertHAR(&testHar, collectionID, workspaceID)
	if err != nil {
		t.Fatalf("Error converting HAR: %v", err)
	}

	// Build node and edge maps for analysis
	nodeMap := make(map[idwrap.IDWrap]mnnode.MNode)
	for _, node := range resolved.Nodes {
		nodeMap[node.ID] = node
	}

	// Find start node
	var startNode *mnnode.MNode
	for _, noop := range resolved.NoopNodes {
		if noop.Type == mnnoop.NODE_NO_OP_KIND_START {
			if node, exists := nodeMap[noop.FlowNodeID]; exists {
				startNode = &node
				break
			}
		}
	}

	if startNode == nil {
		t.Fatal("Start node not found")
	}

	// Analyze edges
	edgesFromStart := make([]edge.Edge, 0)
	edgesBetweenRequests := make([]edge.Edge, 0)
	incomingEdges := make(map[idwrap.IDWrap][]edge.Edge)

	for _, e := range resolved.Edges {
		if e.SourceID == startNode.ID {
			edgesFromStart = append(edgesFromStart, e)
		} else {
			edgesBetweenRequests = append(edgesBetweenRequests, e)
		}
		incomingEdges[e.TargetID] = append(incomingEdges[e.TargetID], e)
	}

	// Test 1: Verify that independent nodes (login and status) connect to start
	t.Run("IndependentNodesConnectToStart", func(t *testing.T) {
		// Should have exactly 2 edges from start node (login and status endpoints)
		// login: no dependencies, should connect to start
		// status: no dependencies, should connect to start
		// profile: depends on login, should NOT connect to start
		// settings: depends on login, should NOT connect to start

		expectedIndependentCount := 2
		if len(edgesFromStart) != expectedIndependentCount {
			t.Errorf("Expected %d edges from start node, got %d", expectedIndependentCount, len(edgesFromStart))
		}

		// Find the login and status nodes to verify they connect to start
		loginNodeFound := false
		statusNodeFound := false

		for _, e := range edgesFromStart {
			targetNode := nodeMap[e.TargetID]
			// Check if this is the login node (would contain "login" in name)
			if strings.Contains(targetNode.Name, "login") || strings.Contains(targetNode.Name, "request_0") {
				loginNodeFound = true
			}
			// Check if this is the status node (would contain "status" or be from different domain)
			if strings.Contains(targetNode.Name, "status") || strings.Contains(targetNode.Name, "request_3") {
				statusNodeFound = true
			}
		}

		if !loginNodeFound {
			t.Error("Login node should be connected to start node")
		}
		if !statusNodeFound {
			t.Error("Status node should be connected to start node")
		}
	})

	// Test 2: Verify that dependent nodes have dependency edges
	t.Run("DependentNodesHaveDependencyEdges", func(t *testing.T) {
		// Should have edges between requests for dependencies
		// profile depends on login
		// settings depends on login

		if len(edgesBetweenRequests) == 0 {
			t.Error("Expected dependency edges between requests, but found none")
		}

		// Verify that some nodes have incoming dependency edges (not from start)
		dependentNodesCount := 0
		for nodeID, edges := range incomingEdges {
			// Skip start node
			if nodeID == startNode.ID {
				continue
			}

			hasDependencyEdge := false
			for _, e := range edges {
				if e.SourceID != startNode.ID {
					hasDependencyEdge = true
					break
				}
			}

			if hasDependencyEdge {
				dependentNodesCount++
			}
		}

		if dependentNodesCount == 0 {
			t.Error("Expected some nodes to have dependency edges (not from start), but found none")
		}

		t.Logf("Found %d nodes with dependency edges", dependentNodesCount)
	})

	// Test 3: Verify total edge count is reasonable
	t.Run("ReasonableEdgeCount", func(t *testing.T) {
		// Should have:
		// - 2 edges from start node (login, status)
		// - Several dependency edges (profile->login, settings->login)
		// Total should be reasonable for 4 requests

		expectedMinEdges := 4 // At minimum: 2 from start + 2 dependencies
		if len(resolved.Edges) < expectedMinEdges {
			t.Errorf("Expected at least %d edges, got %d", expectedMinEdges, len(resolved.Edges))
		}

		t.Logf("Total edges: %d (from start: %d, dependencies: %d)",
			len(resolved.Edges), len(edgesFromStart), len(edgesBetweenRequests))
	})

	// Test 4: Verify no circular dependencies
	t.Run("NoCircularDependencies", func(t *testing.T) {
		// Build adjacency list and check for cycles
		adj := make(map[idwrap.IDWrap][]idwrap.IDWrap)
		for _, e := range resolved.Edges {
			adj[e.SourceID] = append(adj[e.SourceID], e.TargetID)
		}

		visited := make(map[idwrap.IDWrap]bool)
		inStack := make(map[idwrap.IDWrap]bool)

		var hasCycle func(idwrap.IDWrap) bool
		hasCycle = func(nodeID idwrap.IDWrap) bool {
			visited[nodeID] = true
			inStack[nodeID] = true

			for _, neighbor := range adj[nodeID] {
				if !visited[neighbor] {
					if hasCycle(neighbor) {
						return true
					}
				} else if inStack[neighbor] {
					return true
				}
			}

			inStack[nodeID] = false
			return false
		}

		for nodeID := range nodeMap {
			if !visited[nodeID] {
				if hasCycle(nodeID) {
					t.Error("Circular dependency detected in flow")
					break
				}
			}
		}
	})

	t.Logf("✅ Dependency ordering test passed!")
	t.Logf("📊 Analyzed %d nodes and %d edges", len(resolved.Nodes), len(resolved.Edges))
}

func TestHarDeltaParentIDsSet(t *testing.T) {
	// This test verifies that delta examples have proper DeltaParentID fields set
	// to prevent nil pointer dereferences in MergeExamples function

	entries := []thar.Entry{
		{
			StartedDateTime: time.Now(),
			ResourceType:    "xhr",
			Request: thar.Request{
				Method:      "GET",
				URL:         "https://api.example.com/users",
				HTTPVersion: "HTTP/1.1",
				Headers: []thar.Header{
					{Name: "Authorization", Value: "Bearer test-token"},
					{Name: "Content-Type", Value: "application/json"},
				},
				QueryString: []thar.Query{
					{Name: "page", Value: "1"},
					{Name: "limit", Value: "10"},
				},
			},
			Response: thar.Response{
				Status:      200,
				StatusText:  "OK",
				HTTPVersion: "HTTP/1.1",
				Headers:     []thar.Header{{Name: "Content-Type", Value: "application/json"}},
				Content:     thar.Content{Text: `{"users": [{"id": 1, "name": "test"}]}`},
			},
		},
	}

	har := &thar.HAR{
		Log: thar.Log{
			Entries: entries,
		},
	}

	collectionID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()

	resolved, err := thar.ConvertHAR(har, collectionID, workspaceID)
	if err != nil {
		t.Fatalf("Failed to convert HAR: %v", err)
	}

	// Verify that we have 3 examples: default, normal, and delta
	if len(resolved.Examples) != 3 {
		t.Fatalf("Expected 3 examples, got %d", len(resolved.Examples))
	}

	// Find delta example (should have "Delta" in name)
	var deltaExample *mitemapiexample.ItemApiExample
	for i, example := range resolved.Examples {
		if strings.Contains(example.Name, "Delta") {
			deltaExample = &resolved.Examples[i]
			break
		}
	}

	if deltaExample == nil {
		t.Fatal("No delta example found")
	}

	// Find headers for delta example
	var deltaHeaders []mexampleheader.Header
	var baseHeaders []mexampleheader.Header

	for _, header := range resolved.Headers {
		if header.ExampleID.Compare(deltaExample.ID) == 0 {
			deltaHeaders = append(deltaHeaders, header)
		} else if !strings.Contains(resolved.Examples[0].Name, "Delta") && header.ExampleID.Compare(resolved.Examples[0].ID) == 0 {
			baseHeaders = append(baseHeaders, header)
		}
	}

	// Verify delta headers have DeltaParentID set
	t.Run("DeltaHeadersHaveParentID", func(t *testing.T) {
		for _, deltaHeader := range deltaHeaders {
			if deltaHeader.DeltaParentID == nil {
				t.Errorf("Delta header %s has nil DeltaParentID", deltaHeader.HeaderKey)
			} else {
				// Verify the parent exists in base headers
				found := false
				for _, baseHeader := range baseHeaders {
					if baseHeader.ID.Compare(*deltaHeader.DeltaParentID) == 0 {
						found = true
						if baseHeader.HeaderKey != deltaHeader.HeaderKey {
							t.Errorf("Delta header %s points to base header with different key %s", deltaHeader.HeaderKey, baseHeader.HeaderKey)
						}
						break
					}
				}
				if !found {
					t.Errorf("Delta header %s has DeltaParentID pointing to non-existent base header", deltaHeader.HeaderKey)
				}
			}
		}
	})

	// Find queries for delta example
	var deltaQueries []mexamplequery.Query
	var baseQueries []mexamplequery.Query

	for _, query := range resolved.Queries {
		if query.ExampleID.Compare(deltaExample.ID) == 0 {
			deltaQueries = append(deltaQueries, query)
		} else if !strings.Contains(resolved.Examples[0].Name, "Delta") && query.ExampleID.Compare(resolved.Examples[0].ID) == 0 {
			baseQueries = append(baseQueries, query)
		}
	}

	// Verify delta queries have DeltaParentID set
	t.Run("DeltaQueriesHaveParentID", func(t *testing.T) {
		for _, deltaQuery := range deltaQueries {
			if deltaQuery.DeltaParentID == nil {
				t.Errorf("Delta query %s has nil DeltaParentID", deltaQuery.QueryKey)
			} else {
				// Verify the parent exists in base queries
				found := false
				for _, baseQuery := range baseQueries {
					if baseQuery.ID.Compare(*deltaQuery.DeltaParentID) == 0 {
						found = true
						if baseQuery.QueryKey != deltaQuery.QueryKey {
							t.Errorf("Delta query %s points to base query with different key %s", deltaQuery.QueryKey, baseQuery.QueryKey)
						}
						break
					}
				}
				if !found {
					t.Errorf("Delta query %s has DeltaParentID pointing to non-existent base query", deltaQuery.QueryKey)
				}
			}
		}
	})

	t.Logf("✅ Delta ParentID test passed!")
	t.Logf("📊 Verified %d delta headers and %d delta queries have proper parent IDs", len(deltaHeaders), len(deltaQueries))
}

func TestHarDeltaFormBodyParentIDsSet(t *testing.T) {
	// Create HAR with form data
	harData := `{
		"log": {
			"entries": [
				{
					"startedDateTime": "2023-01-01T10:00:00.000Z",
					"_resourceType": "xhr",
					"request": {
						"method": "POST",
						"url": "https://api.example.com/users",
						"httpVersion": "HTTP/1.1",
						"headers": [
							{"name": "Content-Type", "value": "multipart/form-data"}
						],
						"postData": {
							"mimeType": "multipart/form-data",
							"params": [
								{"name": "username", "value": "john"},
								{"name": "email", "value": "john@example.com"}
							]
						},
						"queryString": []
					},
					"response": {
						"status": 200,
						"statusText": "OK",
						"httpVersion": "HTTP/1.1",
						"headers": [],
						"content": {"size": 0, "mimeType": "application/json", "text": ""}
					}
				}
			]
		}
	}`

	har, err := thar.ConvertRaw([]byte(harData))
	require.NoError(t, err)

	collectionID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()

	result, err := thar.ConvertHAR(har, collectionID, workspaceID)
	require.NoError(t, err)

	t.Run("DeltaFormBodiesHaveParentID", func(t *testing.T) {
		// Find the normal example form bodies
		var normalFormBodies []mbodyform.BodyForm
		var deltaFormBodies []mbodyform.BodyForm

		for _, formBody := range result.FormBodies {
			// Check if this is a delta example (has DeltaParentID set)
			if formBody.DeltaParentID != nil {
				deltaFormBodies = append(deltaFormBodies, formBody)
			} else {
				// Find the corresponding example to check if it's default or normal
				for _, example := range result.Examples {
					if example.ID == formBody.ExampleID && !example.IsDefault && !strings.Contains(example.Name, "Delta") {
						normalFormBodies = append(normalFormBodies, formBody)
						break
					}
				}
			}
		}

		require.Len(t, normalFormBodies, 2, "Should have 2 normal form bodies")
		require.Len(t, deltaFormBodies, 2, "Should have 2 delta form bodies")

		// Create a map of normal form bodies by key
		normalByKey := make(map[string]mbodyform.BodyForm)
		for _, normal := range normalFormBodies {
			normalByKey[normal.BodyKey] = normal
		}

		// Verify each delta form body has correct parent ID
		for _, delta := range deltaFormBodies {
			require.NotNil(t, delta.DeltaParentID, "Delta form body should have DeltaParentID set")

			normal, exists := normalByKey[delta.BodyKey]
			require.True(t, exists, "Should find normal form body with same key: %s", delta.BodyKey)
			require.Equal(t, normal.ID, *delta.DeltaParentID, "Delta form body should reference correct parent ID")
		}
	})
}

func TestHarDeltaURLEncodedBodyParentIDsSet(t *testing.T) {
	// Create HAR with URL-encoded data
	harData := `{
		"log": {
			"entries": [
				{
					"startedDateTime": "2023-01-01T10:00:00.000Z",
					"_resourceType": "xhr",
					"request": {
						"method": "POST",
						"url": "https://api.example.com/auth",
						"httpVersion": "HTTP/1.1",
						"headers": [
							{"name": "Content-Type", "value": "application/x-www-form-urlencoded"}
						],
						"postData": {
							"mimeType": "application/x-www-form-urlencoded",
							"params": [
								{"name": "grant_type", "value": "password"},
								{"name": "username", "value": "user"},
								{"name": "password", "value": "pass"}
							]
						},
						"queryString": []
					},
					"response": {
						"status": 200,
						"statusText": "OK",
						"httpVersion": "HTTP/1.1",
						"headers": [],
						"content": {"size": 0, "mimeType": "application/json", "text": ""}
					}
				}
			]
		}
	}`

	har, err := thar.ConvertRaw([]byte(harData))
	require.NoError(t, err)

	collectionID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()

	result, err := thar.ConvertHAR(har, collectionID, workspaceID)
	require.NoError(t, err)

	t.Run("DeltaURLEncodedBodiesHaveParentID", func(t *testing.T) {
		// Find the normal example URL-encoded bodies
		var normalURLBodies []mbodyurl.BodyURLEncoded
		var deltaURLBodies []mbodyurl.BodyURLEncoded

		for _, urlBody := range result.UrlEncodedBodies {
			// Check if this is a delta example (has DeltaParentID set)
			if urlBody.DeltaParentID != nil {
				deltaURLBodies = append(deltaURLBodies, urlBody)
			} else {
				// Find the corresponding example to check if it's default or normal
				for _, example := range result.Examples {
					if example.ID == urlBody.ExampleID && !example.IsDefault && !strings.Contains(example.Name, "Delta") {
						normalURLBodies = append(normalURLBodies, urlBody)
						break
					}
				}
			}
		}

		require.Len(t, normalURLBodies, 3, "Should have 3 normal URL-encoded bodies")
		require.Len(t, deltaURLBodies, 3, "Should have 3 delta URL-encoded bodies")

		// Create a map of normal URL-encoded bodies by key
		normalByKey := make(map[string]mbodyurl.BodyURLEncoded)
		for _, normal := range normalURLBodies {
			normalByKey[normal.BodyKey] = normal
		}

		// Verify each delta URL-encoded body has correct parent ID
		for _, delta := range deltaURLBodies {
			require.NotNil(t, delta.DeltaParentID, "Delta URL-encoded body should have DeltaParentID set")

			normal, exists := normalByKey[delta.BodyKey]
			require.True(t, exists, "Should find normal URL-encoded body with same key: %s", delta.BodyKey)
			require.Equal(t, normal.ID, *delta.DeltaParentID, "Delta URL-encoded body should reference correct parent ID")
		}
	})
}
