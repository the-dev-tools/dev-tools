package tcurl_test

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/translate/tcurl"
)

var curlStr string = `curl 'http://localhost:8080/collection.item.v1.CollectionItemService/CollectionItemList' \
  -H 'Accept: */*' \
  -H 'Accept-Language: en-US' \
  -H 'Connection: keep-alive' \
  -H 'Origin: http://localhost:5173' \
  -H 'Referer: http://localhost:5173/' \
  -H 'Sec-Fetch-Dest: empty' \
  -H 'Sec-Fetch-Mode: cors' \
  -H 'Sec-Fetch-Site: same-site' \
  -H 'User-Agent: Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.6834.210 Electron/34.3.3 Safari/537.36' \
  -H 'connect-protocol-version: 1' \
  -H 'content-type: application/json' \
  -H 'sec-ch-ua: "Not A(Brand";v="8", "Chromium";v="132"' \
  -H 'sec-ch-ua-mobile: ?0' \
  -H 'sec-ch-ua-platform: "Linux"' \
  --data-raw '{"collectionId":"AZX2h4p7aJyUSR0lYMfcfQ=="}' ;
`

const (
	curlFormSample       = `curl 'http://example.com/api' -F 'field1=value1' -F 'field2=value2'`
	curlURLEncodedSample = `curl 'http://example.com/api' --data-urlencode 'param1=value1' --data-urlencode 'param2=value2'`
	curlQuerySample      = `curl 'http://example.com/api?param1=value1&param2=value2'`
)

type curlSnapshot struct {
	Method     string
	URL        string
	Headers    map[string]string
	Queries    map[string]string
	RawBody    string
	FormFields map[string]string
	UrlEncoded map[string]string
}

func snapshotFromResolved(t *testing.T, resolved tcurl.CurlResolved) curlSnapshot {
	t.Helper()
	snapshot := curlSnapshot{
		Headers:    map[string]string{},
		Queries:    map[string]string{},
		FormFields: map[string]string{},
		UrlEncoded: map[string]string{},
	}

	if len(resolved.Apis) == 0 {
		return snapshot
	}

	api := resolved.Apis[0]
	snapshot.Method = strings.ToUpper(api.Method)
	snapshot.URL = api.Url

	example := pickExampleForAPI(resolved, api.ID)
	if example == nil {
		return snapshot
	}

	exampleID := example.ID

	for _, header := range resolved.Headers {
		if header.ExampleID == exampleID && header.Enable {
			snapshot.Headers[header.HeaderKey] = header.Value
		}
	}

	for _, query := range resolved.Queries {
		if query.ExampleID == exampleID && query.Enable {
			snapshot.Queries[query.QueryKey] = query.Value
		}
	}

	snapshot.RawBody = extractRawBody(t, resolved, exampleID)

	for _, form := range resolved.FormBodies {
		if form.ExampleID == exampleID && form.IsEnabled() {
			snapshot.FormFields[form.BodyKey] = form.Value
		}
	}

	for _, body := range resolved.UrlEncodedBodies {
		if body.ExampleID == exampleID && body.IsEnabled() {
			snapshot.UrlEncoded[body.BodyKey] = body.Value
		}
	}

	return snapshot
}

func pickExampleForAPI(resolved tcurl.CurlResolved, apiID idwrap.IDWrap) *mitemapiexample.ItemApiExample {
	var fallback *mitemapiexample.ItemApiExample
	for i := range resolved.Examples {
		example := &resolved.Examples[i]
		if example.ItemApiID != apiID {
			continue
		}
		if example.IsDefault {
			return example
		}
		if fallback == nil {
			fallback = example
		}
	}
	return fallback
}

func extractRawBody(t *testing.T, resolved tcurl.CurlResolved, exampleID idwrap.IDWrap) string {
	t.Helper()
	for _, body := range resolved.RawBodies {
		if body.ExampleID != exampleID {
			continue
		}
		data := body.Data
		if body.CompressType != compress.CompressTypeNone && len(body.Data) > 0 {
			decompressed, err := compress.Decompress(body.Data, body.CompressType)
			if err != nil {
				t.Fatalf("failed to decompress raw body: %v", err)
			}
			data = decompressed
		}
		return string(data)
	}
	return ""
}

func compareSnapshots(t *testing.T, want, got curlSnapshot) {
	t.Helper()
	if want.Method != got.Method {
		t.Fatalf("method mismatch: want %s, got %s", want.Method, got.Method)
	}
	if want.URL != got.URL {
		t.Fatalf("url mismatch: want %s, got %s", want.URL, got.URL)
	}
	if !reflect.DeepEqual(want.Headers, got.Headers) {
		t.Fatalf("headers mismatch: want %v, got %v", want.Headers, got.Headers)
	}
	if !reflect.DeepEqual(want.Queries, got.Queries) {
		t.Fatalf("queries mismatch: want %v, got %v", want.Queries, got.Queries)
	}
	if want.RawBody != got.RawBody {
		t.Fatalf("raw body mismatch: want %s, got %s", want.RawBody, got.RawBody)
	}
	if !reflect.DeepEqual(want.FormFields, got.FormFields) {
		t.Fatalf("form fields mismatch: want %v, got %v", want.FormFields, got.FormFields)
	}
	if !reflect.DeepEqual(want.UrlEncoded, got.UrlEncoded) {
		t.Fatalf("urlencoded fields mismatch: want %v, got %v", want.UrlEncoded, got.UrlEncoded)
	}
}

func TestBuildCurlRoundTripRaw(t *testing.T) {
	resolved, err := tcurl.ConvertCurl(curlStr, idwrap.NewNow())
	if err != nil {
		t.Fatalf("ConvertCurl failed: %v", err)
	}

	built, err := tcurl.BuildCurl(resolved)
	if err != nil {
		t.Fatalf("BuildCurl failed: %v", err)
	}

	if !strings.Contains(built, "--data-raw") {
		t.Fatalf("expected raw data flag in built curl, got %s", built)
	}

	roundTrip, err := tcurl.ConvertCurl(built, idwrap.NewNow())
	if err != nil {
		t.Fatalf("ConvertCurl (round trip) failed: %v", err)
	}

	compareSnapshots(t, snapshotFromResolved(t, resolved), snapshotFromResolved(t, roundTrip))
}

func TestBuildCurlRoundTripForm(t *testing.T) {
	resolved, err := tcurl.ConvertCurl(curlFormSample, idwrap.NewNow())
	if err != nil {
		t.Fatalf("ConvertCurl failed: %v", err)
	}

	built, err := tcurl.BuildCurl(resolved)
	if err != nil {
		t.Fatalf("BuildCurl failed: %v", err)
	}

	if !strings.Contains(built, "-F 'field1=value1'") {
		t.Fatalf("expected form flag in built curl, got %s", built)
	}

	roundTrip, err := tcurl.ConvertCurl(built, idwrap.NewNow())
	if err != nil {
		t.Fatalf("ConvertCurl (round trip) failed: %v", err)
	}

	compareSnapshots(t, snapshotFromResolved(t, resolved), snapshotFromResolved(t, roundTrip))
}

func TestBuildCurlRoundTripUrlEncoded(t *testing.T) {
	resolved, err := tcurl.ConvertCurl(curlURLEncodedSample, idwrap.NewNow())
	if err != nil {
		t.Fatalf("ConvertCurl failed: %v", err)
	}

	built, err := tcurl.BuildCurl(resolved)
	if err != nil {
		t.Fatalf("BuildCurl failed: %v", err)
	}

	if !strings.Contains(built, "--data-urlencode 'param1=value1'") {
		t.Fatalf("expected data-urlencode in built curl, got %s", built)
	}

	roundTrip, err := tcurl.ConvertCurl(built, idwrap.NewNow())
	if err != nil {
		t.Fatalf("ConvertCurl (round trip) failed: %v", err)
	}

	compareSnapshots(t, snapshotFromResolved(t, resolved), snapshotFromResolved(t, roundTrip))
}

func TestBuildCurlRoundTripQuery(t *testing.T) {
	resolved, err := tcurl.ConvertCurl(curlQuerySample, idwrap.NewNow())
	if err != nil {
		t.Fatalf("ConvertCurl failed: %v", err)
	}

	built, err := tcurl.BuildCurl(resolved)
	if err != nil {
		t.Fatalf("BuildCurl failed: %v", err)
	}

	if !strings.Contains(built, "?param1=value1&param2=value2") {
		t.Fatalf("expected query parameters in built curl url, got %s", built)
	}

	roundTrip, err := tcurl.ConvertCurl(built, idwrap.NewNow())
	if err != nil {
		t.Fatalf("ConvertCurl (round trip) failed: %v", err)
	}

	compareSnapshots(t, snapshotFromResolved(t, resolved), snapshotFromResolved(t, roundTrip))
}

func TestBuildCurlNoAPIs(t *testing.T) {
	if _, err := tcurl.BuildCurl(tcurl.CurlResolved{}); err == nil {
		t.Fatal("expected error when building curl without apis")
	}
}

func TestCurl(t *testing.T) {
	curlResolved, err := tcurl.ConvertCurl(curlStr, idwrap.NewNow())
	if err != nil {
		t.Errorf("Failed to convert curl command: %v", err)
	}

	// Basic validation
	if len(curlResolved.Apis) != 1 {
		t.Errorf("Expected 1 API, got %d", len(curlResolved.Apis))
	}
}

func TestCurlHeaders(t *testing.T) {
	curlResolved, err := tcurl.ConvertCurl(curlStr, idwrap.NewNow())
	if err != nil {
		t.Errorf("Failed to convert curl command: %v", err)
	}

	// Header validation
	if len(curlResolved.Headers) != 14 {
		for _, header := range curlResolved.Headers {
			fmt.Printf("Header: %s: %s\n", header.HeaderKey, header.Value)
		}
		t.Errorf("Expected 14 headers, got %d", len(curlResolved.Headers))
	}

	// Check for specific headers
	var contentTypeFound bool
	for _, header := range curlResolved.Headers {
		if header.HeaderKey == "content-type" && header.Value == "application/json" {
			contentTypeFound = true
			break
		}
	}
	if !contentTypeFound {
		t.Error("Expected to find content-type header with value application/json")
	}
}

func TestCurlRawBody(t *testing.T) {
	curlResolved, err := tcurl.ConvertCurl(curlStr, idwrap.NewNow())
	if err != nil {
		t.Errorf("Failed to convert curl command: %v", err)
	}

	// Raw body validation
	if len(curlResolved.RawBodies) != 1 {
		t.Errorf("Expected 1 raw body, got %d", len(curlResolved.RawBodies))
	}

	if len(curlResolved.RawBodies) > 0 {
		expectedData := []byte(`{"collectionId":"AZX2h4p7aJyUSR0lYMfcfQ=="}`)
		if !bytes.Equal(curlResolved.RawBodies[0].Data, expectedData) {
			t.Errorf("Expected body %s, got %s", expectedData, curlResolved.RawBodies[0].Data)
		}
	}
}

func TestCurlHttpMethods(t *testing.T) {
	// Test GET method (default)
	getCurl := `curl 'http://example.com/api'`
	getResolved, err := tcurl.ConvertCurl(getCurl, idwrap.NewNow())
	if err != nil {
		t.Errorf("Failed to convert GET curl command: %v", err)
	}
	if len(getResolved.Apis) == 0 || getResolved.Apis[0].Method != "GET" {
		t.Errorf("Expected GET method, got %s", getResolved.Apis[0].Method)
	}

	// Test POST method (explicit)
	postCurl := `curl -X POST 'http://example.com/api'`
	postResolved, err := tcurl.ConvertCurl(postCurl, idwrap.NewNow())
	if err != nil {
		t.Errorf("Failed to convert POST curl command: %v", err)
	}
	if len(postResolved.Apis) == 0 || postResolved.Apis[0].Method != "POST" {
		t.Errorf("Expected POST method, got %s", postResolved.Apis[0].Method)
	}

	// Test implicit POST method (with data flag)
	implicitPostCurl := `curl 'http://example.com/api' -d 'data'`
	implicitPostResolved, err := tcurl.ConvertCurl(implicitPostCurl, idwrap.NewNow())
	if err != nil {
		t.Errorf("Failed to convert implicit POST curl command: %v", err)
	}
	if len(implicitPostResolved.Apis) == 0 || implicitPostResolved.Apis[0].Method != "POST" {
		t.Errorf("Expected implicit POST method, got %s", implicitPostResolved.Apis[0].Method)
	}
}

func TestCurlQueryParams(t *testing.T) {
	curlWithQuery := `curl 'http://example.com/api?param1=value1&param2=value2'`
	resolved, err := tcurl.ConvertCurl(curlWithQuery, idwrap.NewNow())
	if err != nil {
		t.Errorf("Failed to convert curl command with query: %v", err)
	}

	// Check if query parameters were parsed
	if len(resolved.Queries) != 2 {
		t.Errorf("Expected 2 query parameters, got %d", len(resolved.Queries))
	}

	// Verify specific query parameters
	var param1Found, param2Found bool
	for _, query := range resolved.Queries {
		if query.QueryKey == "param1" && query.Value == "value1" {
			param1Found = true
		}
		if query.QueryKey == "param2" && query.Value == "value2" {
			param2Found = true
		}
	}

	if !param1Found {
		t.Error("Query parameter param1=value1 not found")
	}
	if !param2Found {
		t.Error("Query parameter param2=value2 not found")
	}

	// Check that URL doesn't contain query parameters
	if len(resolved.Apis) > 0 {
		if strings.Contains(resolved.Apis[0].Url, "?") {
			t.Errorf("URL should not contain query parameters: %s", resolved.Apis[0].Url)
		}
	}
}

func TestCurlFormBody(t *testing.T) {
	curlWithForm := `curl 'http://example.com/api' -F 'field1=value1' -F 'field2=value2'`
	resolved, err := tcurl.ConvertCurl(curlWithForm, idwrap.NewNow())
	if err != nil {
		t.Errorf("Failed to convert curl command with form data: %v", err)
	}

	// Check if form bodies were parsed
	if len(resolved.FormBodies) != 2 {
		t.Errorf("Expected 2 form bodies, got %d", len(resolved.FormBodies))
	}

	// Verify specific form fields
	var field1Found, field2Found bool
	for _, form := range resolved.FormBodies {
		if form.BodyKey == "field1" && form.Value == "value1" {
			field1Found = true
		}
		if form.BodyKey == "field2" && form.Value == "value2" {
			field2Found = true
		}
	}

	if !field1Found {
		t.Error("Form field field1=value1 not found")
	}
	if !field2Found {
		t.Error("Form field field2=value2 not found")
	}
}

func TestCurlUrlEncodedBody(t *testing.T) {
	curlWithUrlEncoded := `curl 'http://example.com/api' --data-urlencode 'param1=value1' --data-urlencode 'param2=value2'`
	resolved, err := tcurl.ConvertCurl(curlWithUrlEncoded, idwrap.NewNow())
	if err != nil {
		t.Errorf("Failed to convert curl command with url-encoded data: %v", err)
	}

	// Check if url-encoded bodies were parsed
	if len(resolved.UrlEncodedBodies) != 2 {
		t.Errorf("Expected 2 url-encoded bodies, got %d", len(resolved.UrlEncodedBodies))
	}

	// Verify specific url-encoded parameters
	var param1Found, param2Found bool
	for _, urlBody := range resolved.UrlEncodedBodies {
		if urlBody.BodyKey == "param1" && urlBody.Value == "value1" {
			param1Found = true
		}
		if urlBody.BodyKey == "param2" && urlBody.Value == "value2" {
			param2Found = true
		}
	}

	if !param1Found {
		t.Error("URL-encoded parameter param1=value1 not found")
	}
	if !param2Found {
		t.Error("URL-encoded parameter param2=value2 not found")
	}
}

func TestInvalidCurl(t *testing.T) {
	invalidCurl := `not a curl command`
	_, err := tcurl.ConvertCurl(invalidCurl, idwrap.NewNow())
	if err == nil {
		t.Error("Expected error for invalid curl command, got nil")
	}
}

func TestSimpleOneLine(t *testing.T) {
	oneLinerCurl := `curl https://api.example.com/v1/data -H "Authorization: Bearer token123"`
	resolved, err := tcurl.ConvertCurl(oneLinerCurl, idwrap.NewNow())
	if err != nil {
		t.Errorf("Failed to convert one-line curl command: %v", err)
	}

	// Verify URL
	if len(resolved.Apis) != 1 || resolved.Apis[0].Url != "https://api.example.com/v1/data" {
		t.Errorf("Expected URL 'https://api.example.com/v1/data', got %s", resolved.Apis[0].Url)
	}

	// Verify header
	if len(resolved.Headers) != 1 || resolved.Headers[0].HeaderKey != "Authorization" ||
		resolved.Headers[0].Value != "Bearer token123" {
		t.Error("Authorization header not correctly parsed")
	}
}

// TestIndexOutOfRangeBug tests the specific case that causes index out of range panic
func TestIndexOutOfRangeBug(t *testing.T) {
	// This curl command would trigger the index out of range panic before the fix
	// The issue occurs when the URL regex doesn't match and it falls back to field parsing
	problematicCurl := `-L https://example.com`

	// This should not panic, it should either extract the URL or return empty string
	_, err := tcurl.ConvertCurl(problematicCurl, idwrap.NewNow())
	// We expect an error because it's not a valid curl command, but it shouldn't panic
	if err == nil {
		t.Error("Expected error for invalid curl command starting with -L, got nil")
	}
}

// TestExtractURLDirectly tests the extractURL function directly with problematic inputs
func TestExtractURLDirectly(t *testing.T) {
	// Test cases that previously caused panic
	testCases := []struct {
		input    string
		expected string
		name     string
	}{
		{"curl", "", "empty_curl_command"},
		{"curl ", "", "curl_with_space"},
		{"curl -L", "", "curl_with_L_flag_only"},
		{"curl -X GET", "", "curl_with_method_only"},
		{"curl -L https://example.com", "https://example.com", "curl_with_L_and_url"},
		{"curl https://example.com", "https://example.com", "normal_curl_with_url"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// This should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("extractURL panicked for input '%s': %v", tc.input, r)
				}
			}()

			result := tcurl.ExtractURLForTesting(tc.input)
			if result != tc.expected {
				t.Errorf("Expected '%s', got '%s' for input '%s'", tc.expected, result, tc.input)
			}
		})
	}
}

// TestEdgeCaseEmptyFields tests edge cases with empty or minimal input
func TestEdgeCaseEmptyFields(t *testing.T) {
	testCases := []string{
		"",        // Empty string
		"curl",    // Just curl command
		"curl ",   // curl with space
		"-L",      // Just -L flag
		"curl -L", // curl with -L but no URL
	}

	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("input_%s", strings.ReplaceAll(testCase, " ", "_space_")), func(t *testing.T) {
			// These should not panic, even if they return errors
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Panic occurred for input '%s': %v", testCase, r)
				}
			}()

			_, err := tcurl.ConvertCurl(testCase, idwrap.NewNow())
			// We expect errors for these invalid inputs, but no panics
			if err == nil && testCase != "" && testCase != "curl" && testCase != "curl " {
				t.Logf("Unexpectedly succeeded for input '%s'", testCase)
			}
		})
	}
}

// TestProtocolLessURL tests parsing URLs without protocol (http:// or https://)
func TestProtocolLessURL(t *testing.T) {
	testCases := []struct {
		input       string
		expectedURL string
		shouldFail  bool
		name        string
	}{
		{"curl google.com", "google.com", false, "google_com_no_protocol"},
		{"curl example.com", "example.com", false, "example_com_no_protocol"},
		{"curl www.example.com", "www.example.com", false, "www_example_com"},
		{"curl https://google.com", "https://google.com", false, "google_com_with_https"},
		{"curl http://google.com", "http://google.com", false, "google_com_with_http"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// This should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("ConvertCurl panicked for input '%s': %v", tc.input, r)
				}
			}()

			resolved, err := tcurl.ConvertCurl(tc.input, idwrap.NewNow())
			if tc.shouldFail {
				if err == nil {
					t.Errorf("Expected error for input '%s', got nil", tc.input)
				}
				return
			}

			if err != nil {
				t.Errorf("Failed to convert curl command '%s': %v", tc.input, err)
				return
			}

			if len(resolved.Apis) != 1 {
				t.Errorf("Expected 1 API for '%s', got %d", tc.input, len(resolved.Apis))
				return
			}

			if resolved.Apis[0].Url != tc.expectedURL {
				t.Errorf("Expected URL '%s' for input '%s', got '%s'", tc.expectedURL, tc.input, resolved.Apis[0].Url)
			}
		})
	}
}
