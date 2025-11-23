package converter

import (
	"fmt"
	"time"
	"unicode/utf8"

	"google.golang.org/protobuf/types/known/timestamppb"

	dbmodels "the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mhttpassert"
	"the-dev-tools/server/pkg/model/mhttpbodyform"
	"the-dev-tools/server/pkg/model/mhttpbodyurlencoded"
	"the-dev-tools/server/pkg/model/mhttpheader"
	"the-dev-tools/server/pkg/model/mhttpsearchparam"

	filev1 "the-dev-tools/spec/dist/buf/go/api/file_system/v1"
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

// ToAPIHttpHeader converts model HttpHeader to API HttpHeader
func ToAPIHttpHeader(header mhttpheader.HttpHeader) *httpv1.HttpHeader {
	return &httpv1.HttpHeader{
		HttpHeaderId: header.ID.Bytes(),
		HttpId:       header.HttpID.Bytes(),
		Key:          header.Key,
		Value:        header.Value,
		Enabled:      header.Enabled,
		Description:  header.Description,
		Order:        float32(header.Order),
	}
}

// ToAPIHttpHeaderFromMHttp converts mhttp.HTTPHeader to API HttpHeader
func ToAPIHttpHeaderFromMHttp(header mhttp.HTTPHeader) *httpv1.HttpHeader {
	return &httpv1.HttpHeader{
		HttpHeaderId: header.ID.Bytes(),
		HttpId:       header.HttpID.Bytes(),
		Key:          header.HeaderKey,
		Value:        header.HeaderValue,
		Enabled:      header.Enabled,
		Description:  header.Description,
		Order:        0, // mhttp.HTTPHeader doesn't seem to have Order
	}
}

// ToAPIHttpSearchParam converts model HttpSearchParam to API HttpSearchParam
func ToAPIHttpSearchParam(param mhttpsearchparam.HttpSearchParam) *httpv1.HttpSearchParam {
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
		Key:               param.ParamKey,
		Value:             param.ParamValue,
		Enabled:           param.Enabled,
		Description:       param.Description,
		Order:             0,
	}
}

// ToAPIHttpBodyFormData converts model HttpBodyForm to API HttpBodyFormData
func ToAPIHttpBodyFormData(form mhttpbodyform.HttpBodyForm) *httpv1.HttpBodyFormData {
	return &httpv1.HttpBodyFormData{
		HttpBodyFormDataId: form.ID.Bytes(),
		HttpId:             form.HttpID.Bytes(),
		Key:                form.Key,
		Value:              form.Value,
		Enabled:            form.Enabled,
		Description:        form.Description,
	}
}

// ToAPIHttpBodyFormDataFromMHttp converts mhttp.HTTPBodyForm to API HttpBodyFormData
func ToAPIHttpBodyFormDataFromMHttp(form mhttp.HTTPBodyForm) *httpv1.HttpBodyFormData {
	return &httpv1.HttpBodyFormData{
		HttpBodyFormDataId: form.ID.Bytes(),
		HttpId:             form.HttpID.Bytes(),
		Key:                form.FormKey,
		Value:              form.FormValue,
		Enabled:            form.Enabled,
		Description:        form.Description,
	}
}

// ToAPIHttpBodyUrlEncoded converts model HttpBodyUrlEncoded to API HttpBodyUrlEncoded
func ToAPIHttpBodyUrlEncoded(urlEncoded mhttpbodyurlencoded.HttpBodyUrlEncoded) *httpv1.HttpBodyUrlEncoded {
	return &httpv1.HttpBodyUrlEncoded{
		HttpBodyUrlEncodedId: urlEncoded.ID.Bytes(),
		HttpId:               urlEncoded.HttpID.Bytes(),
		Key:                  urlEncoded.Key,
		Value:                urlEncoded.Value,
		Enabled:              urlEncoded.Enabled,
		Description:          urlEncoded.Description,
	}
}

// ToAPIHttpBodyUrlEncodedFromMHttp converts mhttp.HTTPBodyUrlencoded to API HttpBodyUrlEncoded
func ToAPIHttpBodyUrlEncodedFromMHttp(encoded mhttp.HTTPBodyUrlencoded) *httpv1.HttpBodyUrlEncoded {
	return &httpv1.HttpBodyUrlEncoded{
		HttpBodyUrlEncodedId: encoded.ID.Bytes(),
		HttpId:               encoded.HttpID.Bytes(),
		Key:                  encoded.UrlencodedKey,
		Value:                encoded.UrlencodedValue,
		Enabled:              encoded.Enabled,
		Description:          encoded.Description,
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
	return &httpv1.HttpBodyRaw{
		HttpId: raw.HttpID.Bytes(),
		Data:   string(raw.RawData),
	}
}

// ToAPIHttpAssert converts model HttpAssert to API HttpAssert
func ToAPIHttpAssert(assert mhttpassert.HttpAssert) *httpv1.HttpAssert {
	return &httpv1.HttpAssert{
		HttpAssertId: assert.ID.Bytes(),
		HttpId:       assert.HttpID.Bytes(),
		Value:        assert.Value,
	}
}

// ToAPIHttpVersion converts model HttpVersion to API HttpVersion
func ToAPIHttpVersion(version dbmodels.HttpVersion) *httpv1.HttpVersion {
	return &httpv1.HttpVersion{
		HttpVersionId: version.ID.Bytes(),
		HttpId:        version.HttpID.Bytes(),
		Name:          version.VersionName,
		Description:   version.VersionDescription,
		CreatedAt:     version.CreatedAt,
	}
}

// ToAPIHttpResponse converts DB HttpResponse to API HttpResponse
func ToAPIHttpResponse(response dbmodels.HttpResponse) *httpv1.HttpResponse {
	var body string
	if utf8.Valid(response.Body) {
		body = string(response.Body)
	} else {
		body = fmt.Sprintf("[Binary data: %d bytes]", len(response.Body))
	}

	return &httpv1.HttpResponse{
		HttpResponseId: response.ID.Bytes(),
		HttpId:         response.HttpID.Bytes(),
		Status:         int32(response.Status.(int32)),
		Body:           body,
		Time:           timestamppb.New(response.Time),
		Duration:       int32(response.Duration.(int32)),
		Size:           int32(response.Size.(int32)),
	}
}

// ToAPIHttpResponseHeader converts DB HttpResponseHeader to API HttpResponseHeader
func ToAPIHttpResponseHeader(header dbmodels.HttpResponseHeader) *httpv1.HttpResponseHeader {
	return &httpv1.HttpResponseHeader{
		HttpResponseHeaderId: header.ID.Bytes(),
		HttpResponseId:       header.ResponseID.Bytes(),
		Key:                  header.Key,
		Value:                header.Value,
	}
}

// ToAPIHttpResponseAssert converts DB HttpResponseAssert to API HttpResponseAssert
func ToAPIHttpResponseAssert(assert dbmodels.HttpResponseAssert) *httpv1.HttpResponseAssert {
	return &httpv1.HttpResponseAssert{
		HttpResponseAssertId: assert.ID,
		HttpResponseId:       assert.ResponseID,
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

	if file.FolderID != nil {
		apiFile.ParentFolderId = file.FolderID.Bytes()
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
	case mfile.ContentTypeFlow:
		return filev1.FileKind_FILE_KIND_FLOW
	default:
		return filev1.FileKind_FILE_KIND_UNSPECIFIED
	}
}
