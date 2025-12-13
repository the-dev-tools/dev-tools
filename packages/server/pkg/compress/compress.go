//nolint:revive // exported
package compress

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"sync"
	"the-dev-tools/server/pkg/zstdcompress"

	"github.com/andybalholm/brotli"
)

type CompressType = int8

const (
	CompressTypeNone CompressType = 0
	CompressTypeGzip CompressType = 1
	CompressTypeZstd CompressType = 2
	CompressTypeBr   CompressType = 3
)

var CompressLockupMap map[string]CompressType = map[string]CompressType{
	"":         CompressTypeNone,
	"identity": CompressTypeNone,
	"gzip":     CompressTypeGzip,
	"zstd":     CompressTypeZstd,
	"br":       CompressTypeBr,
}

var (
	gzipWriterPool = sync.Pool{
		New: func() interface{} {
			return gzip.NewWriter(io.Discard)
		},
	}
	brotliWriterPool = sync.Pool{
		New: func() interface{} {
			return brotli.NewWriter(io.Discard)
		},
	}
)

func Compress(data []byte, compressType CompressType) ([]byte, error) {
	var buf bytes.Buffer
	switch compressType {
	case CompressTypeGzip:
		// compress data with gzip
		z := gzipWriterPool.Get().(*gzip.Writer)
		defer gzipWriterPool.Put(z)

		z.Reset(&buf)
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
	case CompressTypeBr:
		// compress data with brotli
		w := brotliWriterPool.Get().(*brotli.Writer)
		defer brotliWriterPool.Put(w)

		w.Reset(&buf)
		_, err := w.Write(data)
		if err != nil {
			return nil, err
		}
		err = w.Close()
		if err != nil {
			return nil, err
		}
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
		defer func() { _ = z.Close() }()
		return io.ReadAll(z)

	case CompressTypeZstd:
		return zstdcompress.Decompress(data)
	case CompressTypeBr:
		// decompress data with brotli
		br := brotli.NewReader(&buf)
		return io.ReadAll(br)
	default:
		return nil, fmt.Errorf("unsupported compression type: %v", compressType)
	}
}

func DecompressWithContentEncodeStr(data []byte, contentEncoding string) ([]byte, error) {
	compressType, ok := CompressLockupMap[contentEncoding]
	if !ok {
		return nil, fmt.Errorf("%s encoding not supported", contentEncoding)
	}

	return Decompress(data, compressType)
}
