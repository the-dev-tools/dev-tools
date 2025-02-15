package thar_test

/*
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
	if len(resolved.RawBodies) != 2 {
		t.Errorf("Expected 4 Raw Body, got %d", len(resolved.RawBodies))
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
	if len(resolved.RawBodies) != 2 {
		t.Errorf("Expected 1 Raw Body, got %d", len(resolved.RawBodies))
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
	if len(resolved.RawBodies) != 2 {
		t.Errorf("Expected 1 Raw Body, got %d", len(resolved.RawBodies))
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
	if len(resolved.RawBodies) != 2 {
		t.Errorf("Expected 2 Raw Bodies, got %d", len(resolved.RawBodies))
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
	if len(resolved.RawBodies) != 6 {
		t.Errorf("Expected 6 Raw Bodies, got %d", len(resolved.RawBodies))
	}

	// Verify that GET (entry1) did not produce form or URL encoded bodies.
	// Adjust counts based on your conversion logic; here we assume each POST produces 2 bodies
	// specific to their MIME type.
	if len(resolved.FormBodies) != 2 {
		t.Errorf("Expected 2 Form Bodies, got %d", len(resolved.FormBodies))
	}

	if len(resolved.UrlEncodedBodies) != 2 {
		t.Errorf("Expected 2 UrlEncoded Bodies, got %d", len(resolved.UrlEncodedBodies))
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
*/
