package zstdcompress_test

import (
	"bytes"
	"dev-tools-backend/pkg/zstdcompress"
	"testing"
)

func TestZstdDecompressor_Read(t *testing.T) {
	t.Parallel()
	SomeBodyBytes := make([]byte, 1024)
	for i := 0; i < 1024; i++ {
		SomeBodyBytes[i] = byte(i % 256)
	}

	CompressedData := zstdcompress.Compress(SomeBodyBytes)
	if len(CompressedData) == 0 {
		t.Errorf("Compressed data is empty")
	}

	DecompressedData, err := zstdcompress.Decompress(CompressedData)
	if err != nil {
		t.Errorf("Error in decompressing data")
	}

	if len(DecompressedData) == 0 {
		t.Errorf("Decompressed data is empty")
	}

	if len(DecompressedData) != len(SomeBodyBytes) {
		t.Errorf("Decompressed data length is not equal to original data length")
	}

	if !bytes.Equal(DecompressedData, SomeBodyBytes) {
		t.Errorf("Decompressed data is not equal to original data")
	}
}
