//go:build tools
// +build tools

package tools

// Workaround for tool dependency management. Replace with built-in solution once Go 1.24 is released
// https://marcofranssen.nl/manage-go-tools-via-go-modules
// https://tip.golang.org/doc/go1.24#tools

import (
	_ "connectrpc.com/connect/cmd/protoc-gen-connect-go"
	_ "github.com/sqlc-dev/sqlc/cmd/sqlc"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
)
