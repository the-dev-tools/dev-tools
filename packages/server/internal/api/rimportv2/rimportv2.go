package rimportv2

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/rhttp"
	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/service/sfile"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/shttpbodyform"
	"the-dev-tools/server/pkg/service/shttpbodyurlencoded"
	"the-dev-tools/server/pkg/service/shttpheader"
	"the-dev-tools/server/pkg/service/shttpsearchparam"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	httpv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/import/v1"
	"the-dev-tools/spec/dist/buf/go/api/import/v1/importv1connect"
)

// ImportV2RPC implements the Connect RPC interface for HAR import v2
type ImportV2RPC struct {
	db      *sql.DB
	service *Service
	logger  *slog.Logger
	ws      sworkspace.WorkspaceService
	us      suser.UserService

	// Streamers for real-time updates
	stream                   eventstream.SyncStreamer[rhttp.HttpTopic, rhttp.HttpEvent]
	httpHeaderStream         eventstream.SyncStreamer[rhttp.HttpHeaderTopic, rhttp.HttpHeaderEvent]
	httpSearchParamStream    eventstream.SyncStreamer[rhttp.HttpSearchParamTopic, rhttp.HttpSearchParamEvent]
	httpBodyFormStream       eventstream.SyncStreamer[rhttp.HttpBodyFormTopic, rhttp.HttpBodyFormEvent]
	httpBodyUrlEncodedStream eventstream.SyncStreamer[rhttp.HttpBodyUrlEncodedTopic, rhttp.HttpBodyUrlEncodedEvent]
	httpBodyRawStream        eventstream.SyncStreamer[rhttp.HttpBodyRawTopic, rhttp.HttpBodyRawEvent]
}

// NewImportV2RPC creates a new ImportV2RPC handler with all required dependencies
func NewImportV2RPC(
	db *sql.DB,
	ws sworkspace.WorkspaceService,
	us suser.UserService,
	httpService *shttp.HTTPService,
	flowService *sflow.FlowService,
	fileService *sfile.FileService,
	// Child entity services
	httpHeaderService shttpheader.HttpHeaderService,
	httpSearchParamService shttpsearchparam.HttpSearchParamService,
	httpBodyFormService shttpbodyform.HttpBodyFormService,
	httpBodyUrlEncodedService shttpbodyurlencoded.HttpBodyUrlEncodedService,
	bodyService *shttp.HttpBodyRawService,
	logger *slog.Logger,
	// Streamers
	stream eventstream.SyncStreamer[rhttp.HttpTopic, rhttp.HttpEvent],
	httpHeaderStream eventstream.SyncStreamer[rhttp.HttpHeaderTopic, rhttp.HttpHeaderEvent],
	httpSearchParamStream eventstream.SyncStreamer[rhttp.HttpSearchParamTopic, rhttp.HttpSearchParamEvent],
	httpBodyFormStream eventstream.SyncStreamer[rhttp.HttpBodyFormTopic, rhttp.HttpBodyFormEvent],
	httpBodyUrlEncodedStream eventstream.SyncStreamer[rhttp.HttpBodyUrlEncodedTopic, rhttp.HttpBodyUrlEncodedEvent],
	httpBodyRawStream eventstream.SyncStreamer[rhttp.HttpBodyRawTopic, rhttp.HttpBodyRawEvent],
) *ImportV2RPC {
	// Create the importer with modern service dependencies
	importer := NewImporter(httpService, flowService, fileService,
		httpHeaderService, httpSearchParamService, httpBodyFormService, httpBodyUrlEncodedService, bodyService)

	// Create the validator for input validation
	validator := NewValidator(&us)

	// Create the main service with functional options
	service := NewService(importer, validator, WithLogger(logger))

	// Create and return the RPC handler
	return &ImportV2RPC{
		db:                       db,
		service:                  service,
		logger:                   logger,
		ws:                       ws,
		us:                       us,
		stream:                   stream,
		httpHeaderStream:         httpHeaderStream,
		httpSearchParamStream:    httpSearchParamStream,
		httpBodyFormStream:       httpBodyFormStream,
		httpBodyUrlEncodedStream: httpBodyUrlEncodedStream,
		httpBodyRawStream:        httpBodyRawStream,
	}
}

// CreateImportV2Service creates the service registration for rimportv2
// This follows the exact same pattern as rimport.CreateService function
func CreateImportV2Service(srv ImportV2RPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := importv1connect.NewImportServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

// Import implements the Import RPC method from the TypeSpec interface
// This method delegates to the internal service after proper validation and setup
func (h *ImportV2RPC) Import(ctx context.Context, req *connect.Request[apiv1.ImportRequest]) (*connect.Response[apiv1.ImportResponse], error) {
	startTime := time.Now()

	h.logger.Info("Received ImportV2 RPC request",
		"workspace_id", req.Msg.WorkspaceId,
		"name", req.Msg.Name,
		"data_size", len(req.Msg.Data))

	// Convert protobuf request to internal request model
	importReq, err := convertToImportRequest(req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Call the service to process the import
	results, err := h.service.Import(ctx, importReq)
	if err != nil {
		return handleServiceError(err)
	}

	// Publish events for real-time sync
	h.publishEvents(ctx, results)

	// Convert internal response to protobuf response
	protoResp, err := convertToImportResponse(results)
	if err != nil {
		h.logger.Error("Response conversion failed - unexpected internal error",
			"workspace_id", req.Msg.WorkspaceId,
			"missing_data", results.MissingData,
			"domains_count", len(results.Domains),
			"error", err)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	rpcDuration := time.Since(startTime)
	h.logger.Info("ImportV2 RPC completed successfully",
		"workspace_id", req.Msg.WorkspaceId,
		"missing_data", protoResp.MissingData,
		"domains", len(protoResp.Domains),
		"duration_ms", rpcDuration.Milliseconds())

	return connect.NewResponse(protoResp), nil
}

// Private conversion functions moved from conversion.go

// convertToImportRequest converts protobuf request to internal request model.
// It parses workspace ID, converts domain data structures, and validates basic constraints.
func convertToImportRequest(msg *apiv1.ImportRequest) (*ImportRequest, error) {
	// Parse workspace ID
	workspaceID, err := idwrap.NewFromBytes(msg.WorkspaceId)
	if err != nil {
		return nil, NewValidationErrorWithCause("workspaceId", err)
	}

	// Convert domain data
	domainData := make([]ImportDomainData, len(msg.DomainData))
	for i, dd := range msg.DomainData {
		domainData[i] = ImportDomainData{
			Enabled:  dd.Enabled,
			Domain:   dd.Domain,
			Variable: dd.Variable,
		}
	}

	return &ImportRequest{
		WorkspaceID: workspaceID,
		Name:        msg.Name,
		Data:        msg.Data,
		TextData:    msg.TextData,
		DomainData:  domainData,
	}, nil
}

// convertToImportResponse converts internal response to protobuf response model.
// It maps missing data kinds and domain lists to their protobuf equivalents.
func convertToImportResponse(results *ImportResults) (*apiv1.ImportResponse, error) {
	return &apiv1.ImportResponse{
		MissingData: apiv1.ImportMissingDataKind(results.MissingData),
		Domains:     results.Domains,
	}, nil
}

// handleServiceError converts service errors to appropriate Connect errors.
// It maps validation, workspace, permission, storage, and format errors
// to their corresponding Connect status codes with proper error wrapping.
func handleServiceError(err error) (*connect.Response[apiv1.ImportResponse], error) {
	if err == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("nil error provided to handleServiceError"))
	}

	switch {
	case IsValidationError(err):
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	case err == ErrWorkspaceNotFound:
		return nil, connect.NewError(connect.CodeNotFound, err)
	case err == ErrPermissionDenied:
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	case err == ErrStorageFailed:
		return nil, connect.NewError(connect.CodeInternal, err)
	case err == ErrInvalidHARFormat:
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	default:
		return nil, connect.NewError(connect.CodeInternal, err)
	}
}

// publishEvents publishes real-time sync events for imported entities
func (h *ImportV2RPC) publishEvents(ctx context.Context, results *ImportResults) {
	// Publish HTTP events
	for _, httpReq := range results.HTTPReqs {
		h.stream.Publish(rhttp.HttpTopic{WorkspaceID: httpReq.WorkspaceID}, rhttp.HttpEvent{
			Type: "insert",
			Http: toAPIHttp(httpReq),
		})
	}

	// Publish Header events
	for _, header := range results.HTTPHeaders {
		h.httpHeaderStream.Publish(rhttp.HttpHeaderTopic{WorkspaceID: results.WorkspaceID}, rhttp.HttpHeaderEvent{
			Type:       "insert",
			HttpHeader: toAPIHttpHeader(header),
		})
	}

	// Publish SearchParam events
	for _, param := range results.HTTPSearchParams {
		h.httpSearchParamStream.Publish(rhttp.HttpSearchParamTopic{WorkspaceID: results.WorkspaceID}, rhttp.HttpSearchParamEvent{
			Type:            "insert",
			HttpSearchParam: toAPIHttpSearchParam(param),
		})
	}

	// Publish BodyForm events
	for _, form := range results.HTTPBodyForms {
		h.httpBodyFormStream.Publish(rhttp.HttpBodyFormTopic{WorkspaceID: results.WorkspaceID}, rhttp.HttpBodyFormEvent{
			Type:         "insert",
			HttpBodyForm: toAPIHttpBodyFormData(form),
		})
	}

	// Publish BodyUrlEncoded events
	for _, encoded := range results.HTTPBodyUrlEncoded {
		h.httpBodyUrlEncodedStream.Publish(rhttp.HttpBodyUrlEncodedTopic{WorkspaceID: results.WorkspaceID}, rhttp.HttpBodyUrlEncodedEvent{
			Type:               "insert",
			HttpBodyUrlEncoded: toAPIHttpBodyUrlEncoded(encoded),
		})
	}

	// Publish BodyRaw events
	for _, raw := range results.HTTPBodyRaws {
		h.httpBodyRawStream.Publish(rhttp.HttpBodyRawTopic{WorkspaceID: results.WorkspaceID}, rhttp.HttpBodyRawEvent{
			Type:        "insert",
			HttpBodyRaw: toAPIHttpBodyRaw(raw),
		})
	}
}

// Helper functions for API conversion

func toAPIHttp(http *mhttp.HTTP) *httpv1.Http {
	apiHttp := &httpv1.Http{
		HttpId:   http.ID.Bytes(),
		Name:     http.Name,
		Url:      http.Url,
		Method:   toAPIHttpMethod(http.Method),
		BodyKind: toAPIHttpBodyKind(http.BodyKind),
	}
	if http.LastRunAt != nil {
		apiHttp.LastRunAt = timestamppb.New(time.Unix(*http.LastRunAt, 0))
	}
	return apiHttp
}

func toAPIHttpMethod(method string) httpv1.HttpMethod {
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

func toAPIHttpBodyKind(kind mhttp.HttpBodyKind) httpv1.HttpBodyKind {
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

func toAPIHttpHeader(header *mhttp.HTTPHeader) *httpv1.HttpHeader {
	return &httpv1.HttpHeader{
		HttpHeaderId: header.ID.Bytes(),
		HttpId:       header.HttpID.Bytes(),
		Key:          header.HeaderKey,
		Value:        header.HeaderValue,
		Enabled:      header.Enabled,
		Description:  header.Description,
		Order:        0,
	}
}

func toAPIHttpSearchParam(param *mhttp.HTTPSearchParam) *httpv1.HttpSearchParam {
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

func toAPIHttpBodyFormData(form *mhttp.HTTPBodyForm) *httpv1.HttpBodyFormData {
	return &httpv1.HttpBodyFormData{
		HttpBodyFormDataId: form.ID.Bytes(),
		HttpId:             form.HttpID.Bytes(),
		Key:                form.FormKey,
		Value:              form.FormValue,
		Enabled:            form.Enabled,
		Description:        form.Description,
	}
}

func toAPIHttpBodyUrlEncoded(encoded *mhttp.HTTPBodyUrlencoded) *httpv1.HttpBodyUrlEncoded {
	return &httpv1.HttpBodyUrlEncoded{
		HttpBodyUrlEncodedId: encoded.ID.Bytes(),
		HttpId:               encoded.HttpID.Bytes(),
		Key:                  encoded.UrlencodedKey,
		Value:                encoded.UrlencodedValue,
		Enabled:              encoded.Enabled,
		Description:          encoded.Description,
	}
}

func toAPIHttpBodyRaw(raw *mhttp.HTTPBodyRaw) *httpv1.HttpBodyRaw {
	return &httpv1.HttpBodyRaw{
		Data: string(raw.RawData),
	}
}

