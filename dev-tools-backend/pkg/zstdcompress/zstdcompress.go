package zstdcompress

import (
	"io"

	"connectrpc.com/connect"
	"github.com/klauspost/compress/zstd"
)

type errorDecompressor struct {
	err error
}

func (c *errorDecompressor) Read(_ []byte) (int, error) {
	return 0, c.err
}

func (c *errorDecompressor) Reset(_ io.Reader) error {
	return c.err
}

func (c *errorDecompressor) Close() error {
	return c.err
}

type errorCompressor struct {
	err error
}

func (c *errorCompressor) Write(_ []byte) (int, error) {
	return 0, c.err
}

func (c *errorCompressor) Reset(_ io.Writer) {}

func (c *errorCompressor) Close() error {
	return c.err
}

type zstdDecompressor struct {
	decoder *zstd.Decoder
}

func (c *zstdDecompressor) Read(bytes []byte) (int, error) {
	if c.decoder == nil {
		return 0, io.EOF
	}
	return c.decoder.Read(bytes)
}

func (c *zstdDecompressor) Reset(rdr io.Reader) error {
	if c.decoder == nil {
		var err error
		c.decoder, err = zstd.NewReader(rdr)
		return err
	}
	return c.decoder.Reset(rdr)
}

func (c *zstdDecompressor) Close() error {
	if c.decoder == nil {
		return nil
	}
	c.decoder.Close()
	c.decoder = nil
	return nil
}

func NewZstdDecompressor() connect.Decompressor {
	d, err := zstd.NewReader(nil)
	if err != nil {
		return &errorDecompressor{err: err}
	}
	return &zstdDecompressor{
		decoder: d,
	}
}

func NewZstdCompressor() connect.Compressor {
	w, err := zstd.NewWriter(nil)
	if err != nil {
		return &errorCompressor{err: err}
	}
	return w
}
