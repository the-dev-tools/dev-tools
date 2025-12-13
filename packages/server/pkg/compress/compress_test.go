package compress

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompressDecompress(t *testing.T) {
	data := []byte("Hello, world! This is a test string to compress.")

	tests := []struct {
		name    string
		algo    CompressType
		encoded string
	}{
		{
			name: "Gzip",
			algo: CompressTypeGzip,
		},
		{
			name: "Zstd",
			algo: CompressTypeZstd,
		},
		{
			name: "Brotli",
			algo: CompressTypeBr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Compress
			compressed, err := Compress(data, tt.algo)
			require.NoError(t, err)
			assert.NotEmpty(t, compressed)

			// Decompress
			decompressed, err := Decompress(compressed, tt.algo)
			require.NoError(t, err)
			assert.Equal(t, data, decompressed)
		})
	}
}

func TestDecompressWithContentEncodeStr(t *testing.T) {
	data := []byte("Hello, Content-Encoding!")

	tests := []struct {
		name     string
		encoding string
		algo     CompressType
	}{
		{"gzip", "gzip", CompressTypeGzip},
		{"zstd", "zstd", CompressTypeZstd},
		{"br", "br", CompressTypeBr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compressed, err := Compress(data, tt.algo)
			require.NoError(t, err)

			decompressed, err := DecompressWithContentEncodeStr(compressed, tt.encoding)
			require.NoError(t, err)
			assert.Equal(t, data, decompressed)
		})
	}
}
