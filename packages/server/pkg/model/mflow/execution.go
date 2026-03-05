//nolint:revive // exported
package mflow

import (
	"encoding/json"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/compress"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

type NodeExecution struct {
	ID                     idwrap.IDWrap  `json:"id"`
	NodeID                 idwrap.IDWrap  `json:"node_id"`
	Name                   string         `json:"name"`
	State                  int8           `json:"state"`
	Error                  *string        `json:"error,omitempty"`
	InputData              []byte         `json:"input_data,omitempty"`
	InputDataCompressType  int8           `json:"input_data_compress_type"`
	OutputData             []byte         `json:"output_data,omitempty"`
	OutputDataCompressType int8           `json:"output_data_compress_type"`
	ResponseID             *idwrap.IDWrap `json:"response_id,omitempty"`
	GraphQLResponseID      *idwrap.IDWrap `json:"graphql_response_id,omitempty"`
	CompletedAt            *int64         `json:"completed_at,omitempty"`
}

// Helper methods for JSON handling with compression
func (ne *NodeExecution) GetInputJSON() (json.RawMessage, error) {
	if ne.InputData == nil {
		return nil, nil
	}
	if ne.InputDataCompressType == compress.CompressTypeNone {
		return ne.InputData, nil
	}
	return compress.Decompress(ne.InputData, ne.InputDataCompressType)
}

func (ne *NodeExecution) SetInputJSON(data json.RawMessage) error {
	// For small data (< 1KB), don't compress
	if len(data) < 1024 {
		ne.InputData = data
		ne.InputDataCompressType = compress.CompressTypeNone
		return nil
	}

	// Use zstd compression for larger data
	compressed, err := compress.Compress(data, compress.CompressTypeZstd)
	if err != nil {
		return err
	}

	// Only use compressed if it's actually smaller
	if len(compressed) < len(data) {
		ne.InputData = compressed
		ne.InputDataCompressType = compress.CompressTypeZstd
	} else {
		ne.InputData = data
		ne.InputDataCompressType = compress.CompressTypeNone
	}
	return nil
}

// Similar methods for output data
func (ne *NodeExecution) GetOutputJSON() (json.RawMessage, error) {
	if ne.OutputData == nil {
		return nil, nil
	}
	if ne.OutputDataCompressType == compress.CompressTypeNone {
		return ne.OutputData, nil
	}
	return compress.Decompress(ne.OutputData, ne.OutputDataCompressType)
}

func (ne *NodeExecution) SetOutputJSON(data json.RawMessage) error {
	// Same compression logic as SetInputJSON
	if len(data) < 1024 {
		ne.OutputData = data
		ne.OutputDataCompressType = compress.CompressTypeNone
		return nil
	}

	compressed, err := compress.Compress(data, compress.CompressTypeZstd)
	if err != nil {
		return err
	}

	if len(compressed) < len(data) {
		ne.OutputData = compressed
		ne.OutputDataCompressType = compress.CompressTypeZstd
	} else {
		ne.OutputData = data
		ne.OutputDataCompressType = compress.CompressTypeNone
	}
	return nil
}
