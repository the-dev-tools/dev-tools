package rhttp

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	httpv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
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

	require.NoError(t, f.hs.Create(f.ctx, httpModel), "create http")

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
	require.NoError(t, formService.Create(f.ctx, form), "create http body form")
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
	require.NoError(t, urlEncodedService.Create(f.ctx, urlEncoded), "create http body url encoded")
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
	require.NoError(t, err, "HttpRun failed")

	// Verify Content-Type header
	require.Contains(t, receivedContentType, "multipart/form-data", "Expected Content-Type to contain 'multipart/form-data'")

	// Verify boundary parameter
	_, params, err := mime.ParseMediaType(receivedContentType)
	require.NoError(t, err, "Failed to parse media type")
	boundary, ok := params["boundary"]
	require.True(t, ok, "Content-Type missing boundary parameter")
	require.NotEmpty(t, boundary, "boundary parameter should not be empty")

	// Verify form values
	require.NotNil(t, formValues, "Failed to parse multipart form")
	val, ok := formValues["username"]
	require.True(t, ok, "Expected form field 'username'")
	require.Equal(t, "testuser", val, "Expected form field 'username'='testuser'")
	val, ok = formValues["role"]
	require.True(t, ok, "Expected form field 'role'")
	require.Equal(t, "admin", val, "Expected form field 'role'='admin'")
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
	require.NoError(t, err, "HttpRun failed")

	// Verify Content-Type header
	require.Equal(t, "application/x-www-form-urlencoded", receivedContentType, "Expected Content-Type 'application/x-www-form-urlencoded'")

	// Verify body content
	require.NotNil(t, formValues, "Failed to parse form values")

	require.Equal(t, "go testing", formValues.Get("search"), "Expected form field 'search'='go testing'")
	require.Equal(t, "1", formValues.Get("page"), "Expected form field 'page'='1'")
}
