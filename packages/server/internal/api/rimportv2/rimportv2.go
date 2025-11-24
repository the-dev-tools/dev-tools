package rimportv2

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/rflowv2"
	"the-dev-tools/server/internal/api/rhttp"
	"the-dev-tools/server/internal/api/rfile"
	"the-dev-tools/server/internal/converter"
	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/sfile"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/shttpbodyform"
	"the-dev-tools/server/pkg/service/shttpbodyurlencoded"
	"the-dev-tools/server/pkg/service/shttpheader"
	"the-dev-tools/server/pkg/service/shttpsearchparam"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
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
	flowStream               eventstream.SyncStreamer[rflowv2.FlowTopic, rflowv2.FlowEvent]
	nodeStream               eventstream.SyncStreamer[rflowv2.NodeTopic, rflowv2.NodeEvent]
	edgeStream               eventstream.SyncStreamer[rflowv2.EdgeTopic, rflowv2.EdgeEvent]
	stream                   eventstream.SyncStreamer[rhttp.HttpTopic, rhttp.HttpEvent]
	httpHeaderStream         eventstream.SyncStreamer[rhttp.HttpHeaderTopic, rhttp.HttpHeaderEvent]
	httpSearchParamStream    eventstream.SyncStreamer[rhttp.HttpSearchParamTopic, rhttp.HttpSearchParamEvent]
	httpBodyFormStream       eventstream.SyncStreamer[rhttp.HttpBodyFormTopic, rhttp.HttpBodyFormEvent]
	httpBodyUrlEncodedStream eventstream.SyncStreamer[rhttp.HttpBodyUrlEncodedTopic, rhttp.HttpBodyUrlEncodedEvent]
	httpBodyRawStream        eventstream.SyncStreamer[rhttp.HttpBodyRawTopic, rhttp.HttpBodyRawEvent]
	fileStream               eventstream.SyncStreamer[rfile.FileTopic, rfile.FileEvent]
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
	nodeService *snode.NodeService,
	nodeRequestService *snoderequest.NodeRequestService,
	edgeService *sedge.EdgeService,
	logger *slog.Logger,
	// Streamers
	flowStream eventstream.SyncStreamer[rflowv2.FlowTopic, rflowv2.FlowEvent],
	nodeStream eventstream.SyncStreamer[rflowv2.NodeTopic, rflowv2.NodeEvent],
	edgeStream eventstream.SyncStreamer[rflowv2.EdgeTopic, rflowv2.EdgeEvent],
	stream eventstream.SyncStreamer[rhttp.HttpTopic, rhttp.HttpEvent],
	httpHeaderStream eventstream.SyncStreamer[rhttp.HttpHeaderTopic, rhttp.HttpHeaderEvent],
	httpSearchParamStream eventstream.SyncStreamer[rhttp.HttpSearchParamTopic, rhttp.HttpSearchParamEvent],
	httpBodyFormStream eventstream.SyncStreamer[rhttp.HttpBodyFormTopic, rhttp.HttpBodyFormEvent],
	httpBodyUrlEncodedStream eventstream.SyncStreamer[rhttp.HttpBodyUrlEncodedTopic, rhttp.HttpBodyUrlEncodedEvent],
	httpBodyRawStream eventstream.SyncStreamer[rhttp.HttpBodyRawTopic, rhttp.HttpBodyRawEvent],
	fileStream eventstream.SyncStreamer[rfile.FileTopic, rfile.FileEvent],
) *ImportV2RPC {
	// Create the importer with modern service dependencies
	importer := NewImporter(db, httpService, flowService, fileService,
		httpHeaderService, httpSearchParamService, httpBodyFormService, httpBodyUrlEncodedService, bodyService,
		nodeService, nodeRequestService, edgeService)

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
		flowStream:               flowStream,
		nodeStream:               nodeStream,
		edgeStream:               edgeStream,
		stream:                   stream,
		httpHeaderStream:         httpHeaderStream,
		httpSearchParamStream:    httpSearchParamStream,
		httpBodyFormStream:       httpBodyFormStream,
		httpBodyUrlEncodedStream: httpBodyUrlEncodedStream,
		httpBodyRawStream:        httpBodyRawStream,
		fileStream:               fileStream,
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
	results, err := h.service.ImportUnified(ctx, importReq)
	if err != nil {
		return handleServiceError(err)
	}

	// Publish events for real-time sync ONLY if storage occurred (no missing data)
	if results.MissingData == ImportMissingDataKind_UNSPECIFIED {
		h.publishEvents(ctx, results)
	}

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
	resp := &apiv1.ImportResponse{
		MissingData: apiv1.ImportMissingDataKind(results.MissingData),
		Domains:     results.Domains,
	}

	if results.Flow != nil {
		resp.FlowId = results.Flow.ID.Bytes()
	}

	return resp, nil
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
	// Publish Flow event FIRST if present
	if results.Flow != nil {
		flowPB := &flowv1.Flow{
			FlowId: results.Flow.ID.Bytes(),
			Name:   results.Flow.Name,
		}
		if results.Flow.Duration != 0 {
			d := results.Flow.Duration
			flowPB.Duration = &d
		}

		h.flowStream.Publish(rflowv2.FlowTopic{WorkspaceID: results.Flow.WorkspaceID}, rflowv2.FlowEvent{
			Type: "insert",
			Flow: flowPB,
		})

		// Publish Nodes events
		for _, node := range results.Nodes {
			nodePB := &flowv1.Node{
				NodeId: node.ID.Bytes(),
				FlowId: node.FlowID.Bytes(),
				Name:   node.Name,
				Kind:   flowv1.NodeKind(node.NodeKind),
				Position: &flowv1.Position{
					X: float32(node.PositionX),
					Y: float32(node.PositionY),
				},
			}
			h.nodeStream.Publish(rflowv2.NodeTopic{FlowID: node.FlowID}, rflowv2.NodeEvent{
				Type: "insert",
				Node: nodePB,
			})
		}

		// Publish Request Nodes (as node events? No, wait, request nodes are just configuration for nodes)
		// Request Node configuration is usually fetched separately or part of node details depending on API.
		// Looking at rflowv2.go, NodeHttpCollection returns the configuration.
		// There isn't a separate stream for NodeRequest configuration in the list I saw in rflowv2.go (NodeStream carries NodeEvent).
		// However, NodeEvent in rflowv2.go only contains *flowv1.Node.
		// There doesn't seem to be a stream for NodeRequest updates in rflowv2.go.
		// Assuming we only need to stream the base nodes and edges for the graph to appear.

		// Publish Edges events
		for _, edge := range results.Edges {
			edgePB := &flowv1.Edge{
				EdgeId:       edge.ID.Bytes(),
				FlowId:       edge.FlowID.Bytes(),
				SourceId:     edge.SourceID.Bytes(),
				TargetId:     edge.TargetID.Bytes(),
				SourceHandle: flowv1.HandleKind(edge.SourceHandler),
				Kind:         flowv1.EdgeKind(edge.Kind),
			}
			h.edgeStream.Publish(rflowv2.EdgeTopic{FlowID: edge.FlowID}, rflowv2.EdgeEvent{
				Type: "insert",
				Edge: edgePB,
			})
		}
	}

	// Publish HTTP events
	for _, httpReq := range results.HTTPReqs {
		h.stream.Publish(rhttp.HttpTopic{WorkspaceID: httpReq.WorkspaceID}, rhttp.HttpEvent{
			Type: "insert",
			Http: converter.ToAPIHttp(*httpReq),
		})
	}

	// Publish File events
	for _, file := range results.Files {
		// No longer skipping Flow files since we publish Flow event first now
		h.fileStream.Publish(rfile.FileTopic{WorkspaceID: file.WorkspaceID}, rfile.FileEvent{
			Type: "create",
			File: converter.ToAPIFile(*file),
			Name: file.Name,
		})
	}

	// Publish Header events
	for _, header := range results.HTTPHeaders {
		h.httpHeaderStream.Publish(rhttp.HttpHeaderTopic{WorkspaceID: results.WorkspaceID}, rhttp.HttpHeaderEvent{
			Type:       "insert",
			HttpHeader: converter.ToAPIHttpHeaderFromMHttp(*header),
		})
	}

	// Publish SearchParam events
	for _, param := range results.HTTPSearchParams {
		h.httpSearchParamStream.Publish(rhttp.HttpSearchParamTopic{WorkspaceID: results.WorkspaceID}, rhttp.HttpSearchParamEvent{
			Type:            "insert",
			HttpSearchParam: converter.ToAPIHttpSearchParamFromMHttp(*param),
		})
	}

	// Publish BodyForm events
	for _, form := range results.HTTPBodyForms {
		h.httpBodyFormStream.Publish(rhttp.HttpBodyFormTopic{WorkspaceID: results.WorkspaceID}, rhttp.HttpBodyFormEvent{
			Type:         "insert",
			HttpBodyForm: converter.ToAPIHttpBodyFormDataFromMHttp(*form),
		})
	}

	// Publish BodyUrlEncoded events
	for _, encoded := range results.HTTPBodyUrlEncoded {
		h.httpBodyUrlEncodedStream.Publish(rhttp.HttpBodyUrlEncodedTopic{WorkspaceID: results.WorkspaceID}, rhttp.HttpBodyUrlEncodedEvent{
			Type:               "insert",
			HttpBodyUrlEncoded: converter.ToAPIHttpBodyUrlEncodedFromMHttp(*encoded),
		})
	}

	// Publish BodyRaw events
	for _, raw := range results.HTTPBodyRaws {
		h.httpBodyRawStream.Publish(rhttp.HttpBodyRawTopic{WorkspaceID: results.WorkspaceID}, rhttp.HttpBodyRawEvent{
			Type:        "insert",
			HttpBodyRaw: converter.ToAPIHttpBodyRawFromMHttp(*raw),
		})
	}
}
