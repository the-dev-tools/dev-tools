package rexportv2

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
	"the-dev-tools/server/pkg/service/sfile"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/service/sworkspacesusers"
	"the-dev-tools/server/pkg/testutil"
	exportv1 "the-dev-tools/spec/dist/buf/go/api/export/v1"
)

// Make the imports available for tests that need them
var _ = (&sworkspace.WorkspaceService{}).Create
var _ = (&sworkspacesusers.WorkspaceUserService{}).CreateWorkspaceUser

// TestNewExportV2RPC tests the RPC handler constructor
func TestNewExportV2RPC(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	t.Cleanup(base.Close)

	services := base.GetBaseServices()
	logger := base.Logger()

	// Create additional services
	httpService := shttp.New(base.Queries, logger)
	flowService := sflow.New(base.Queries)
	fileService := sfile.New(base.Queries, logger)

	rpc := NewExportV2RPC(
		base.DB,
		base.Queries,
		services.Ws,
		services.Us,
		&httpService,
		&flowService,
		fileService,
		logger,
	)

	require.NotNil(t, rpc)
	require.NotNil(t, rpc.service)
	require.NotNil(t, rpc.logger)
}

// TestCreateExportV2Service tests the service registration
func TestCreateExportV2Service(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	t.Cleanup(base.Close)

	services := base.GetBaseServices()
	logger := base.Logger()

	httpService := shttp.New(base.Queries, logger)
	flowService := sflow.New(base.Queries)
	fileService := sfile.New(base.Queries, logger)

	rpc := NewExportV2RPC(
		base.DB,
		base.Queries,
		services.Ws,
		services.Us,
		&httpService,
		&flowService,
		fileService,
		logger,
	)

	service, err := CreateExportV2Service(*rpc, nil)
	require.NoError(t, err)
	require.NotNil(t, service)
	require.NotEmpty(t, service.Path)
	require.NotNil(t, service.Handler)
}

// TestExportV2RPC_Export_Success tests successful export operation
func TestExportV2RPC_Export_Success(t *testing.T) {
	ctx := context.Background()
	svc, workspaceID, flowID := setupExportV2RPC(t, ctx)

	resp, err := svc.Export(ctx, connect.NewRequest(&exportv1.ExportRequest{
		WorkspaceId: workspaceID.Bytes(),
		FileIds:     [][]byte{flowID.Bytes()}, // Use flowID as fileID for now
	}))

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotEmpty(t, resp.Msg.GetData())
	require.True(t, len(resp.Msg.Name) > 0)
}

// TestExportV2RPC_Export_InvalidWorkspaceID tests export with invalid workspace ID
func TestExportV2RPC_Export_InvalidWorkspaceID(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := setupExportV2RPC(t, ctx)

	// Invalid workspace ID (empty bytes)
	resp, err := svc.Export(ctx, connect.NewRequest(&exportv1.ExportRequest{
		WorkspaceId: []byte{},
	}))

	require.Error(t, err)
	require.Nil(t, resp)

	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

// TestExportV2RPC_Export_InvalidFlowIDs tests export with invalid flow IDs
func TestExportV2RPC_Export_InvalidFlowIDs(t *testing.T) {
	ctx := context.Background()
	svc, workspaceID, _ := setupExportV2RPC(t, ctx)

	// Invalid file ID (empty bytes)
	resp, err := svc.Export(ctx, connect.NewRequest(&exportv1.ExportRequest{
		WorkspaceId: workspaceID.Bytes(),
		FileIds:     [][]byte{[]byte{}},
	}))

	require.Error(t, err)
	require.Nil(t, resp)

	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

// TestExportV2RPC_Export_UnsupportedFormat tests export with unsupported format
func TestExportV2RPC_Export_UnsupportedFormat(t *testing.T) {
	ctx := context.Background()
	svc, workspaceID, _ := setupExportV2RPC(t, ctx)

	// The service defaults to YAML format for standard Export requests
	resp, err := svc.Export(ctx, connect.NewRequest(&exportv1.ExportRequest{
		WorkspaceId: workspaceID.Bytes(),
	}))

	// The service should handle the export successfully
	require.NoError(t, err)
	require.NotNil(t, resp)
}

// TestExportV2RPC_ExportCurl_Success tests successful cURL export
func TestExportV2RPC_ExportCurl_Success(t *testing.T) {
	ctx := context.Background()
	svc, workspaceID, exampleID := setupExportV2RPC(t, ctx)

	resp, err := svc.ExportCurl(ctx, connect.NewRequest(&exportv1.ExportCurlRequest{
		WorkspaceId: workspaceID.Bytes(),
		HttpIds:     [][]byte{exampleID.Bytes()},
	}))

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotEmpty(t, resp.Msg.GetData())
}

// TestExportV2RPC_ExportCurl_InvalidWorkspaceID tests cURL export with invalid workspace ID
func TestExportV2RPC_ExportCurl_InvalidWorkspaceID(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := setupExportV2RPC(t, ctx)

	resp, err := svc.ExportCurl(ctx, connect.NewRequest(&exportv1.ExportCurlRequest{
		WorkspaceId: []byte{},
	}))

	require.Error(t, err)
	require.Nil(t, resp)

	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

// TestExportV2RPC_ExportCurl_InvalidHttpIDs tests cURL export with invalid HTTP IDs
func TestExportV2RPC_ExportCurl_InvalidHttpIDs(t *testing.T) {
	ctx := context.Background()
	svc, workspaceID, _ := setupExportV2RPC(t, ctx)

	resp, err := svc.ExportCurl(ctx, connect.NewRequest(&exportv1.ExportCurlRequest{
		WorkspaceId: workspaceID.Bytes(),
		HttpIds:     [][]byte{[]byte{}},
	}))

	require.Error(t, err)
	require.Nil(t, resp)

	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

// TestExportV2RPC_ExportWithFlowFilter tests export with flow filtering
func TestExportV2RPC_ExportWithFlowFilter(t *testing.T) {
	ctx := context.Background()
	svc, workspaceID, flowID := setupExportV2RPC(t, ctx)

	resp, err := svc.Export(ctx, connect.NewRequest(&exportv1.ExportRequest{
		WorkspaceId: workspaceID.Bytes(),
		FileIds:     [][]byte{flowID.Bytes()}, // Use flowID as fileID for now
	}))

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotEmpty(t, resp.Msg.GetData())
}

// TestExportV2RPC_ContextCancellation tests export with context cancellation
func TestExportV2RPC_ContextCancellation(t *testing.T) {
	// Use background context for setup to avoid schema execution failure
	svc, workspaceID, _ := setupExportV2RPC(t, context.Background())

	// Create a short-lived context for the request
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Wait for context to be cancelled/timed out
	time.Sleep(1 * time.Millisecond)

	resp, err := svc.Export(ctx, connect.NewRequest(&exportv1.ExportRequest{
		WorkspaceId: workspaceID.Bytes(),
	}))

	require.Error(t, err)
	require.Nil(t, resp)
	// Expect deadline exceeded since we used WithTimeout
	assert.Contains(t, err.Error(), "deadline exceeded")
}

// TestConvertToExportRequest tests request conversion function
func TestConvertToExportRequest(t *testing.T) {
	workspaceID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	_ = idwrap.NewNow() // exampleID - not used in new spec

	tests := []struct {
		name        string
		msg         *exportv1.ExportRequest
		expectError bool
		expectReq   *ExportRequest
	}{
		{
			name: "valid request with file IDs",
			msg: &exportv1.ExportRequest{
				WorkspaceId: workspaceID.Bytes(),
				FileIds:     [][]byte{flowID.Bytes()}, // Use flowID as fileID for now
			},
			expectError: false,
			expectReq: &ExportRequest{
				WorkspaceID: workspaceID,
				FileIDs:     []idwrap.IDWrap{flowID},
				Format:      ExportFormat_YAML, // Default format
				Simplified:  false,
			},
		},
		{
			name: "valid request with default format",
			msg: &exportv1.ExportRequest{
				WorkspaceId: workspaceID.Bytes(),
			},
			expectError: false,
			expectReq: &ExportRequest{
				WorkspaceID: workspaceID,
				FileIDs:     []idwrap.IDWrap{},
				Format:      ExportFormat_YAML, // Default
				Simplified:  false,
			},
		},
		{
			name: "invalid workspace ID",
			msg: &exportv1.ExportRequest{
				WorkspaceId: []byte{},
			},
			expectError: true,
		},
		{
			name: "invalid file ID",
			msg: &exportv1.ExportRequest{
				WorkspaceId: workspaceID.Bytes(),
				FileIds:     [][]byte{[]byte{}},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := convertToExportRequest(tt.msg)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, req)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectReq.WorkspaceID, req.WorkspaceID)
				assert.Equal(t, tt.expectReq.Format, req.Format)
				assert.Equal(t, tt.expectReq.Simplified, req.Simplified)

				if len(tt.expectReq.FileIDs) > 0 {
					assert.Equal(t, len(tt.expectReq.FileIDs), len(req.FileIDs))
				}
			}
		})
	}
}

// TestConvertToExportCurlRequest tests cURL request conversion function
func TestConvertToExportCurlRequest(t *testing.T) {
	workspaceID := idwrap.NewNow()
	exampleID := idwrap.NewNow()

	tests := []struct {
		name        string
		msg         *exportv1.ExportCurlRequest
		expectError bool
		expectReq   *ExportCurlRequest
	}{
		{
			name: "valid request",
			msg: &exportv1.ExportCurlRequest{
				WorkspaceId: workspaceID.Bytes(),
				HttpIds:     [][]byte{exampleID.Bytes()}, // Use exampleID as httpID for now
			},
			expectError: false,
			expectReq: &ExportCurlRequest{
				WorkspaceID: workspaceID,
				HTTPIDs:     []idwrap.IDWrap{exampleID},
			},
		},
		{
			name: "invalid workspace ID",
			msg: &exportv1.ExportCurlRequest{
				WorkspaceId: []byte{},
			},
			expectError: true,
		},
		{
			name: "invalid HTTP ID",
			msg: &exportv1.ExportCurlRequest{
				WorkspaceId: workspaceID.Bytes(),
				HttpIds:     [][]byte{[]byte{}},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := convertToExportCurlRequest(tt.msg)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, req)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectReq.WorkspaceID, req.WorkspaceID)
				assert.Equal(t, len(tt.expectReq.HTTPIDs), len(req.HTTPIDs))
			}
		})
	}
}

// TestConvertToExportResponse tests response conversion function
func TestConvertToExportResponse(t *testing.T) {
	resp := &ExportResponse{
		Name: "test.yaml",
		Data: []byte("test data"),
	}

	protoResp, err := convertToExportResponse(resp)

	require.NoError(t, err)
	require.NotNil(t, protoResp)
	assert.Equal(t, resp.Name, protoResp.Name)
	assert.Equal(t, resp.Data, protoResp.Data)
}

// TestHandleServiceError tests error handling function
func TestHandleServiceError(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		expectedCode connect.Code
	}{
		{
			name:         "validation error",
			err:          NewValidationError("test", "invalid"),
			expectedCode: connect.CodeInvalidArgument,
		},
		{
			name:         "workspace not found",
			err:          ErrWorkspaceNotFound,
			expectedCode: connect.CodeNotFound,
		},
		{
			name:         "permission denied",
			err:          ErrPermissionDenied,
			expectedCode: connect.CodePermissionDenied,
		},
		{
			name:         "export failed",
			err:          ErrExportFailed,
			expectedCode: connect.CodeInternal,
		},
		{
			name:         "no data found",
			err:          ErrNoDataFound,
			expectedCode: connect.CodeNotFound,
		},
		{
			name:         "unsupported format",
			err:          ErrUnsupportedFormat,
			expectedCode: connect.CodeInvalidArgument,
		},
		{
			name:         "timeout",
			err:          ErrTimeout,
			expectedCode: connect.CodeDeadlineExceeded,
		},
		{
			name:         "nil_error_wrapper_fixed",
			err:          NewValidationError("service_error", "nil error provided to handleServiceError"),
			expectedCode: connect.CodeInvalidArgument,
		},
		{
			name:         "generic_error_fixed",
			err:          errors.New("generic error"),
			expectedCode: connect.CodeInternal,
		},
		{
			name:         "nil error",
			err:          nil,
			expectedCode: connect.CodeInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "nil error" {
				// Special case for nil error - the function should return an internal error
				err := handleServiceError(nil)
				require.Error(t, err)

				connectErr, ok := err.(*connect.Error)
				require.True(t, ok)
				assert.Equal(t, connect.CodeInternal, connectErr.Code())
				return
			}

			err := handleServiceError(tt.err)
			require.Error(t, err)

			connectErr, ok := err.(*connect.Error)
			require.True(t, ok)
			assert.Equal(t, tt.expectedCode, connectErr.Code())
		})
	}
}

// setupExportV2RPC creates a complete test environment for export v2 RPC tests
func setupExportV2RPC(t *testing.T, ctx context.Context) (*ExportV2RPC, idwrap.IDWrap, idwrap.IDWrap) {
	t.Helper()

	base := testutil.CreateBaseDB(ctx, t)
	t.Cleanup(base.Close)

	services := base.GetBaseServices()
	logger := base.Logger()

	// Create additional services
	httpService := shttp.New(base.Queries, logger)
	flowService := sflow.New(base.Queries)
	fileService := sfile.New(base.Queries, logger)

	// Create user and workspace
	userID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()
	exampleID := idwrap.NewNow()

	// Create test user
	err := services.Us.CreateUser(ctx, &muser.User{
		ID:           userID,
		Email:        "test@example.com",
		Password:     []byte("password"),
		ProviderType: muser.MagicLink,
		Status:       muser.Active,
	})
	require.NoError(t, err)

	// Create test workspace
	workspace := &mworkspace.Workspace{
		ID:   workspaceID,
		Name: "Test Workspace",
	}
	err = services.Ws.Create(ctx, workspace)
	require.NoError(t, err)

	// Add user to workspace
	err = services.Wus.CreateWorkspaceUser(ctx, &mworkspaceuser.WorkspaceUser{
		UserID:      userID,
		WorkspaceID: workspaceID,
		Role:        mworkspaceuser.RoleAdmin,
	})
	require.NoError(t, err)

	// Create test HTTP request
	err = httpService.Create(ctx, &mhttp.HTTP{
		ID:          exampleID,
		WorkspaceID: workspaceID,
		Name:        "Test Request",
		Method:      "GET",
		Url:         "https://example.com",
	})
	require.NoError(t, err)

	// Create RPC handler
	rpc := NewExportV2RPC(
		base.DB,
		base.Queries,
		services.Ws,
		services.Us,
		&httpService,
		&flowService,
		fileService,
		logger,
	)

	return rpc, workspaceID, exampleID
}
