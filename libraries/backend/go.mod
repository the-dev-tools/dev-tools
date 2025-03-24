module the-dev-tools/backend

go 1.24.1

require (
	connectrpc.com/connect v1.18.1
	github.com/PaesslerAG/gval v1.2.3
	github.com/bufbuild/httplb v0.3.0
	github.com/goccy/go-json v0.10.3
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/klauspost/compress v1.17.9
	github.com/oklog/ulid/v2 v2.1.0
	github.com/rs/cors v1.11.0
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842
	golang.org/x/net v0.33.0
	google.golang.org/protobuf v1.36.4
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/shopspring/decimal v1.3.1 // indirect
	github.com/stretchr/testify v1.10.0 // indirect
	golang.org/x/sync v0.10.0 // indirect
	golang.org/x/text v0.21.0 // indirect
)

tool (
	connectrpc.com/connect/cmd/protoc-gen-connect-go
	github.com/sqlc-dev/sqlc/cmd/sqlc
	google.golang.org/protobuf/cmd/protoc-gen-go
)
