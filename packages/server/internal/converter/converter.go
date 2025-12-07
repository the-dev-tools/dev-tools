//nolint:revive // exported
package converter

import (
	"fmt"
	"time"
	"unicode/utf8"

	"google.golang.org/protobuf/types/known/timestamppb"

	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"

	filev1 "the-dev-tools/spec/dist/buf/go/api/file_system/v1"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
	httpv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
)

// ToAPIHttp converts model HTTP to API HTTP
func ToAPIHttp(http mhttp.HTTP) *httpv1.Http {
	apiHttp := &httpv1.Http{
		HttpId:   http.ID.Bytes(),
		Name:     http.Name,
		Url:      http.Url,
		Method:   ToAPIHttpMethod(http.Method),
		BodyKind: ToAPIHttpBodyKind(http.BodyKind),
	}

	if http.LastRunAt != nil {
		apiHttp.LastRunAt = timestamppb.New(time.Unix(*http.LastRunAt, 0))
	}

	return apiHttp
}

// ToAPIHttpMethod converts string method to API HttpMethod
func ToAPIHttpMethod(method string) httpv1.HttpMethod {
	switch method {
	case "GET":
		return httpv1.HttpMethod_HTTP_METHOD_GET
	case "POST":
		return httpv1.HttpMethod_HTTP_METHOD_POST
	case "PUT":
		return httpv1.HttpMethod_HTTP_METHOD_PUT
	case "PATCH":
		return httpv1.HttpMethod_HTTP_METHOD_PATCH
	case "DELETE":
		return httpv1.HttpMethod_HTTP_METHOD_DELETE
	case "HEAD":
		return httpv1.HttpMethod_HTTP_METHOD_HEAD
	case "OPTION":
		return httpv1.HttpMethod_HTTP_METHOD_OPTION
	case "CONNECT":
		return httpv1.HttpMethod_HTTP_METHOD_CONNECT
	default:
		return httpv1.HttpMethod_HTTP_METHOD_UNSPECIFIED
	}
}

// FromAPIHttpMethod converts API HttpMethod to string
func FromAPIHttpMethod(method httpv1.HttpMethod) string {
	switch method {
	case httpv1.HttpMethod_HTTP_METHOD_GET:
		return "GET"
	case httpv1.HttpMethod_HTTP_METHOD_POST:
		return "POST"
	case httpv1.HttpMethod_HTTP_METHOD_PUT:
		return "PUT"
	case httpv1.HttpMethod_HTTP_METHOD_PATCH:
		return "PATCH"
	case httpv1.HttpMethod_HTTP_METHOD_DELETE:
		return "DELETE"
	case httpv1.HttpMethod_HTTP_METHOD_HEAD:
		return "HEAD"
	case httpv1.HttpMethod_HTTP_METHOD_OPTION:
		return "OPTION"
	case httpv1.HttpMethod_HTTP_METHOD_CONNECT:
		return "CONNECT"
	default:
		return ""
	}
}

// ToAPIHttpBodyKind converts model HttpBodyKind to API HttpBodyKind
func ToAPIHttpBodyKind(kind mhttp.HttpBodyKind) httpv1.HttpBodyKind {
	switch kind {
	case mhttp.HttpBodyKindNone:
		return httpv1.HttpBodyKind_HTTP_BODY_KIND_UNSPECIFIED
	case mhttp.HttpBodyKindFormData:
		return httpv1.HttpBodyKind_HTTP_BODY_KIND_FORM_DATA
	case mhttp.HttpBodyKindUrlEncoded:
		return httpv1.HttpBodyKind_HTTP_BODY_KIND_URL_ENCODED
	case mhttp.HttpBodyKindRaw:
		return httpv1.HttpBodyKind_HTTP_BODY_KIND_RAW
	default:
		return httpv1.HttpBodyKind_HTTP_BODY_KIND_UNSPECIFIED
	}
}

// FromAPIHttpBodyKind converts API HttpBodyKind to model HttpBodyKind
func FromAPIHttpBodyKind(kind httpv1.HttpBodyKind) mhttp.HttpBodyKind {
	switch kind {
	case httpv1.HttpBodyKind_HTTP_BODY_KIND_UNSPECIFIED:
		return mhttp.HttpBodyKindNone
	case httpv1.HttpBodyKind_HTTP_BODY_KIND_FORM_DATA:
		return mhttp.HttpBodyKindFormData
	case httpv1.HttpBodyKind_HTTP_BODY_KIND_URL_ENCODED:
		return mhttp.HttpBodyKindUrlEncoded
	case httpv1.HttpBodyKind_HTTP_BODY_KIND_RAW:
		return mhttp.HttpBodyKindRaw
	default:
		return mhttp.HttpBodyKindNone
	}
}

// ToAPIHttpHeader converts model HTTPHeader to API HttpHeader
func ToAPIHttpHeader(header mhttp.HTTPHeader) *httpv1.HttpHeader {
	return &httpv1.HttpHeader{
		HttpHeaderId: header.ID.Bytes(),
		HttpId:       header.HttpID.Bytes(),
		Key:          header.Key,
		Value:        header.Value,
		Enabled:      header.Enabled,
		Description:  header.Description,
		Order:        header.Order,
	}
}

// ToAPIHttpSearchParam converts model HttpSearchParam to API HttpSearchParam
func ToAPIHttpSearchParam(param mhttp.HTTPSearchParam) *httpv1.HttpSearchParam {
	return &httpv1.HttpSearchParam{
		HttpSearchParamId: param.ID.Bytes(),
		HttpId:            param.HttpID.Bytes(),
		Key:               param.Key,
		Value:             param.Value,
		Enabled:           param.Enabled,
		Description:       param.Description,
		Order:             float32(param.Order),
	}
}

// ToAPIHttpSearchParamFromMHttp converts mhttp.HTTPSearchParam to API HttpSearchParam
func ToAPIHttpSearchParamFromMHttp(param mhttp.HTTPSearchParam) *httpv1.HttpSearchParam {
	return &httpv1.HttpSearchParam{
		HttpSearchParamId: param.ID.Bytes(),
		HttpId:            param.HttpID.Bytes(),
		Key:               param.Key,
		Value:             param.Value,
		Enabled:           param.Enabled,
		Description:       param.Description,
		Order:             0,
	}
}

// ToAPIHttpBodyFormData converts model HttpBodyForm to API HttpBodyFormData
func ToAPIHttpBodyFormData(form mhttp.HTTPBodyForm) *httpv1.HttpBodyFormData {
	return &httpv1.HttpBodyFormData{
		HttpBodyFormDataId: form.ID.Bytes(),
		HttpId:             form.HttpID.Bytes(),
		Key:                form.Key,
		Value:              form.Value,
		Enabled:            form.Enabled,
		Description:        form.Description,
		Order:              form.Order,
	}
}

// ToAPIHttpBodyFormDataFromMHttp converts mhttp.HTTPBodyForm to API HttpBodyFormData
func ToAPIHttpBodyFormDataFromMHttp(form mhttp.HTTPBodyForm) *httpv1.HttpBodyFormData {
	return &httpv1.HttpBodyFormData{
		HttpBodyFormDataId: form.ID.Bytes(),
		HttpId:             form.HttpID.Bytes(),
		Key:                form.Key,
		Value:              form.Value,
		Enabled:            form.Enabled,
		Description:        form.Description,
		Order:              form.Order,
	}
}

// ToAPIHttpBodyUrlEncoded converts model HttpBodyUrlEncoded to API HttpBodyUrlEncoded
func ToAPIHttpBodyUrlEncoded(urlEncoded mhttp.HTTPBodyUrlencoded) *httpv1.HttpBodyUrlEncoded {
	return &httpv1.HttpBodyUrlEncoded{
		HttpBodyUrlEncodedId: urlEncoded.ID.Bytes(),
		HttpId:               urlEncoded.HttpID.Bytes(),
		Key:                  urlEncoded.Key,
		Value:                urlEncoded.Value,
		Enabled:              urlEncoded.Enabled,
		Description:          urlEncoded.Description,
		Order:                urlEncoded.Order,
	}
}

// ToAPIHttpBodyUrlEncodedFromMHttp converts mhttp.HTTPBodyUrlencoded to API HttpBodyUrlEncoded
func ToAPIHttpBodyUrlEncodedFromMHttp(encoded mhttp.HTTPBodyUrlencoded) *httpv1.HttpBodyUrlEncoded {
	return &httpv1.HttpBodyUrlEncoded{
		HttpBodyUrlEncodedId: encoded.ID.Bytes(),
		HttpId:               encoded.HttpID.Bytes(),
		Key:                  encoded.Key,
		Value:                encoded.Value,
		Enabled:              encoded.Enabled,
		Description:          encoded.Description,
		Order:                encoded.Order,
	}
}

// ToAPIHttpBodyRaw converts raw body data to API HttpBodyRaw
func ToAPIHttpBodyRaw(httpID []byte, data string) *httpv1.HttpBodyRaw {
	return &httpv1.HttpBodyRaw{
		HttpId: httpID,
		Data:   data,
	}
}

// ToAPIHttpBodyRawFromMHttp converts mhttp.HTTPBodyRaw to API HttpBodyRaw
func ToAPIHttpBodyRawFromMHttp(raw mhttp.HTTPBodyRaw) *httpv1.HttpBodyRaw {
	// For delta bodies, the override content is stored in DeltaRawData, not RawData
	var data string
	if raw.IsDelta && len(raw.DeltaRawData) > 0 {
		data = string(raw.DeltaRawData)
	} else {
		data = string(raw.RawData)
	}

	return &httpv1.HttpBodyRaw{
		HttpId: raw.HttpID.Bytes(),
		Data:   data,
	}
}

// ToAPIHttpAssert converts model HttpAssert to API HttpAssert
func ToAPIHttpAssert(assert mhttp.HTTPAssert) *httpv1.HttpAssert {
	return &httpv1.HttpAssert{
		HttpAssertId: assert.ID.Bytes(),
		HttpId:       assert.HttpID.Bytes(),
		Value:        assert.Value,
	}
}

// ToAPIHttpVersion converts model HttpVersion to API HttpVersion
func ToAPIHttpVersion(version mhttp.HttpVersion) *httpv1.HttpVersion {
	return &httpv1.HttpVersion{
		HttpVersionId: version.ID.Bytes(),
		HttpId:        version.HttpID.Bytes(),
		Name:          version.VersionName,
		Description:   version.VersionDescription,
		CreatedAt:     version.CreatedAt,
	}
}

// ToAPIHttpResponse converts DB HttpResponse to API HttpResponse
func ToAPIHttpResponse(response mhttp.HTTPResponse) *httpv1.HttpResponse {
	var body string
	if utf8.Valid(response.Body) {
		body = string(response.Body)
	} else {
		body = fmt.Sprintf("[Binary data: %d bytes]", len(response.Body))
	}

	return &httpv1.HttpResponse{
		HttpResponseId: response.ID.Bytes(),
		HttpId:         response.HttpID.Bytes(),
		Status:         response.Status,
		Body:           body,
		Time:           timestamppb.New(time.Unix(response.Time, 0)),
		Duration:       response.Duration,
		Size:           response.Size,
	}
}

// ToAPIHttpResponseHeader converts DB HttpResponseHeader to API HttpResponseHeader
func ToAPIHttpResponseHeader(header mhttp.HTTPResponseHeader) *httpv1.HttpResponseHeader {
	return &httpv1.HttpResponseHeader{
		HttpResponseHeaderId: header.ID.Bytes(),
		HttpResponseId:       header.ResponseID.Bytes(),
		Key:                  header.HeaderKey,
		Value:                header.HeaderValue,
	}
}

// ToAPIHttpResponseAssert converts DB HttpResponseAssert to API HttpResponseAssert
func ToAPIHttpResponseAssert(assert mhttp.HTTPResponseAssert) *httpv1.HttpResponseAssert {
	return &httpv1.HttpResponseAssert{
		HttpResponseAssertId: assert.ID.Bytes(),
		HttpResponseId:       assert.ResponseID.Bytes(),
		Value:                assert.Value,
		Success:              assert.Success,
	}
}

// ToAPIFile converts a model File to an API File
func ToAPIFile(file mfile.File) *filev1.File {
	apiFile := &filev1.File{
		FileId:      file.ID.Bytes(),
		WorkspaceId: file.WorkspaceID.Bytes(),
		Order:       float32(file.Order),
		Kind:        ToAPIFileKind(file.ContentType),
	}

	if file.ParentID != nil {
		apiFile.ParentId = file.ParentID.Bytes()
	}

	return apiFile
}

// ToAPIFileKind converts a model ContentType to an API FileKind
func ToAPIFileKind(kind mfile.ContentType) filev1.FileKind {
	switch kind {
	case mfile.ContentTypeFolder:
		return filev1.FileKind_FILE_KIND_FOLDER
	case mfile.ContentTypeHTTP:
		return filev1.FileKind_FILE_KIND_HTTP
	case mfile.ContentTypeHTTPDelta:
		return filev1.FileKind_FILE_KIND_HTTP_DELTA
	case mfile.ContentTypeFlow:
		return filev1.FileKind_FILE_KIND_FLOW
	default:
		return filev1.FileKind_FILE_KIND_UNSPECIFIED
	}
}

// ToAPINodeKind converts model NodeKind to API NodeKind
func ToAPINodeKind(kind mnnode.NodeKind) flowv1.NodeKind {
	switch kind {
	case mnnode.NODE_KIND_NO_OP:
		return flowv1.NodeKind_NODE_KIND_NO_OP
	case mnnode.NODE_KIND_REQUEST:
		return flowv1.NodeKind_NODE_KIND_HTTP
	case mnnode.NODE_KIND_CONDITION:
		return flowv1.NodeKind_NODE_KIND_CONDITION
	case mnnode.NODE_KIND_FOR:
		return flowv1.NodeKind_NODE_KIND_FOR
	case mnnode.NODE_KIND_FOR_EACH:
		return flowv1.NodeKind_NODE_KIND_FOR_EACH
	case mnnode.NODE_KIND_JS:
		return flowv1.NodeKind_NODE_KIND_JS
	default:
		return flowv1.NodeKind_NODE_KIND_NO_OP
	}
}

// ToAPINodeNoOpKind converts model NoopTypes to API NodeNoOpKind
func ToAPINodeNoOpKind(kind mnnoop.NoopTypes) flowv1.NodeNoOpKind {
	switch kind {
	case mnnoop.NODE_NO_OP_KIND_START:
		return flowv1.NodeNoOpKind_NODE_NO_OP_KIND_START
	case mnnoop.NODE_NO_OP_KIND_CREATE:
		return flowv1.NodeNoOpKind_NODE_NO_OP_KIND_CREATE
	case mnnoop.NODE_NO_OP_KIND_THEN:
		return flowv1.NodeNoOpKind_NODE_NO_OP_KIND_THEN
	case mnnoop.NODE_NO_OP_KIND_ELSE:
		return flowv1.NodeNoOpKind_NODE_NO_OP_KIND_ELSE
	case mnnoop.NODE_NO_OP_KIND_LOOP:
		return flowv1.NodeNoOpKind_NODE_NO_OP_KIND_LOOP
	default:
		return flowv1.NodeNoOpKind_NODE_NO_OP_KIND_START
	}
}

// ToAPIErrorHandling converts model ErrorHandling to API ErrorHandling
func ToAPIErrorHandling(eh mnfor.ErrorHandling) flowv1.ErrorHandling {
	switch eh {
	case mnfor.ErrorHandling_ERROR_HANDLING_IGNORE:
		return flowv1.ErrorHandling_ERROR_HANDLING_IGNORE
	case mnfor.ErrorHandling_ERROR_HANDLING_BREAK:
		return flowv1.ErrorHandling_ERROR_HANDLING_BREAK
	default:
		return flowv1.ErrorHandling_ERROR_HANDLING_IGNORE
	}
}
