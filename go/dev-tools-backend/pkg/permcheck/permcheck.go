package permcheck

import (
	"connectrpc.com/connect"
)

func CheckPerm(ok bool, error error) *connect.Error {
	if error != nil {
		return connect.NewError(connect.CodeInternal, error)
	}
	if !ok {
		return connect.NewError(connect.CodePermissionDenied, nil)
	}
	return nil
}
