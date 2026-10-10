package httpclient

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

// StreamEvent is one event of a streamed response. Data is the parsed JSON when the payload
// is JSON, else the text.
type StreamEvent struct {
	Event string `json:"event,omitempty"`
	Data  any    `json:"data"`
	ID    string `json:"id,omitempty"`
}

// StreamResult is what a streamed response assembled.
type StreamResult struct {
	Preset string `json:"preset"`
	// Text is the message assembled by the preset.
	Text string `json:"text"`
	// Events are the first MaxStreamEvents events; EventCount counts all of them.
	Events     []StreamEvent `json:"events,omitempty"`
	EventCount int           `json:"event_count"`
	// TTFT is the time from sending the request to the first event that added text (raw sse:
	// the first data event). Nil when none did.
	TTFT *time.Duration `json:"ttft,omitempty"`
	// Usage is the token usage the stream reported, merged across events.
	Usage map[string]any `json:"usage,omitempty"`
}

// isEventStream reports whether a Content-Type is text/event-stream.
func isEventStream(contentType string) bool {
	mt, _, err := mime.ParseMediaType(contentType)
	return err == nil && strings.EqualFold(mt, "text/event-stream")
}

// SendRequestAndConvertStream sends a request and converts the response. The body is read as a
// stream when opts is set or the response is text/event-stream; otherwise it is read whole,
// as SendRequestAndConvertWithContext does.
func SendRequestAndConvertStream(ctx context.Context, client HttpClient, req *Request, exampleID idwrap.IDWrap, opts *StreamOptions) (Response, error) {
	start := time.Now()
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	resp, err := SendRequestWithContext(streamCtx, client, req)
	if err != nil {
		return Response{}, err
	}
	if opts == nil && !isEventStream(resp.Header.Get("Content-Type")) {
		return convertResponse(resp)
	}
	defer func() { _ = resp.Body.Close() }()

	o := StreamOptions{Preset: StreamPresetSSE, Timeout: DefaultStreamTimeout}
	if opts != nil {
		if opts.Preset != "" {
			o.Preset = opts.Preset
		}
		if opts.Timeout > 0 {
			o.Timeout = opts.Timeout
		}
	}
	timedOut := make(chan struct{})
	timer := time.AfterFunc(o.Timeout, func() {
		close(timedOut)
		cancel()
	})
	defer timer.Stop()

	body, result, err := readStream(resp, o.Preset, start)
	if err != nil {
		select {
		case <-timedOut:
			return Response{}, fmt.Errorf("stream did not end within %s (%d events received)", o.Timeout, result.EventCount)
		default:
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Response{}, ctxErr
		}
		return Response{}, fmt.Errorf("reading stream: %w", err)
	}
	return Response{
		StatusCode: resp.StatusCode,
		Body:       body,
		Headers:    ConvertHttpHeaderToHeaders(resp.Header),
		Stream:     result,
	}, nil
}

// decodedBody undoes a Content-Encoding the transport left in place, as a stream.
func decodedBody(resp *http.Response) (io.Reader, error) {
	switch strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding"))) {
	case "", "identity":
		return resp.Body, nil
	case "gzip", "x-gzip":
		return gzip.NewReader(resp.Body)
	case "deflate":
		return zlib.NewReader(resp.Body)
	case "br":
		return brotli.NewReader(resp.Body), nil
	case "zstd":
		d, err := zstd.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		return d.IOReadCloser(), nil
	default:
		return nil, fmt.Errorf("unsupported Content-Encoding %q on a stream", resp.Header.Get("Content-Encoding"))
	}
}

// streamParser assembles events and text as lines arrive.
type streamParser struct {
	preset string
	start  time.Time
	res    *StreamResult
	text   strings.Builder

	// The event being read (SSE fields until a blank line).
	event   string
	id      string
	data    []string
	hasData bool
}

func readStream(resp *http.Response, preset string, start time.Time) ([]byte, *StreamResult, error) {
	p := &streamParser{preset: preset, start: start, res: &StreamResult{Preset: preset}}
	r, err := decodedBody(resp)
	if err != nil {
		return nil, p.res, err
	}
	var raw bytes.Buffer
	br := bufio.NewReader(io.TeeReader(r, &raw))
	for {
		line, err := br.ReadString('\n')
		if line != "" {
			p.line(strings.TrimRight(line, "\r\n"))
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			p.res.Text = p.text.String()
			return nil, p.res, err
		}
	}
	p.dispatch() // a last event without a trailing blank line
	p.res.Text = p.text.String()
	return raw.Bytes(), p.res, nil
}

// line handles one line: an SSE field, a blank line (dispatch), a comment, or (vercel-ai) a
// data-stream part such as 0:"text".
func (p *streamParser) line(l string) {
	if l == "" {
		p.dispatch()
		return
	}
	if strings.HasPrefix(l, ":") {
		return // comment
	}
	field, value, found := strings.Cut(l, ":")
	if !found {
		return
	}
	value = strings.TrimPrefix(value, " ")
	switch field {
	case "data":
		p.data = append(p.data, value)
		p.hasData = true
	case "event":
		p.event = value
	case "id":
		p.id = value
	case "retry":
	default:
		if p.preset == StreamPresetVercelAI && len(field) == 1 {
			p.part(field, value)
		}
	}
}

// dispatch completes the current SSE event.
func (p *streamParser) dispatch() {
	if !p.hasData {
		p.event, p.id = "", ""
		return
	}
	payload := strings.Join(p.data, "\n")
	ev := StreamEvent{Event: p.event, ID: p.id, Data: parseData(payload)}
	p.event, p.id, p.data, p.hasData = "", "", nil, false
	p.add(ev, payload)
}

// part is one AI SDK v4 data-stream line: a type code and a JSON value.
func (p *streamParser) part(code, value string) {
	p.add(StreamEvent{Event: code, Data: parseData(value)}, value)
}

func parseData(payload string) any {
	var v any
	if json.Unmarshal([]byte(payload), &v) == nil {
		return v
	}
	return payload
}

func (p *streamParser) add(ev StreamEvent, payload string) {
	p.res.EventCount++
	if len(p.res.Events) < MaxStreamEvents {
		p.res.Events = append(p.res.Events, ev)
	}
	delta, usage := assemble(p.preset, ev, payload)
	if delta != "" {
		if p.res.TTFT == nil {
			d := time.Since(p.start)
			p.res.TTFT = &d
		}
		p.text.WriteString(delta)
	}
	if len(usage) > 0 {
		if p.res.Usage == nil {
			p.res.Usage = map[string]any{}
		}
		for k, v := range usage {
			p.res.Usage[k] = v
		}
	}
}

// assemble is the text an event adds and the usage it reports, by preset.
func assemble(preset string, ev StreamEvent, payload string) (string, map[string]any) {
	if payload == "[DONE]" {
		return "", nil
	}
	m, _ := ev.Data.(map[string]any)
	switch preset {
	case StreamPresetOpenAI:
		usage, _ := m["usage"].(map[string]any)
		if t, _ := m["type"].(string); t != "" {
			// Responses API.
			if r, ok := m["response"].(map[string]any); ok && usage == nil {
				usage, _ = r["usage"].(map[string]any)
			}
			if t == "response.output_text.delta" {
				s, _ := m["delta"].(string)
				return s, usage
			}
			return "", usage
		}
		choices, _ := m["choices"].([]any)
		if len(choices) == 0 {
			return "", usage
		}
		c, _ := choices[0].(map[string]any)
		delta, _ := c["delta"].(map[string]any)
		s, _ := delta["content"].(string)
		return s, usage
	case StreamPresetAnthropic:
		switch m["type"] {
		case "message_start":
			msg, _ := m["message"].(map[string]any)
			usage, _ := msg["usage"].(map[string]any)
			return "", usage
		case "message_delta":
			usage, _ := m["usage"].(map[string]any)
			return "", usage
		case "content_block_delta":
			delta, _ := m["delta"].(map[string]any)
			if delta["type"] == "text_delta" {
				s, _ := delta["text"].(string)
				return s, nil
			}
		}
		return "", nil
	case StreamPresetVercelAI:
		switch ev.Event {
		case "0": // v4 text part: a JSON string
			if s, ok := ev.Data.(string); ok && strings.HasPrefix(payload, `"`) {
				return s, nil
			}
			return "", nil
		case "d", "e": // v4 finish parts
			usage, _ := m["usage"].(map[string]any)
			return "", usage
		}
		if m["type"] == "text-delta" {
			if s, ok := m["delta"].(string); ok {
				return s, nil
			}
			s, _ := m["textDelta"].(string)
			return s, nil
		}
		usage, _ := m["usage"].(map[string]any)
		return "", usage
	default: // raw sse
		return payload, nil
	}
}
