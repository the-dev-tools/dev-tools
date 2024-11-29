package mwcompress

import (
	"the-dev-tools/backend/pkg/zstdcompress"

	"connectrpc.com/connect"
)

func NewCompress() connect.Compressor {
	return zstdcompress.NewZstdCompressor()
}

func NewDecompress() connect.Decompressor {
	return zstdcompress.NewZstdDecompressor()
}
