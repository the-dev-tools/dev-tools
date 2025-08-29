package permcheck

import (
	"connectrpc.com/connect"
)

func CheckPerm(ok bool, error error) *connect.Error {
	if error != nil {
		// If error is already a connect.Error, preserve it
		if connectErr, isConnectErr := error.(*connect.Error); isConnectErr {
			return connectErr
		}
		return connect.NewError(connect.CodeInternal, error)
	}
	if !ok {
		return connect.NewError(connect.CodePermissionDenied, nil)
	}
	return nil
}
