package httpclient

import (
	"slices"
	"time"
)

// Stream presets: how a streamed response's text is assembled from its events.
const (
	StreamPresetOpenAI    = "openai"
	StreamPresetAnthropic = "anthropic"
	StreamPresetVercelAI  = "vercel-ai"
	// StreamPresetSSE is raw Server-Sent Events: the text is every data payload.
	StreamPresetSSE = "sse"
)

// StreamPresets lists every preset.
var StreamPresets = []string{StreamPresetOpenAI, StreamPresetAnthropic, StreamPresetVercelAI, StreamPresetSSE}

// IsStreamPreset reports whether p names a preset.
func IsStreamPreset(p string) bool { return slices.Contains(StreamPresets, p) }

// DefaultStreamTimeout bounds how long a stream may stay open.
const DefaultStreamTimeout = 30 * time.Second

// MaxStreamEvents bounds the events kept per response; EventCount still counts all of them.
const MaxStreamEvents = 10000

// StreamOptions asks for a response to be read as a stream.
type StreamOptions struct {
	// Preset assembles the text; "" is raw sse.
	Preset string
	// Timeout is the longest the stream may stay open; 0 is DefaultStreamTimeout.
	Timeout time.Duration
}
