package rhttp

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	httpv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"

	"connectrpc.com/connect"
)

func (f *httpFixture) createHttpWithBodyKind(t *testing.T, workspaceID idwrap.IDWrap, name, url, method string, bodyKind mhttp.HttpBodyKind) idwrap.IDWrap {
	t.Helper()

	httpID := idwrap.NewNow()
	httpModel := &mhttp.HTTP{
		ID:          httpID,
		WorkspaceID: workspaceID,
		Name:        name,
		Url:         url,
		Method:      method,
		Description: "Test HTTP entry with BodyKind",
		BodyKind:    bodyKind,
	}

	if err := f.hs.Create(f.ctx, httpModel); err != nil {
		t.Fatalf("create http: %v", err)
	}

	return httpID
}

func (f *httpFixture) createHttpBodyForm(t *testing.T, httpID idwrap.IDWrap, key, value string) {
	t.Helper()

	formID := idwrap.NewNow()
	form := &mhttp.HTTPBodyForm{
		ID:      formID,
		HttpID:  httpID,
		Key:     key,
		Value:   value,
		Enabled: true,
	}

	// Access the body form service from the handler
	formService := f.handler.httpBodyFormService
	if err := formService.Create(f.ctx, form); err != nil {
		t.Fatalf("create http body form: %v", err)
	}
}

func (f *httpFixture) createHttpBodyUrlEncoded(t *testing.T, httpID idwrap.IDWrap, key, value string) {
	t.Helper()

	urlEncodedID := idwrap.NewNow()
	urlEncoded := &mhttp.HTTPBodyUrlencoded{
		ID:      urlEncodedID,
		HttpID:  httpID,
		Key:     key,
		Value:   value,
		Enabled: true,
	}

	// Access the body url encoded service from the handler
	urlEncodedService := f.handler.httpBodyUrlEncodedService
	if err := urlEncodedService.Create(f.ctx, urlEncoded); err != nil {
		t.Fatalf("create http body url encoded: %v", err)
	}
}

func TestHttpRun_WithFormData(t *testing.T) {
	t.Parallel()

	var receivedContentType string
	var formValues map[string]string

	testServer := createTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")

		// Parse multipart form
		err := r.ParseMultipartForm(32 << 20) // 32 MB
		if err != nil {
			// Fallback to reading body directly if parsing fails
			io.ReadAll(r.Body)
		} else {
			formValues = make(map[string]string)
			for key, values := range r.MultipartForm.Value {
				if len(values) > 0 {
					formValues[key] = values[0]
				}
			}
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"success"}`)
	})
	defer testServer.Close()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttpWithBodyKind(t, ws, "test-http-form", testServer.URL, "POST", mhttp.HttpBodyKindFormData)

	// Add form data
	f.createHttpBodyForm(t, httpID, "username", "testuser")
	f.createHttpBodyForm(t, httpID, "role", "admin")

	req := connect.NewRequest(&httpv1.HttpRunRequest{
		HttpId: httpID.Bytes(),
	})

	_, err := f.handler.HttpRun(f.ctx, req)
	if err != nil {
		t.Fatalf("HttpRun failed: %v", err)
	}

	// Verify Content-Type header
	if !strings.Contains(receivedContentType, "multipart/form-data") {
		t.Errorf("Expected Content-Type to contain 'multipart/form-data', got '%s'", receivedContentType)
	}

	// Verify boundary parameter
	_, params, err := mime.ParseMediaType(receivedContentType)
	if err != nil {
		t.Fatalf("Failed to parse media type: %v", err)
	}
	if boundary, ok := params["boundary"]; !ok || boundary == "" {
		t.Error("Content-Type missing boundary parameter")
	}

	// Verify form values
	if formValues == nil {
		t.Fatal("Failed to parse multipart form")
	}
	if val, ok := formValues["username"]; !ok || val != "testuser" {
		t.Errorf("Expected form field 'username'='testuser', got '%s'", val)
	}
	if val, ok := formValues["role"]; !ok || val != "admin" {
		t.Errorf("Expected form field 'role'='admin', got '%s'", val)
	}
}

func TestHttpRun_WithUrlEncoded(t *testing.T) {
	t.Parallel()

	var receivedContentType string
	var receivedBody string
	var formValues url.Values

	testServer := createTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")

		// Read body
		bodyBytes, _ := io.ReadAll(r.Body)
		receivedBody = string(bodyBytes)

		// Parse form values
		r.Body = io.NopCloser(strings.NewReader(receivedBody)) // Reset body for parsing
		if err := r.ParseForm(); err == nil {
			formValues = r.PostForm
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"success"}`)
	})
	defer testServer.Close()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttpWithBodyKind(t, ws, "test-http-urlencoded", testServer.URL, "POST", mhttp.HttpBodyKindUrlEncoded)

	// Add url encoded data
	f.createHttpBodyUrlEncoded(t, httpID, "search", "go testing")
	f.createHttpBodyUrlEncoded(t, httpID, "page", "1")

	req := connect.NewRequest(&httpv1.HttpRunRequest{
		HttpId: httpID.Bytes(),
	})

	_, err := f.handler.HttpRun(f.ctx, req)
	if err != nil {
		t.Fatalf("HttpRun failed: %v", err)
	}

	// Verify Content-Type header
	if receivedContentType != "application/x-www-form-urlencoded" {
		t.Errorf("Expected Content-Type 'application/x-www-form-urlencoded', got '%s'", receivedContentType)
	}

	// Verify body content

	if formValues == nil {
		t.Fatal("Failed to parse form values")
	}

	if val := formValues.Get("search"); val != "go testing" {
		t.Errorf("Expected form field 'search'='go testing', got '%s'", val)
	}
	if val := formValues.Get("page"); val != "1" {
		t.Errorf("Expected form field 'page'='1', got '%s'", val)
	}
}
