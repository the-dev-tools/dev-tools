//nolint:revive // exported
package permcheck

import (
	"errors"

	"connectrpc.com/connect"
)

func CheckPerm(ok bool, error error) *connect.Error {
	if error != nil {
		// If error is already a connect.Error, preserve it
		var connectErr *connect.Error
		if errors.As(error, &connectErr) {
			return connectErr
		}
		return connect.NewError(connect.CodeInternal, error)
	}
	if !ok {
		return connect.NewError(connect.CodePermissionDenied, nil)
	}
	return nil
}
