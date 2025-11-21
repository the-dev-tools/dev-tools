package mocklogger

import (
	"context"
	"log/slog"
	"sync"
)

// MockHandler is a mock implementation of slog.Handler
type MockHandler struct {
	mu sync.Mutex
	// You can add fields here to track calls for testing if needed
	// For example:
	LoggedMessages []string
	LoggedLevels   []slog.Level
}

// Enabled implements slog.Handler.
func (h *MockHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

// Handle implements slog.Handler.
func (h *MockHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	// In a real test, you might want to store the record information
	if h.LoggedMessages != nil {
		h.LoggedMessages = append(h.LoggedMessages, r.Message)
	}
	if h.LoggedLevels != nil {
		h.LoggedLevels = append(h.LoggedLevels, r.Level)
	}
	return nil
}

// WithAttrs implements slog.Handler.
func (h *MockHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

// WithGroup implements slog.Handler.
func (h *MockHandler) WithGroup(name string) slog.Handler {
	return h
}

// NewMockLogger creates a new logger with the mock handler
func NewMockLogger() *slog.Logger {
	handler := &MockHandler{
		LoggedMessages: []string{},
		LoggedLevels:   []slog.Level{},
	}
	return slog.New(handler)
}
