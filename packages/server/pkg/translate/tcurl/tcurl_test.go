package tcurl_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"the-dev-tools/server/pkg/idwrap"
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
