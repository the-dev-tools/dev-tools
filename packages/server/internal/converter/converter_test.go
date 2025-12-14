package converter

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	filev1 "the-dev-tools/spec/dist/buf/go/api/file_system/v1"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
	httpv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
)

func TestToAPIHttp(t *testing.T) {
	httpID := idwrap.NewNow()
	now := time.Now().Unix()

	tests := []struct {
		name     string
		input    mhttp.HTTP
		expected func(*httpv1.Http)
	}{
		{
			name: "Basic conversion",
			input: mhttp.HTTP{
				ID:       httpID,
				Name:     "Test Request",
				Url:      "https://api.example.com",
				Method:   "GET",
				BodyKind: mhttp.HttpBodyKindNone,
			},
			expected: func(res *httpv1.Http) {
				assert.Equal(t, httpID.Bytes(), res.HttpId)
				assert.Equal(t, "Test Request", res.Name)
				assert.Equal(t, "https://api.example.com", res.Url)
				assert.Equal(t, httpv1.HttpMethod_HTTP_METHOD_GET, res.Method)
				assert.Equal(t, httpv1.HttpBodyKind_HTTP_BODY_KIND_UNSPECIFIED, res.BodyKind)
				assert.Nil(t, res.LastRunAt)
			},
		},
		{
			name: "With LastRunAt",
			input: mhttp.HTTP{
				ID:        httpID,
				Name:      "Test Request",
				Url:       "https://api.example.com",
				Method:    "POST",
				BodyKind:  mhttp.HttpBodyKindRaw,
				LastRunAt: &now,
			},
			expected: func(res *httpv1.Http) {
				assert.Equal(t, httpID.Bytes(), res.HttpId)
				assert.Equal(t, httpv1.HttpMethod_HTTP_METHOD_POST, res.Method)
				assert.Equal(t, httpv1.HttpBodyKind_HTTP_BODY_KIND_RAW, res.BodyKind)
				assert.NotNil(t, res.LastRunAt)
				assert.Equal(t, now, res.LastRunAt.Seconds)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := ToAPIHttp(tt.input)
			tt.expected(res)
		})
	}
}

func TestToAPIHttpMethod(t *testing.T) {
	tests := []struct {
		input    string
		expected httpv1.HttpMethod
	}{
		{"GET", httpv1.HttpMethod_HTTP_METHOD_GET},
		{"POST", httpv1.HttpMethod_HTTP_METHOD_POST},
		{"PUT", httpv1.HttpMethod_HTTP_METHOD_PUT},
		{"PATCH", httpv1.HttpMethod_HTTP_METHOD_PATCH},
		{"DELETE", httpv1.HttpMethod_HTTP_METHOD_DELETE},
		{"HEAD", httpv1.HttpMethod_HTTP_METHOD_HEAD},
		{"OPTION", httpv1.HttpMethod_HTTP_METHOD_OPTION},
		{"CONNECT", httpv1.HttpMethod_HTTP_METHOD_CONNECT},
		{"UNKNOWN", httpv1.HttpMethod_HTTP_METHOD_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, ToAPIHttpMethod(tt.input))
		})
	}
}

func TestFromAPIHttpMethod(t *testing.T) {
	tests := []struct {
		input    httpv1.HttpMethod
		expected string
	}{
		{httpv1.HttpMethod_HTTP_METHOD_GET, "GET"},
		{httpv1.HttpMethod_HTTP_METHOD_POST, "POST"},
		{httpv1.HttpMethod_HTTP_METHOD_PUT, "PUT"},
		{httpv1.HttpMethod_HTTP_METHOD_PATCH, "PATCH"},
		{httpv1.HttpMethod_HTTP_METHOD_DELETE, "DELETE"},
		{httpv1.HttpMethod_HTTP_METHOD_HEAD, "HEAD"},
		{httpv1.HttpMethod_HTTP_METHOD_OPTION, "OPTION"},
		{httpv1.HttpMethod_HTTP_METHOD_CONNECT, "CONNECT"},
		{httpv1.HttpMethod_HTTP_METHOD_UNSPECIFIED, ""},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, FromAPIHttpMethod(tt.input))
		})
	}
}

func TestToAPIHttpBodyKind(t *testing.T) {
	tests := []struct {
		input    mhttp.HttpBodyKind
		expected httpv1.HttpBodyKind
	}{
		{mhttp.HttpBodyKindNone, httpv1.HttpBodyKind_HTTP_BODY_KIND_UNSPECIFIED},
		{mhttp.HttpBodyKindFormData, httpv1.HttpBodyKind_HTTP_BODY_KIND_FORM_DATA},
		{mhttp.HttpBodyKindUrlEncoded, httpv1.HttpBodyKind_HTTP_BODY_KIND_URL_ENCODED},
		{mhttp.HttpBodyKindRaw, httpv1.HttpBodyKind_HTTP_BODY_KIND_RAW},
		{mhttp.HttpBodyKind(-1), httpv1.HttpBodyKind_HTTP_BODY_KIND_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d", tt.input), func(t *testing.T) {
			assert.Equal(t, tt.expected, ToAPIHttpBodyKind(tt.input))
		})
	}
}

func TestFromAPIHttpBodyKind(t *testing.T) {
	tests := []struct {
		input    httpv1.HttpBodyKind
		expected mhttp.HttpBodyKind
	}{
		{httpv1.HttpBodyKind_HTTP_BODY_KIND_UNSPECIFIED, mhttp.HttpBodyKindNone},
		{httpv1.HttpBodyKind_HTTP_BODY_KIND_FORM_DATA, mhttp.HttpBodyKindFormData},
		{httpv1.HttpBodyKind_HTTP_BODY_KIND_URL_ENCODED, mhttp.HttpBodyKindUrlEncoded},
		{httpv1.HttpBodyKind_HTTP_BODY_KIND_RAW, mhttp.HttpBodyKindRaw},
		{httpv1.HttpBodyKind(-1), mhttp.HttpBodyKindNone},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d", tt.expected), func(t *testing.T) {
			assert.Equal(t, tt.expected, FromAPIHttpBodyKind(tt.input))
		})
	}
}

func TestToAPIHttpHeader(t *testing.T) {
	headerID := idwrap.NewNow()
	httpID := idwrap.NewNow()

	header := mhttp.HTTPHeader{
		ID:          headerID,
		HttpID:      httpID,
		Key:         "Content-Type",
		Value:       "application/json",
		Enabled:     true,
		Description: "The content type",
		Order:       1,
	}

	res := ToAPIHttpHeader(header)

	assert.Equal(t, headerID.Bytes(), res.HttpHeaderId)
	assert.Equal(t, httpID.Bytes(), res.HttpId)
	assert.Equal(t, "Content-Type", res.Key)
	assert.Equal(t, "application/json", res.Value)
	assert.True(t, res.Enabled)
	assert.Equal(t, "The content type", res.Description)
	assert.Equal(t, float32(1), res.Order)
}

func TestToAPIHttpSearchParam(t *testing.T) {
	paramID := idwrap.NewNow()
	httpID := idwrap.NewNow()

	param := mhttp.HTTPSearchParam{
		ID:          paramID,
		HttpID:      httpID,
		Key:         "page",
		Value:       "1",
		Enabled:     true,
		Description: "Page number",
		Order:       1,
	}

	res := ToAPIHttpSearchParam(param)

	assert.Equal(t, paramID.Bytes(), res.HttpSearchParamId)
	assert.Equal(t, httpID.Bytes(), res.HttpId)
	assert.Equal(t, "page", res.Key)
	assert.Equal(t, "1", res.Value)
	assert.True(t, res.Enabled)
	assert.Equal(t, "Page number", res.Description)
	assert.Equal(t, float32(1), res.Order)
}

func TestToAPIHttpSearchParamFromMHttp(t *testing.T) {
	paramID := idwrap.NewNow()
	httpID := idwrap.NewNow()

	param := mhttp.HTTPSearchParam{
		ID:          paramID,
		HttpID:      httpID,
		Key:         "q",
		Value:       "search",
		Enabled:     false,
		Description: "Query",
		Order:       5, // Should be ignored in FromMHttp
	}

	res := ToAPIHttpSearchParamFromMHttp(param)

	assert.Equal(t, paramID.Bytes(), res.HttpSearchParamId)
	assert.Equal(t, httpID.Bytes(), res.HttpId)
	assert.Equal(t, "q", res.Key)
	assert.Equal(t, "search", res.Value)
	assert.False(t, res.Enabled)
	assert.Equal(t, "Query", res.Description)
	assert.Equal(t, float32(0), res.Order) // Order is hardcoded to 0
}

func TestToAPIHttpBodyFormData(t *testing.T) {
	formID := idwrap.NewNow()
	httpID := idwrap.NewNow()

	form := mhttp.HTTPBodyForm{
		ID:          formID,
		HttpID:      httpID,
		Key:         "file",
		Value:       "test.txt",
		Enabled:     true,
		Description: "File upload",
		Order:       2,
	}

	res := ToAPIHttpBodyFormData(form)

	assert.Equal(t, formID.Bytes(), res.HttpBodyFormDataId)
	assert.Equal(t, httpID.Bytes(), res.HttpId)
	assert.Equal(t, "file", res.Key)
	assert.Equal(t, "test.txt", res.Value)
	assert.True(t, res.Enabled)
	assert.Equal(t, "File upload", res.Description)
	assert.Equal(t, float32(2), res.Order)
}

func TestToAPIHttpBodyFormDataFromMHttp(t *testing.T) {
	formID := idwrap.NewNow()
	httpID := idwrap.NewNow()

	form := mhttp.HTTPBodyForm{
		ID:          formID,
		HttpID:      httpID,
		Key:         "username",
		Value:       "admin",
		Enabled:     true,
		Description: "Login username",
		Order:       1,
	}

	res := ToAPIHttpBodyFormDataFromMHttp(form)

	assert.Equal(t, formID.Bytes(), res.HttpBodyFormDataId)
	assert.Equal(t, httpID.Bytes(), res.HttpId)
	assert.Equal(t, "username", res.Key)
	assert.Equal(t, "admin", res.Value)
	assert.True(t, res.Enabled)
	assert.Equal(t, "Login username", res.Description)
	assert.Equal(t, float32(1), res.Order)
}

func TestToAPIHttpBodyUrlEncoded(t *testing.T) {
	urlEncID := idwrap.NewNow()
	httpID := idwrap.NewNow()

	urlEnc := mhttp.HTTPBodyUrlencoded{
		ID:          urlEncID,
		HttpID:      httpID,
		Key:         "token",
		Value:       "123",
		Enabled:     true,
		Description: "Auth token",
		Order:       1,
	}

	res := ToAPIHttpBodyUrlEncoded(urlEnc)

	assert.Equal(t, urlEncID.Bytes(), res.HttpBodyUrlEncodedId)
	assert.Equal(t, httpID.Bytes(), res.HttpId)
	assert.Equal(t, "token", res.Key)
	assert.Equal(t, "123", res.Value)
	assert.True(t, res.Enabled)
	assert.Equal(t, "Auth token", res.Description)
	assert.Equal(t, float32(1), res.Order)
}

func TestToAPIHttpBodyUrlEncodedFromMHttp(t *testing.T) {
	urlEncID := idwrap.NewNow()
	httpID := idwrap.NewNow()

	urlEnc := mhttp.HTTPBodyUrlencoded{
		ID:          urlEncID,
		HttpID:      httpID,
		Key:         "scope",
		Value:       "read",
		Enabled:     false,
		Description: "Access scope",
		Order:       2,
	}

	res := ToAPIHttpBodyUrlEncodedFromMHttp(urlEnc)

	assert.Equal(t, urlEncID.Bytes(), res.HttpBodyUrlEncodedId)
	assert.Equal(t, httpID.Bytes(), res.HttpId)
	assert.Equal(t, "scope", res.Key)
	assert.Equal(t, "read", res.Value)
	assert.False(t, res.Enabled)
	assert.Equal(t, "Access scope", res.Description)
	assert.Equal(t, float32(2), res.Order)
}

func TestToAPIHttpBodyRaw(t *testing.T) {
	httpID := idwrap.NewNow()
	data := "raw data"

	res := ToAPIHttpBodyRaw(httpID.Bytes(), data)

	assert.Equal(t, httpID.Bytes(), res.HttpId)
	assert.Equal(t, data, res.Data)
}

func TestToAPIHttpBodyRawFromMHttp(t *testing.T) {
	httpID := idwrap.NewNow()

	tests := []struct {
		name     string
		input    mhttp.HTTPBodyRaw
		expected string
	}{
		{
			name: "Regular raw data",
			input: mhttp.HTTPBodyRaw{
				HttpID:  httpID,
				RawData: []byte("original data"),
				IsDelta: false,
			},
			expected: "original data",
		},
		{
			name: "Delta data exists",
			input: mhttp.HTTPBodyRaw{
				HttpID:       httpID,
				RawData:      []byte("original data"),
				DeltaRawData: []byte("delta data"),
				IsDelta:      true,
			},
			expected: "delta data",
		},
		{
			name: "Delta flag true but no delta data",
			input: mhttp.HTTPBodyRaw{
				HttpID:       httpID,
				RawData:      []byte("original data"),
				DeltaRawData: []byte{},
				IsDelta:      true,
			},
			expected: "original data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := ToAPIHttpBodyRawFromMHttp(tt.input)
			assert.Equal(t, httpID.Bytes(), res.HttpId)
			assert.Equal(t, tt.expected, res.Data)
		})
	}
}

func TestToAPIHttpAssert(t *testing.T) {
	assertID := idwrap.NewNow()
	httpID := idwrap.NewNow()

	assertion := mhttp.HTTPAssert{
		ID:     assertID,
		HttpID: httpID,
		Value:  "status == 200",
	}

	res := ToAPIHttpAssert(assertion)

	assert.Equal(t, assertID.Bytes(), res.HttpAssertId)
	assert.Equal(t, httpID.Bytes(), res.HttpId)
	assert.Equal(t, "status == 200", res.Value)
}

func TestToAPIHttpVersion(t *testing.T) {
	versionID := idwrap.NewNow()
	httpID := idwrap.NewNow()
	now := time.Now().Unix()

	version := mhttp.HttpVersion{
		ID:                 versionID,
		HttpID:             httpID,
		VersionName:        "v1.0",
		VersionDescription: "Initial version",
		CreatedAt:          now,
	}

	res := ToAPIHttpVersion(version)

	assert.Equal(t, versionID.Bytes(), res.HttpVersionId)
	assert.Equal(t, httpID.Bytes(), res.HttpId)
	assert.Equal(t, "v1.0", res.Name)
	assert.Equal(t, "Initial version", res.Description)
	assert.Equal(t, now, res.CreatedAt)
}

func TestToAPIHttpResponse(t *testing.T) {
	respID := idwrap.NewNow()
	httpID := idwrap.NewNow()
	now := time.Now().Unix()

	tests := []struct {
		name         string
		input        mhttp.HTTPResponse
		expectedBody string
	}{
		{
			name: "Valid UTF-8 body",
			input: mhttp.HTTPResponse{
				ID:       respID,
				HttpID:   httpID,
				Status:   200,
				Body:     []byte("{\"success\": true}"),
				Time:     now,
				Duration: 100,
				Size:     15,
			},
			expectedBody: "{\"success\": true}",
		},
		{
			name: "Binary body",
			input: mhttp.HTTPResponse{
				ID:       respID,
				HttpID:   httpID,
				Status:   200,
				Body:     []byte{0xFF, 0xFE, 0x00}, // Invalid UTF-8
				Time:     now,
				Duration: 100,
				Size:     3,
			},
			expectedBody: "[Binary data: 3 bytes]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := ToAPIHttpResponse(tt.input)
			assert.Equal(t, respID.Bytes(), res.HttpResponseId)
			assert.Equal(t, httpID.Bytes(), res.HttpId)
			assert.Equal(t, int32(200), res.Status)
			assert.Equal(t, tt.expectedBody, res.Body)
			assert.Equal(t, now, res.Time.Seconds)
			assert.Equal(t, int32(100), res.Duration)
			assert.Equal(t, int32(tt.input.Size), res.Size)
		})
	}
}

func TestToAPIHttpResponseHeader(t *testing.T) {
	headerID := idwrap.NewNow()
	respID := idwrap.NewNow()

	header := mhttp.HTTPResponseHeader{
		ID:          headerID,
		ResponseID:  respID,
		HeaderKey:   "Content-Type",
		HeaderValue: "application/json",
	}

	res := ToAPIHttpResponseHeader(header)

	assert.Equal(t, headerID.Bytes(), res.HttpResponseHeaderId)
	assert.Equal(t, respID.Bytes(), res.HttpResponseId)
	assert.Equal(t, "Content-Type", res.Key)
	assert.Equal(t, "application/json", res.Value)
}

func TestToAPIHttpResponseAssert(t *testing.T) {
	assertID := idwrap.NewNow()
	respID := idwrap.NewNow()

	assertion := mhttp.HTTPResponseAssert{
		ID:         assertID,
		ResponseID: respID,
		Value:      "status == 200",
		Success:    true,
	}

	res := ToAPIHttpResponseAssert(assertion)

	assert.Equal(t, assertID.Bytes(), res.HttpResponseAssertId)
	assert.Equal(t, respID.Bytes(), res.HttpResponseId)
	assert.Equal(t, "status == 200", res.Value)
	assert.True(t, res.Success)
}

func TestToAPIFile(t *testing.T) {
	fileID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()
	parentID := idwrap.NewNow()

	tests := []struct {
		name     string
		input    mfile.File
		expected func(*filev1.File)
	}{
		{
			name: "Root file",
			input: mfile.File{
				ID:          fileID,
				WorkspaceID: workspaceID,
				ParentID:    nil,
				Order:       1.5,
				ContentType: mfile.ContentTypeHTTP,
			},
			expected: func(res *filev1.File) {
				assert.Equal(t, fileID.Bytes(), res.FileId)
				assert.Equal(t, workspaceID.Bytes(), res.WorkspaceId)
				assert.Nil(t, res.ParentId)
				assert.Equal(t, float32(1.5), res.Order)
				assert.Equal(t, filev1.FileKind_FILE_KIND_HTTP, res.Kind)
			},
		},
		{
			name: "Nested file",
			input: mfile.File{
				ID:          fileID,
				WorkspaceID: workspaceID,
				ParentID:    &parentID,
				Order:       2.0,
				ContentType: mfile.ContentTypeFolder,
			},
			expected: func(res *filev1.File) {
				assert.Equal(t, fileID.Bytes(), res.FileId)
				assert.Equal(t, workspaceID.Bytes(), res.WorkspaceId)
				assert.Equal(t, parentID.Bytes(), res.ParentId)
				assert.Equal(t, float32(2.0), res.Order)
				assert.Equal(t, filev1.FileKind_FILE_KIND_FOLDER, res.Kind)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := ToAPIFile(tt.input)
			tt.expected(res)
		})
	}
}

func TestToAPIFileKind(t *testing.T) {
	tests := []struct {
		input    mfile.ContentType
		expected filev1.FileKind
	}{
		{mfile.ContentTypeFolder, filev1.FileKind_FILE_KIND_FOLDER},
		{mfile.ContentTypeHTTP, filev1.FileKind_FILE_KIND_HTTP},
		{mfile.ContentTypeHTTPDelta, filev1.FileKind_FILE_KIND_HTTP_DELTA},
		{mfile.ContentTypeFlow, filev1.FileKind_FILE_KIND_FLOW},
		{mfile.ContentType(-1), filev1.FileKind_FILE_KIND_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d", tt.input), func(t *testing.T) {
			assert.Equal(t, tt.expected, ToAPIFileKind(tt.input))
		})
	}
}

func TestToAPINodeKind(t *testing.T) {
	tests := []struct {
		input    mnnode.NodeKind
		expected flowv1.NodeKind
	}{
		{mnnode.NODE_KIND_NO_OP, flowv1.NodeKind_NODE_KIND_NO_OP},
		{mnnode.NODE_KIND_REQUEST, flowv1.NodeKind_NODE_KIND_HTTP},
		{mnnode.NODE_KIND_CONDITION, flowv1.NodeKind_NODE_KIND_CONDITION},
		{mnnode.NODE_KIND_FOR, flowv1.NodeKind_NODE_KIND_FOR},
		{mnnode.NODE_KIND_FOR_EACH, flowv1.NodeKind_NODE_KIND_FOR_EACH},
		{mnnode.NODE_KIND_JS, flowv1.NodeKind_NODE_KIND_JS},
		{mnnode.NodeKind(-1), flowv1.NodeKind_NODE_KIND_NO_OP},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d", tt.input), func(t *testing.T) {
			assert.Equal(t, tt.expected, ToAPINodeKind(tt.input))
		})
	}
}

func TestToAPINodeNoOpKind(t *testing.T) {
	tests := []struct {
		input    mnnoop.NoopTypes
		expected flowv1.NodeNoOpKind
	}{
		{mnnoop.NODE_NO_OP_KIND_START, flowv1.NodeNoOpKind_NODE_NO_OP_KIND_START},
		{mnnoop.NODE_NO_OP_KIND_CREATE, flowv1.NodeNoOpKind_NODE_NO_OP_KIND_CREATE},
		{mnnoop.NODE_NO_OP_KIND_THEN, flowv1.NodeNoOpKind_NODE_NO_OP_KIND_THEN},
		{mnnoop.NODE_NO_OP_KIND_ELSE, flowv1.NodeNoOpKind_NODE_NO_OP_KIND_ELSE},
		{mnnoop.NODE_NO_OP_KIND_LOOP, flowv1.NodeNoOpKind_NODE_NO_OP_KIND_LOOP},
		{mnnoop.NoopTypes(-1), flowv1.NodeNoOpKind_NODE_NO_OP_KIND_START},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d", tt.input), func(t *testing.T) {
			assert.Equal(t, tt.expected, ToAPINodeNoOpKind(tt.input))
		})
	}
}

func TestToAPIErrorHandling(t *testing.T) {
	tests := []struct {
		input    mnfor.ErrorHandling
		expected flowv1.ErrorHandling
	}{
		{mnfor.ErrorHandling_ERROR_HANDLING_IGNORE, flowv1.ErrorHandling_ERROR_HANDLING_IGNORE},
		{mnfor.ErrorHandling_ERROR_HANDLING_BREAK, flowv1.ErrorHandling_ERROR_HANDLING_BREAK},
		{mnfor.ErrorHandling(-1), flowv1.ErrorHandling_ERROR_HANDLING_IGNORE},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d", tt.input), func(t *testing.T) {
			assert.Equal(t, tt.expected, ToAPIErrorHandling(tt.input))
		})
	}
}
