package connect

// Request is a minimal stub of connectrpc.com/connect.Request for testing.
type Request[T any] struct{ Msg *T }

// Response is a minimal stub of connectrpc.com/connect.Response for testing.
type Response[T any] struct{ Msg *T }

// ServerStream is a minimal stub of connectrpc.com/connect.ServerStream for testing.
type ServerStream[T any] struct{}

func NewRequest[T any](msg *T) *Request[T]   { return nil }
func NewResponse[T any](msg *T) *Response[T] { return nil }
