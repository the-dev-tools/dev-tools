package harv2

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

// The same cases as the Stresseur generator's detection, so a HAR import and a recording agree.
func TestDetectStreamPreset(t *testing.T) {
	cases := map[string]string{
		"data: {\"choices\":[{\"delta\":{\"content\":\"Hi\"}}]}\n\ndata: [DONE]\n\n": "openai",
		"data: {\"object\":\"chat.completion.chunk\"}\n\n":                           "openai",
		"event: response.created\ndata: {\"type\":\"response.created\"}\n\n":         "openai",
		"event: message_start\r\ndata: {}\r\n\r\n":                                   "anthropic",
		"data: {\"type\":\"content_block_delta\"}\n\n":                               "anthropic",
		"data: {\"type\":\"start\"}\n\n":                                             "vercel-ai",
		"0:\"Hel\"\n0:\"lo\"\n":                                                      "vercel-ai",
		": hi\n\ndata: tick\n\n":                                                     "sse",
		"data: [DONE]\n\ndata: {\"choices\":[]}\n\n":                                 "openai",
		"data: {\"type\":\"unknown\"}\n\n":                                           "sse",
		"":                                                                           "sse",
	}
	for text, want := range cases {
		require.Equal(t, want, detectStreamPreset(text), "%q", text)
	}
}

func TestHARStreamEntriesGetAPreset(t *testing.T) {
	data, err := os.ReadFile("testdata/streams.har")
	require.NoError(t, err)
	har, err := ConvertRaw(data)
	require.NoError(t, err)
	resolved, err := ConvertHAR(har, idwrap.NewNow())
	require.NoError(t, err)

	presetByPath := map[string]string{}
	for _, s := range resolved.HTTPStreams {
		for _, h := range resolved.HTTPRequests {
			if h.ID == s.HttpID {
				presetByPath[h.Url[strings.Index(h.Url, "/api/"):]] = s.Preset
			}
		}
	}
	require.Equal(t, map[string]string{
		"/api/openai":    "openai",
		"/api/anthropic": "anthropic", // base64-encoded body
		"/api/vercel":    "vercel-ai",
		"/api/vercel-v4": "vercel-ai",
		"/api/ticks":     "sse",
		"/api/empty":     "sse", // empty body: the mimeType alone marks it
		"/api/no-text":   "sse", // no text field at all
	}, presetByPath)

	// Streams are kept on the base request the flow's request node points to.
	bases := map[idwrap.IDWrap]bool{}
	for _, rn := range resolved.RequestNodes {
		bases[*rn.HttpID] = true
	}
	for _, s := range resolved.HTTPStreams {
		require.True(t, bases[s.HttpID])
	}
}
