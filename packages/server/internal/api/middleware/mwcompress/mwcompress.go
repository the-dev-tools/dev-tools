//nolint:revive // exported
package mwcompress

import (
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/zstdcompress"

	"connectrpc.com/connect"
)

func NewCompress() connect.Compressor {
	return zstdcompress.NewZstdCompressor()
}

func NewDecompress() connect.Decompressor {
	return zstdcompress.NewZstdDecompressor()
}
