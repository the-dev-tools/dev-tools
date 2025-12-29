//nolint:revive // exported
package rhttp

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"

	"connectrpc.com/connect"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
)

// isForeignKeyConstraintError checks if the error is a foreign key constraint violation
func isForeignKeyConstraintError(err error) bool {
	if err == nil {
		return false
	}

	// SQLite foreign key constraint error patterns
	errStr := err.Error()
	return contains(errStr, "FOREIGN KEY constraint failed") ||
		contains(errStr, "foreign key constraint") ||
		contains(errStr, "constraint violation")
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					containsSubstring(s, substr)))
}

// containsSubstring performs a simple substring search
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// bytesToIDWrap converts []byte to *idwrap.IDWrap safely

func CheckOwnerHttp(ctx context.Context, hs shttp.HTTPService, us suser.UserService, httpID idwrap.IDWrap) (bool, error) {
	workspaceID, err := hs.GetWorkspaceID(ctx, httpID)
	if err != nil {
		return false, err
	}
	return mwauth.CheckOwnerWorkspace(ctx, us, workspaceID)
}

// checkWorkspaceReadAccess verifies if user has read access to workspace (any role)
func (h *HttpServiceRPC) checkWorkspaceReadAccess(ctx context.Context, workspaceID idwrap.IDWrap) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	wsUser, err := h.userReader.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceID, userID)
	if err != nil {
		if errors.Is(err, sworkspace.ErrWorkspaceUserNotFound) {
			return connect.NewError(connect.CodeNotFound, errors.New("workspace not found or access denied"))
		}
		return connect.NewError(connect.CodeInternal, err)
	}

	// Any role provides read access
	if wsUser.Role < mworkspace.RoleUser {
		return connect.NewError(connect.CodePermissionDenied, errors.New("permission denied"))
	}

	return nil
}

// checkWorkspaceWriteAccess verifies if user has write access to workspace (Admin or Owner)
func (h *HttpServiceRPC) checkWorkspaceWriteAccess(ctx context.Context, workspaceID idwrap.IDWrap) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	wsUser, err := h.userReader.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceID, userID)
	if err != nil {
		if errors.Is(err, sworkspace.ErrWorkspaceUserNotFound) {
			return connect.NewError(connect.CodeNotFound, errors.New("workspace not found or access denied"))
		}
		return connect.NewError(connect.CodeInternal, err)
	}

	// Write access requires Admin or Owner role
	if wsUser.Role < mworkspace.RoleAdmin {
		return connect.NewError(connect.CodePermissionDenied, errors.New("permission denied"))
	}

	return nil
}

// checkWorkspaceDeleteAccess verifies if user has delete access to workspace (Owner only)
func (h *HttpServiceRPC) checkWorkspaceDeleteAccess(ctx context.Context, workspaceID idwrap.IDWrap) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	wsUser, err := h.userReader.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceID, userID)
	if err != nil {
		if errors.Is(err, sworkspace.ErrWorkspaceUserNotFound) {
			return connect.NewError(connect.CodeNotFound, errors.New("workspace not found or access denied"))
		}
		return connect.NewError(connect.CodeInternal, err)
	}

	// Delete access requires Owner role only
	if wsUser.Role != mworkspace.RoleOwner {
		return connect.NewError(connect.CodePermissionDenied, errors.New("permission denied"))
	}

	return nil
}

// executeHTTPRequest performs the actual HTTP request execution
// cloneStringMapToAny converts a map[string]string to map[string]any
// This follows the pattern from nrequest.go

// isNetworkError checks if the error is a network-related error
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "network is unreachable") ||
		strings.Contains(errStr, "no such host") ||
		isDNSError(err)
}

// isTimeoutError checks if the error is a timeout error
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") ||
		errors.Is(err, context.DeadlineExceeded)
}

// isDNSError checks if the error is a DNS resolution error
func isDNSError(err error) bool {
	if err == nil {
		return false
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		var netErr *net.DNSError
		if errors.As(urlErr.Err, &netErr) {
			return true
		}
	}

	errStr := err.Error()
	return strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "name resolution failed")
}

// parseHttpMethod converts string method to HttpMethod enum
func parseHttpMethod(method string) apiv1.HttpMethod {
	switch strings.ToUpper(method) {
	case "GET":
		return apiv1.HttpMethod_HTTP_METHOD_GET
	case "POST":
		return apiv1.HttpMethod_HTTP_METHOD_POST
	case "PUT":
		return apiv1.HttpMethod_HTTP_METHOD_PUT
	case "PATCH":
		return apiv1.HttpMethod_HTTP_METHOD_PATCH
	case "DELETE":
		return apiv1.HttpMethod_HTTP_METHOD_DELETE
	case "HEAD":
		return apiv1.HttpMethod_HTTP_METHOD_HEAD
	case "OPTION":
		return apiv1.HttpMethod_HTTP_METHOD_OPTION
	case "CONNECT":
		return apiv1.HttpMethod_HTTP_METHOD_CONNECT
	default:
		return apiv1.HttpMethod_HTTP_METHOD_UNSPECIFIED
	}
}

// httpMethodToString converts HttpMethod enum to string
func httpMethodToString(method *apiv1.HttpMethod) *string {
	if method == nil {
		return nil
	}

	var result string
	switch *method {
	case apiv1.HttpMethod_HTTP_METHOD_GET:
		result = "GET"
	case apiv1.HttpMethod_HTTP_METHOD_POST:
		result = "POST"
	case apiv1.HttpMethod_HTTP_METHOD_PUT:
		result = "PUT"
	case apiv1.HttpMethod_HTTP_METHOD_PATCH:
		result = "PATCH"
	case apiv1.HttpMethod_HTTP_METHOD_DELETE:
		result = "DELETE"
	case apiv1.HttpMethod_HTTP_METHOD_HEAD:
		result = "HEAD"
	case apiv1.HttpMethod_HTTP_METHOD_OPTION:
		result = "OPTION"
	case apiv1.HttpMethod_HTTP_METHOD_CONNECT:
		result = "CONNECT"
	default:
		result = ""
	}
	return &result
}

// getStatusText returns the standard HTTP status text for a status code
func (h *HttpServiceRPC) getStatusText(statusCode int) string {
	text := http.StatusText(statusCode)
	if text == "" {
		return "Unknown"
	}
	return text
}

// constructAssertionExpression constructs an expression from key and value
