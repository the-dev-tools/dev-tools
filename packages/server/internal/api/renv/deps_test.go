package renv

import (
	"database/sql"
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream/memory"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/senv"
)

func TestEnvRPCDeps_Validate(t *testing.T) {
	// Create minimal valid dependencies for testing
	mockEnvReader := &senv.EnvReader{}
	mockVarReader := &senv.VariableReader{}
	mockEnvStream := memory.NewInMemorySyncStreamer[EnvironmentTopic, EnvironmentEvent]()
	mockVarStream := memory.NewInMemorySyncStreamer[EnvironmentVariableTopic, EnvironmentVariableEvent]()
	mockDB := &sql.DB{} // Note: This is just for validation testing, not actual DB operations

	tests := []struct {
		name    string
		deps    EnvRPCDeps
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid deps - all required fields present",
			deps: EnvRPCDeps{
				DB: mockDB,
				Readers: EnvRPCReaders{
					Env:      mockEnvReader,
					Variable: mockVarReader,
				},
				Streamers: EnvRPCStreamers{
					Env:      mockEnvStream,
					Variable: mockVarStream,
				},
			},
			wantErr: false,
		},
		{
			name: "missing DB",
			deps: EnvRPCDeps{
				DB: nil,
				Readers: EnvRPCReaders{
					Env:      mockEnvReader,
					Variable: mockVarReader,
				},
				Streamers: EnvRPCStreamers{
					Env:      mockEnvStream,
					Variable: mockVarStream,
				},
			},
			wantErr: true,
			errMsg:  "db is required",
		},
		{
			name: "missing env reader",
			deps: EnvRPCDeps{
				DB: mockDB,
				Readers: EnvRPCReaders{
					Env:      nil,
					Variable: mockVarReader,
				},
				Streamers: EnvRPCStreamers{
					Env:      mockEnvStream,
					Variable: mockVarStream,
				},
			},
			wantErr: true,
			errMsg:  "env reader is required",
		},
		{
			name: "missing variable reader",
			deps: EnvRPCDeps{
				DB: mockDB,
				Readers: EnvRPCReaders{
					Env:      mockEnvReader,
					Variable: nil,
				},
				Streamers: EnvRPCStreamers{
					Env:      mockEnvStream,
					Variable: mockVarStream,
				},
			},
			wantErr: true,
			errMsg:  "variable reader is required",
		},
		{
			name: "missing env stream",
			deps: EnvRPCDeps{
				DB: mockDB,
				Readers: EnvRPCReaders{
					Env:      mockEnvReader,
					Variable: mockVarReader,
				},
				Streamers: EnvRPCStreamers{
					Env:      nil,
					Variable: mockVarStream,
				},
			},
			wantErr: true,
			errMsg:  "env stream is required",
		},
		{
			name: "missing variable stream",
			deps: EnvRPCDeps{
				DB: mockDB,
				Readers: EnvRPCReaders{
					Env:      mockEnvReader,
					Variable: mockVarReader,
				},
				Streamers: EnvRPCStreamers{
					Env:      mockEnvStream,
					Variable: nil,
				},
			},
			wantErr: true,
			errMsg:  "variable stream is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.deps.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.errMsg)
					return
				}
				if err.Error() != tt.errMsg {
					t.Errorf("Validate() error = %q, want %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestEnvRPCReaders_Validate(t *testing.T) {
	mockEnvReader := &senv.EnvReader{}
	mockVarReader := &senv.VariableReader{}

	tests := []struct {
		name    string
		readers EnvRPCReaders
		wantErr bool
	}{
		{
			name:    "valid - all readers present",
			readers: EnvRPCReaders{Env: mockEnvReader, Variable: mockVarReader},
			wantErr: false,
		},
		{
			name:    "missing env reader",
			readers: EnvRPCReaders{Env: nil, Variable: mockVarReader},
			wantErr: true,
		},
		{
			name:    "missing variable reader",
			readers: EnvRPCReaders{Env: mockEnvReader, Variable: nil},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.readers.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEnvRPCStreamers_Validate(t *testing.T) {
	mockEnvStream := memory.NewInMemorySyncStreamer[EnvironmentTopic, EnvironmentEvent]()
	mockVarStream := memory.NewInMemorySyncStreamer[EnvironmentVariableTopic, EnvironmentVariableEvent]()

	tests := []struct {
		name      string
		streamers EnvRPCStreamers
		wantErr   bool
	}{
		{
			name:      "valid - all streamers present",
			streamers: EnvRPCStreamers{Env: mockEnvStream, Variable: mockVarStream},
			wantErr:   false,
		},
		{
			name:      "missing env stream",
			streamers: EnvRPCStreamers{Env: nil, Variable: mockVarStream},
			wantErr:   true,
		},
		{
			name:      "missing variable stream",
			streamers: EnvRPCStreamers{Env: mockEnvStream, Variable: nil},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.streamers.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNew_PanicsOnInvalidDeps(t *testing.T) {
	// Test that New() panics when given invalid deps
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("New() should panic with invalid deps")
		}
	}()

	// This should panic because DB is nil
	New(EnvRPCDeps{})
}
