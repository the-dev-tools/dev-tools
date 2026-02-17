package rcredential

import (
	"database/sql"
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream/memory"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/scredential"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
)

func TestCredentialRPCDeps_Validate(t *testing.T) {
	// Create minimal valid dependencies for testing
	mockCredReader := &scredential.CredentialReader{}
	mockUserReader := &sworkspace.UserReader{}
	mockCredStream := memory.NewInMemorySyncStreamer[CredentialTopic, CredentialEvent]()
	mockOpenAiStream := memory.NewInMemorySyncStreamer[CredentialOpenAiTopic, CredentialOpenAiEvent]()
	mockGeminiStream := memory.NewInMemorySyncStreamer[CredentialGeminiTopic, CredentialGeminiEvent]()
	mockAnthropicStream := memory.NewInMemorySyncStreamer[CredentialAnthropicTopic, CredentialAnthropicEvent]()
	mockDB := &sql.DB{} // Note: This is just for validation testing, not actual DB operations

	tests := []struct {
		name    string
		deps    CredentialRPCDeps
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid deps - all required fields present",
			deps: CredentialRPCDeps{
				DB:       mockDB,
				Services: CredentialRPCServices{},
				Readers: CredentialRPCReaders{
					Credential: mockCredReader,
					User:       mockUserReader,
				},
				Streamers: CredentialRPCStreamers{
					Credential: mockCredStream,
					OpenAi:     mockOpenAiStream,
					Gemini:     mockGeminiStream,
					Anthropic:  mockAnthropicStream,
				},
			},
			wantErr: false,
		},
		{
			name: "missing DB",
			deps: CredentialRPCDeps{
				DB:       nil,
				Services: CredentialRPCServices{},
				Readers: CredentialRPCReaders{
					Credential: mockCredReader,
					User:       mockUserReader,
				},
				Streamers: CredentialRPCStreamers{
					Credential: mockCredStream,
					OpenAi:     mockOpenAiStream,
					Gemini:     mockGeminiStream,
					Anthropic:  mockAnthropicStream,
				},
			},
			wantErr: true,
			errMsg:  "db is required",
		},
		{
			name: "missing credential reader",
			deps: CredentialRPCDeps{
				DB:       mockDB,
				Services: CredentialRPCServices{},
				Readers: CredentialRPCReaders{
					Credential: nil,
					User:       mockUserReader,
				},
				Streamers: CredentialRPCStreamers{
					Credential: mockCredStream,
					OpenAi:     mockOpenAiStream,
					Gemini:     mockGeminiStream,
					Anthropic:  mockAnthropicStream,
				},
			},
			wantErr: true,
			errMsg:  "credential reader is required",
		},
		{
			name: "missing user reader",
			deps: CredentialRPCDeps{
				DB:       mockDB,
				Services: CredentialRPCServices{},
				Readers: CredentialRPCReaders{
					Credential: mockCredReader,
					User:       nil,
				},
				Streamers: CredentialRPCStreamers{
					Credential: mockCredStream,
					OpenAi:     mockOpenAiStream,
					Gemini:     mockGeminiStream,
					Anthropic:  mockAnthropicStream,
				},
			},
			wantErr: true,
			errMsg:  "user reader is required",
		},
		{
			name: "missing credential stream",
			deps: CredentialRPCDeps{
				DB:       mockDB,
				Services: CredentialRPCServices{},
				Readers: CredentialRPCReaders{
					Credential: mockCredReader,
					User:       mockUserReader,
				},
				Streamers: CredentialRPCStreamers{
					Credential: nil,
					OpenAi:     mockOpenAiStream,
					Gemini:     mockGeminiStream,
					Anthropic:  mockAnthropicStream,
				},
			},
			wantErr: true,
			errMsg:  "credential stream is required",
		},
		{
			name: "missing openai stream",
			deps: CredentialRPCDeps{
				DB:       mockDB,
				Services: CredentialRPCServices{},
				Readers: CredentialRPCReaders{
					Credential: mockCredReader,
					User:       mockUserReader,
				},
				Streamers: CredentialRPCStreamers{
					Credential: mockCredStream,
					OpenAi:     nil,
					Gemini:     mockGeminiStream,
					Anthropic:  mockAnthropicStream,
				},
			},
			wantErr: true,
			errMsg:  "openai stream is required",
		},
		{
			name: "missing gemini stream",
			deps: CredentialRPCDeps{
				DB:       mockDB,
				Services: CredentialRPCServices{},
				Readers: CredentialRPCReaders{
					Credential: mockCredReader,
					User:       mockUserReader,
				},
				Streamers: CredentialRPCStreamers{
					Credential: mockCredStream,
					OpenAi:     mockOpenAiStream,
					Gemini:     nil,
					Anthropic:  mockAnthropicStream,
				},
			},
			wantErr: true,
			errMsg:  "gemini stream is required",
		},
		{
			name: "missing anthropic stream",
			deps: CredentialRPCDeps{
				DB:       mockDB,
				Services: CredentialRPCServices{},
				Readers: CredentialRPCReaders{
					Credential: mockCredReader,
					User:       mockUserReader,
				},
				Streamers: CredentialRPCStreamers{
					Credential: mockCredStream,
					OpenAi:     mockOpenAiStream,
					Gemini:     mockGeminiStream,
					Anthropic:  nil,
				},
			},
			wantErr: true,
			errMsg:  "anthropic stream is required",
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

func TestCredentialRPCReaders_Validate(t *testing.T) {
	mockCredReader := &scredential.CredentialReader{}
	mockUserReader := &sworkspace.UserReader{}

	tests := []struct {
		name    string
		readers CredentialRPCReaders
		wantErr bool
	}{
		{
			name:    "valid - all readers present",
			readers: CredentialRPCReaders{Credential: mockCredReader, User: mockUserReader},
			wantErr: false,
		},
		{
			name:    "missing credential reader",
			readers: CredentialRPCReaders{Credential: nil, User: mockUserReader},
			wantErr: true,
		},
		{
			name:    "missing user reader",
			readers: CredentialRPCReaders{Credential: mockCredReader, User: nil},
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

func TestCredentialRPCStreamers_Validate(t *testing.T) {
	mockCredStream := memory.NewInMemorySyncStreamer[CredentialTopic, CredentialEvent]()
	mockOpenAiStream := memory.NewInMemorySyncStreamer[CredentialOpenAiTopic, CredentialOpenAiEvent]()
	mockGeminiStream := memory.NewInMemorySyncStreamer[CredentialGeminiTopic, CredentialGeminiEvent]()
	mockAnthropicStream := memory.NewInMemorySyncStreamer[CredentialAnthropicTopic, CredentialAnthropicEvent]()

	tests := []struct {
		name      string
		streamers CredentialRPCStreamers
		wantErr   bool
	}{
		{
			name: "valid - all streamers present",
			streamers: CredentialRPCStreamers{
				Credential: mockCredStream,
				OpenAi:     mockOpenAiStream,
				Gemini:     mockGeminiStream,
				Anthropic:  mockAnthropicStream,
			},
			wantErr: false,
		},
		{
			name: "missing credential stream",
			streamers: CredentialRPCStreamers{
				Credential: nil,
				OpenAi:     mockOpenAiStream,
				Gemini:     mockGeminiStream,
				Anthropic:  mockAnthropicStream,
			},
			wantErr: true,
		},
		{
			name: "missing openai stream",
			streamers: CredentialRPCStreamers{
				Credential: mockCredStream,
				OpenAi:     nil,
				Gemini:     mockGeminiStream,
				Anthropic:  mockAnthropicStream,
			},
			wantErr: true,
		},
		{
			name: "missing gemini stream",
			streamers: CredentialRPCStreamers{
				Credential: mockCredStream,
				OpenAi:     mockOpenAiStream,
				Gemini:     nil,
				Anthropic:  mockAnthropicStream,
			},
			wantErr: true,
		},
		{
			name: "missing anthropic stream",
			streamers: CredentialRPCStreamers{
				Credential: mockCredStream,
				OpenAi:     mockOpenAiStream,
				Gemini:     mockGeminiStream,
				Anthropic:  nil,
			},
			wantErr: true,
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
	New(CredentialRPCDeps{})
}
