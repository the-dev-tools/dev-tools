package compress

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"the-dev-tools/backend/pkg/zstdcompress"
)

type CompressType int8

const (
	CompressTypeNone CompressType = 0
	CompressTypeGzip CompressType = 1
	CompressTypeZstd CompressType = 2
)

// TODO: refactor this for better performance
func Compress(data []byte, compressType CompressType) ([]byte, error) {
	var buf bytes.Buffer
	switch compressType {
	case CompressTypeGzip:
		// compress data with gzip
		z := gzip.NewWriter(&buf)
		_, err := z.Write(data)
		if err != nil {
			return nil, err
		}
		err = z.Close()
		if err != nil {
			return nil, err
		}

	case CompressTypeZstd:
		byteArr := zstdcompress.Compress(data)
		buf.Write(byteArr)
	}
	return buf.Bytes(), nil
}

func Decompress(data []byte, compressType CompressType) ([]byte, error) {
	var buf bytes.Buffer
	buf.Write(data)

	switch compressType {
	case CompressTypeGzip:
		// decompress data with gzip
		z, err := gzip.NewReader(&buf)
		if err != nil {
			return nil, err
		}
		err = z.Close()
		if err != nil {
			return nil, err
		}
		return io.ReadAll(z)

	case CompressTypeZstd:
		return zstdcompress.Decompress(data)

	default:
		return nil, fmt.Errorf("unsupported compression type: %v", compressType)
	}
}
