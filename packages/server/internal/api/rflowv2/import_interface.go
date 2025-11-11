package rflowv2

import (
	"context"

	"the-dev-tools/server/pkg/idwrap"
)

// WorkspaceImporter defines the interface for importing workspace data
type WorkspaceImporter interface {
	ImportWorkspaceFromYAML(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*ImportResults, error)
	ImportWorkspaceFromCurl(ctx context.Context, curlData []byte, workspaceID idwrap.IDWrap) (*ImportResults, error)
}

// ImportResults represents the results of a workspace import operation
type ImportResults struct {
	WorkspaceID    idwrap.IDWrap
	HTTPReqsCreated int
	HTTPReqsUpdated int
	HTTPReqsSkipped int
	HTTPReqsFailed  int
	FilesCreated    int
	FilesUpdated    int
	FilesSkipped    int
	FilesFailed     int
	FlowsCreated    int
	FlowsUpdated    int
	FlowsSkipped    int
	FlowsFailed     int
	NodesCreated    int
	NodesUpdated    int
	NodesSkipped    int
	NodesFailed     int
	Duration        int64
}

// MockWorkspaceImporter provides a mock implementation for testing
type MockWorkspaceImporter struct {
	results *ImportResults
	err     error
}

func (m *MockWorkspaceImporter) ImportWorkspaceFromYAML(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*ImportResults, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

func (m *MockWorkspaceImporter) ImportWorkspaceFromCurl(ctx context.Context, curlData []byte, workspaceID idwrap.IDWrap) (*ImportResults, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

// NewMockWorkspaceImporter creates a new mock importer
func NewMockWorkspaceImporter(results *ImportResults, err error) *MockWorkspaceImporter {
	return &MockWorkspaceImporter{
		results: results,
		err:     err,
	}
}