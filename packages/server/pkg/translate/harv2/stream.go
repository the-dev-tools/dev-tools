package harv2

import (
	"encoding/base64"
	"encoding/json"
	"mime"
	"regexp"
	"strings"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/httpclient"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
)

// Streaming responses. An entry whose response is text/event-stream becomes a request that is
// read as a stream (`stream: <preset>`). The preset is detected from the recorded events with
// the same rules as the Stresseur generator (stress/internal/generator/stream.go), so a HAR
// import and a Stresseur recording of the same endpoint agree. Browsers often store an event
// stream's body as plain text, as base64 or not at all; without a body the preset is sse.

var anthropicEventTypes = map[string]bool{"message_start": true, "content_block_start": true, "content_block_delta": true,
	"content_block_stop": true, "message_delta": true, "message_stop": true, "ping": true}

var vercelEventTypes = map[string]bool{"start": true, "start-step": true, "text-start": true, "text-delta": true, "text-end": true,
	"finish-step": true, "finish": true, "reasoning-delta": true, "tool-input-delta": true}

var vercelV4Line = regexp.MustCompile(`^[0-9a-z]:`)

// entryStreamPreset is the stream: preset of an entry whose response is an event stream; ""
// when it isn't one.
func entryStreamPreset(entry Entry) string {
	if !isEventStreamType(entry.Response.Content.MimeType) && !isEventStreamType(responseHeader(entry, "Content-Type")) {
		return ""
	}
	text := entry.Response.Content.Text
	if strings.EqualFold(entry.Response.Content.Encoding, "base64") {
		b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(text))
		if err != nil {
			return httpclient.StreamPresetSSE
		}
		text = string(b)
	}
	if strings.TrimSpace(text) == "" {
		return httpclient.StreamPresetSSE
	}
	return detectStreamPreset(text)
}

func isEventStreamType(contentType string) bool {
	mt, _, err := mime.ParseMediaType(contentType)
	return err == nil && strings.EqualFold(mt, "text/event-stream")
}

func responseHeader(entry Entry, name string) string {
	for _, h := range entry.Response.Headers {
		if strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}

// detectStreamPreset reads the first event of a raw event stream.
func detectStreamPreset(text string) string {
	event := ""
	var data []string
	lines := strings.Split(strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(text), "\n")
	for _, line := range append(lines, "") {
		switch {
		case line == "":
			if len(data) == 0 {
				event = ""
				continue
			}
			payload := strings.Join(data, "\n")
			if payload == "[DONE]" {
				data, event = nil, ""
				continue
			}
			return streamPresetOf(event, payload)
		case strings.HasPrefix(line, ":"):
		case strings.HasPrefix(line, "data:"):
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case len(data) == 0 && vercelV4Line.MatchString(line):
			return httpclient.StreamPresetVercelAI
		}
	}
	return httpclient.StreamPresetSSE
}

func streamPresetOf(event, payload string) string {
	if anthropicEventTypes[event] {
		return httpclient.StreamPresetAnthropic
	}
	var obj map[string]any
	if json.Unmarshal([]byte(payload), &obj) != nil {
		return httpclient.StreamPresetSSE
	}
	typ, _ := obj["type"].(string)
	_, choices := obj["choices"].([]any)
	switch {
	case choices || obj["object"] == "chat.completion.chunk" || strings.HasPrefix(typ, "response."):
		return httpclient.StreamPresetOpenAI
	case anthropicEventTypes[typ]:
		return httpclient.StreamPresetAnthropic
	case vercelEventTypes[typ]:
		return httpclient.StreamPresetVercelAI
	}
	return httpclient.StreamPresetSSE
}

// addEntryStream records a streaming entry's preset on its (base) HTTP request, once.
func addEntryStream(result *HarResolved, entry Entry, httpID idwrap.IDWrap) {
	preset := entryStreamPreset(entry)
	if preset == "" {
		return
	}
	for _, s := range result.HTTPStreams {
		if s.HttpID == httpID {
			return
		}
	}
	result.HTTPStreams = append(result.HTTPStreams, mhttp.HTTPStream{HttpID: httpID, Preset: preset})
}
