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

// pool is a type-safe wrapper around sync.Pool
type pool[T any] struct {
	internal sync.Pool
}

func newPool[T any](newFn func() T) *pool[T] {
	return &pool[T]{
		internal: sync.Pool{
			New: func() interface{} { return newFn() },
		},
	}
}

func (p *pool[T]) Get() T {
	return p.internal.Get().(T)
}

func (p *pool[T]) Put(x T) {
	p.internal.Put(x)
}

var (
	gzipWriterPool = newPool(func() *gzip.Writer {
		return gzip.NewWriter(io.Discard)
	})
	brotliWriterPool = newPool(func() *brotli.Writer {
		return brotli.NewWriter(io.Discard)
	})
)

func Compress(data []byte, compressType CompressType) ([]byte, error) {
	switch compressType {
	case CompressTypeGzip:
		return compressGzip(data)
	case CompressTypeZstd:
		return compressZstd(data)
	case CompressTypeBr:
		return compressBrotli(data)
	default:
		// CompressTypeNone or unknown
		return data, nil
	}
}

func compressGzip(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	z := gzipWriterPool.Get()
	defer gzipWriterPool.Put(z)

	z.Reset(&buf)
	if _, err := z.Write(data); err != nil {
		return nil, err
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func compressBrotli(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := brotliWriterPool.Get()
	defer brotliWriterPool.Put(w)

	w.Reset(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func compressZstd(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	byteArr := zstdcompress.Compress(data)
	buf.Write(byteArr)
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
	case CompressTypeNone:
		return data, nil
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