package rimportv2

import (
	"errors"

	"connectrpc.com/connect"
	"the-dev-tools/server/pkg/idwrap"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/import/v1"
)

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
func convertToImportResponse(resp *ImportResponse) (*apiv1.ImportResponse, error) {
	return &apiv1.ImportResponse{
		MissingData: apiv1.ImportMissingDataKind(resp.MissingData),
		Domains:     resp.Domains,
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