package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

// sseServer answers with chunks, flushing each and sleeping before chunk i when delays[i] > 0.
func sseServer(t *testing.T, contentType string, chunks []string, delays map[int]time.Duration) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		f, _ := w.(http.Flusher)
		for i, c := range chunks {
			if d := delays[i]; d > 0 {
				time.Sleep(d)
			}
			_, _ = fmt.Fprint(w, c)
			f.Flush()
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func send(t *testing.T, url string, opts *StreamOptions) (Response, error) {
	t.Helper()
	return SendRequestAndConvertStream(context.Background(), http.DefaultClient,
		&Request{Method: http.MethodPost, URL: url}, idwrap.NewNow(), opts)
}

func TestStreamOpenAIChatCompletions(t *testing.T) {
	url := sseServer(t, "text/event-stream", []string{
		"data: {\"choices\":[{\"delta\":{\"role\":\"assistant\",\"content\":\"\"}}]}\n\n",
		"data: {\"choices\":[{\"delta\":{\"content\":\"Hel\"}}]}\n\n",
		"data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n",
		"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":2,\"total_tokens\":7}}\n\n",
		"data: [DONE]\n\n",
	}, map[int]time.Duration{1: 80 * time.Millisecond})
	resp, err := send(t, url, &StreamOptions{Preset: StreamPresetOpenAI})
	require.NoError(t, err)
	s := resp.Stream
	require.NotNil(t, s)
	require.Equal(t, "Hello", s.Text)
	require.Equal(t, 5, s.EventCount)
	require.Len(t, s.Events, 5)
	require.Equal(t, "[DONE]", s.Events[4].Data)
	require.InDelta(t, 2.0, s.Usage["completion_tokens"], 0)
	require.NotNil(t, s.TTFT)
	// The role-only first chunk adds no text: TTFT is the first token, after the delay.
	require.GreaterOrEqual(t, *s.TTFT, 80*time.Millisecond)
	require.Contains(t, string(resp.Body), "data: [DONE]")
}

func TestStreamOpenAIResponsesAPI(t *testing.T) {
	url := sseServer(t, "text/event-stream", []string{
		"event: response.created\ndata: {\"type\":\"response.created\"}\n\n",
		"event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"Hi \"}\n\n",
		"event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"there\"}\n\n",
		"event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":4,\"output_tokens\":2}}}\n\n",
	}, nil)
	resp, err := send(t, url, &StreamOptions{Preset: StreamPresetOpenAI})
	require.NoError(t, err)
	require.Equal(t, "Hi there", resp.Stream.Text)
	require.Equal(t, "response.output_text.delta", resp.Stream.Events[1].Event)
	require.InDelta(t, 2.0, resp.Stream.Usage["output_tokens"], 0)
}

func TestStreamAnthropic(t *testing.T) {
	url := sseServer(t, "text/event-stream", []string{
		"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":10,\"output_tokens\":1}}}\n\n",
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n",
		"event: ping\ndata: {\"type\": \"ping\"}\n\n",
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\n",
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\" world\"}}\n\n",
		"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":12}}\n\n",
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
	}, nil)
	resp, err := send(t, url, &StreamOptions{Preset: StreamPresetAnthropic})
	require.NoError(t, err)
	require.Equal(t, "Hello world", resp.Stream.Text)
	require.Equal(t, 7, resp.Stream.EventCount)
	require.InDelta(t, 10.0, resp.Stream.Usage["input_tokens"], 0)
	require.InDelta(t, 12.0, resp.Stream.Usage["output_tokens"], 0)
}

func TestStreamVercelAISSE(t *testing.T) {
	url := sseServer(t, "text/event-stream", []string{
		"data: {\"type\":\"start\"}\n\n",
		"data: {\"type\":\"text-start\",\"id\":\"t1\"}\n\n",
		"data: {\"type\":\"text-delta\",\"id\":\"t1\",\"delta\":\"Hi\"}\n\n",
		"data: {\"type\":\"text-delta\",\"id\":\"t1\",\"textDelta\":\"!\"}\n\n",
		"data: {\"type\":\"finish\"}\n\n",
		"data: [DONE]\n\n",
	}, nil)
	resp, err := send(t, url, &StreamOptions{Preset: StreamPresetVercelAI})
	require.NoError(t, err)
	require.Equal(t, "Hi!", resp.Stream.Text)
}

func TestStreamVercelAIDataStreamLines(t *testing.T) {
	// AI SDK v4 data stream: text/plain, one typed part per line.
	url := sseServer(t, "text/plain; charset=utf-8", []string{
		"f:{\"messageId\":\"m1\"}\n",
		"0:\"Hel\"\n",
		"0:\"lo\"\n",
		"e:{\"finishReason\":\"stop\",\"usage\":{\"promptTokens\":3,\"completionTokens\":2}}\n",
		"d:{\"finishReason\":\"stop\",\"usage\":{\"promptTokens\":3,\"completionTokens\":2}}\n",
	}, nil)
	resp, err := send(t, url, &StreamOptions{Preset: StreamPresetVercelAI})
	require.NoError(t, err)
	require.Equal(t, "Hello", resp.Stream.Text)
	require.Equal(t, 5, resp.Stream.EventCount)
	require.Equal(t, "0", resp.Stream.Events[1].Event)
	require.InDelta(t, 2.0, resp.Stream.Usage["completionTokens"], 0)
}

func TestStreamRawSSE(t *testing.T) {
	// Comments, CRLF line endings, a multi-line data field, event and id fields.
	url := sseServer(t, "text/event-stream; charset=utf-8", []string{
		": keep-alive\r\n\r\n",
		"event: token\r\nid: 1\r\ndata: Hello\r\n\r\n",
		"data: line one\ndata: line two\n\n",
		"data: [DONE]\n\n",
	}, map[int]time.Duration{1: 40 * time.Millisecond})
	// No options: a text/event-stream response streams with the raw preset.
	resp, err := send(t, url, nil)
	require.NoError(t, err)
	s := resp.Stream
	require.NotNil(t, s)
	require.Equal(t, StreamPresetSSE, s.Preset)
	require.Equal(t, "Helloline one\nline two", s.Text)
	require.Equal(t, 3, s.EventCount)
	require.Equal(t, StreamEvent{Event: "token", ID: "1", Data: "Hello"}, s.Events[0])
	require.GreaterOrEqual(t, *s.TTFT, 40*time.Millisecond)
}

func TestStreamNeverCloses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hi\"}}]}\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done() // never closes on its own
	}))
	t.Cleanup(srv.Close)
	start := time.Now()
	_, err := send(t, srv.URL, &StreamOptions{Preset: StreamPresetOpenAI, Timeout: 200 * time.Millisecond})
	require.Error(t, err)
	require.Contains(t, err.Error(), "stream did not end within 200ms (1 events received)")
	require.Less(t, time.Since(start), 5*time.Second)
}

func TestStreamKeepsAtMostMaxEvents(t *testing.T) {
	var b strings.Builder
	for i := range MaxStreamEvents + 5 {
		fmt.Fprintf(&b, "data: %d\n\n", i)
	}
	url := sseServer(t, "text/event-stream", []string{b.String()}, nil)
	resp, err := send(t, url, nil)
	require.NoError(t, err)
	require.Equal(t, MaxStreamEvents+5, resp.Stream.EventCount)
	require.Len(t, resp.Stream.Events, MaxStreamEvents)
}

func TestNonStreamResponseIsUnchanged(t *testing.T) {
	url := sseServer(t, "application/json", []string{`{"a":1}`}, nil)
	resp, err := send(t, url, nil)
	require.NoError(t, err)
	require.Nil(t, resp.Stream)
	require.JSONEq(t, `{"a":1}`, string(resp.Body))
}

func TestStreamResponseVar(t *testing.T) {
	ttft := 120 * time.Millisecond
	v := ConvertResponseToVar(Response{StatusCode: 200, Body: []byte("data: x\n\n"), Stream: &StreamResult{
		Preset: StreamPresetSSE, Text: "x", EventCount: 1, Events: []StreamEvent{{Data: "x"}}, TTFT: &ttft,
		Usage: map[string]any{"output_tokens": 1.0},
	}})
	m := v.StreamFields()
	require.Equal(t, "x", m["text"])
	require.InDelta(t, 120.0, m["ttft_ms"], 0)
	require.Equal(t, 1, m["event_count"])
	require.Len(t, m["events"], 1)
	require.Equal(t, map[string]any{"output_tokens": 1.0}, m["usage"])
	require.Nil(t, ConvertResponseToVar(Response{StatusCode: 200}).StreamFields())
}
