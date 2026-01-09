module github.com/the-dev-tools/dev-tools/packages/server

go 1.25

require (
	connectrpc.com/connect v1.18.1
	github.com/andybalholm/brotli v1.1.1
	github.com/expr-lang/expr v1.17.2
	github.com/goccy/go-json v0.10.5
	github.com/golang-jwt/jwt/v5 v5.2.2
	github.com/klauspost/compress v1.18.0
	github.com/lithammer/fuzzysearch v1.1.8
	github.com/oklog/ulid/v2 v2.1.0
	github.com/rs/cors v1.11.0
	github.com/stretchr/testify v1.11.1
	github.com/the-dev-tools/dev-tools/packages/db v0.0.0-00010101000000-000000000000
	github.com/the-dev-tools/dev-tools/packages/spec v0.0.0-00010101000000-000000000000
	golang.org/x/net v0.44.0
	golang.org/x/text v0.29.0
	google.golang.org/protobuf v1.36.10
	gopkg.in/yaml.v3 v3.0.1
)

require (
	buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go v1.36.10-20250912141014-52f32327d4b0.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/ncruces/go-strftime v0.1.9 // indirect
	github.com/pingcap/log v1.1.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/exp v0.0.0-20250620022241-b7579e27df2b // indirect
	golang.org/x/mod v0.28.0 // indirect
	golang.org/x/sys v0.36.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	modernc.org/libc v1.66.10 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
	modernc.org/sqlite v1.40.1 // indirect
)

replace (
	github.com/the-dev-tools/dev-tools/packages/db => ../db
	github.com/the-dev-tools/dev-tools/packages/spec => ../spec
)
