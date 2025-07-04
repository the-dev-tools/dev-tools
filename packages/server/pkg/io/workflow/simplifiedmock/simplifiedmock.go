package simplifiedmock

import (
	"the-dev-tools/server/pkg/io/workflow"
)

// MockWorkflow is a mock implementation for testing
type MockWorkflow struct {
	MarshalFunc   func(*workflow.WorkspaceData, workflow.Format) ([]byte, error)
	UnmarshalFunc func([]byte, workflow.Format) (*workflow.WorkspaceData, error)
}

// Marshal calls the mock function
func (m *MockWorkflow) Marshal(data *workflow.WorkspaceData, format workflow.Format) ([]byte, error) {
	if m.MarshalFunc != nil {
		return m.MarshalFunc(data, format)
	}
	return nil, nil
}

// Unmarshal calls the mock function
func (m *MockWorkflow) Unmarshal(data []byte, format workflow.Format) (*workflow.WorkspaceData, error) {
	if m.UnmarshalFunc != nil {
		return m.UnmarshalFunc(data, format)
	}
	return nil, nil
}

// NewMockWorkflow creates a new mock workflow with default behavior
func NewMockWorkflow() *MockWorkflow {
	return &MockWorkflow{}
}

// WithMarshal sets the Marshal function
func (m *MockWorkflow) WithMarshal(fn func(*workflow.WorkspaceData, workflow.Format) ([]byte, error)) *MockWorkflow {
	m.MarshalFunc = fn
	return m
}

// WithUnmarshal sets the Unmarshal function
func (m *MockWorkflow) WithUnmarshal(fn func([]byte, workflow.Format) (*workflow.WorkspaceData, error)) *MockWorkflow {
	m.UnmarshalFunc = fn
	return m
}