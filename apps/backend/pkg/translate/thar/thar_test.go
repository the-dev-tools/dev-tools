package thar_test

import (
	"bytes"
	"testing"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/translate/thar"
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

	resolved, err := thar.ConvertHAR(&testHar, id)
	if err != nil {
		t.Errorf("Error converting HAR: %v", err)
	}

	if len(resolved.Apis) != 1 {
		t.Errorf("Expected 1 API, got %d", len(resolved.Apis))
	}
}

func TestHarResvoledBody(t *testing.T) {
	Entry := thar.Entry{}
	Entry.Request.Method = "GET"
	Entry.Request.URL = "http://example.com"
	Entry.Request.HTTPVersion = "HTTP/1.1"
	Entry.Request.Headers = []thar.Header{}
	testHar := thar.HAR{
		Log: thar.Log{
			Entries: []thar.Entry{Entry, Entry},
		},
	}
	id := idwrap.NewNow()

	resolved, err := thar.ConvertHAR(&testHar, id)
	if err != nil {
		t.Errorf("Error converting HAR: %v", err)
	}

	if len(resolved.Apis) != 2 {
		t.Errorf("Expected 1 API, got %d", len(resolved.Apis))
	}
	if len(resolved.RawBodies) != 4 {
		t.Errorf("Expected 1 Raw Body, got %d", len(resolved.RawBodies))
	}

	for i, rawBody := range resolved.RawBodies {
		if !bytes.Equal(rawBody.Data, []byte{}) {
			t.Errorf("Expected empty body, got %s", rawBody.Data)
		}

		if rawBody.ExampleID != resolved.Examples[i].ID {
			t.Errorf("Expected ExampleID to be %s, got %s", resolved.Examples[i].ID, rawBody.ExampleID)
		}
	}
}
