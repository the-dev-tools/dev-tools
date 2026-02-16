module github.com/the-dev-tools/dev-tools/packages/auth/auth-service

go 1.25

require (
	connectrpc.com/connect v1.19.1
	github.com/golang-jwt/jwt/v5 v5.3.0
	github.com/oklog/ulid/v2 v2.1.1
	github.com/the-dev-tools/dev-tools/packages/auth/authlib v0.0.0-00010101000000-000000000000
	github.com/the-dev-tools/dev-tools/packages/spec v0.0.0-20260109155745-2a4ef8569d93
	golang.org/x/net v0.48.0
	google.golang.org/protobuf v1.36.11
)

require (
	buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go v1.36.11-20251209175733-2a1774d88802.1 // indirect
	golang.org/x/text v0.32.0 // indirect
)

replace (
	github.com/the-dev-tools/dev-tools/packages/auth/authlib => ../authlib
	github.com/the-dev-tools/dev-tools/packages/spec => ../../spec
)
