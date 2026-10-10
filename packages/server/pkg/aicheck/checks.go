package aicheck

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// MaxReason caps a reason, in characters.
const MaxReason = 300

// Result is one check in one run: the engine's report steps[].checks[].
type Result struct {
	Kind       string   `json:"kind"`
	Passed     bool     `json:"passed"`
	Score      *float64 `json:"score"`
	Reason     string   `json:"reason"`
	JudgeModel *string  `json:"judge_model"`
	Cached     bool     `json:"cached"`
	Value      *float64 `json:"value"`
	Limit      *float64 `json:"limit"`

	// Skipped: the judge could not judge (no key, provider error). Not a pass or a failure.
	Skipped bool `json:"skipped,omitempty"`
	// Judge accounting: calls made (cache hits: 0) and their tokens.
	Calls        int `json:"judge_calls,omitempty"`
	InputTokens  int `json:"judge_input_tokens,omitempty"`
	OutputTokens int `json:"judge_output_tokens,omitempty"`
}

func ptr(v float64) *float64 { return &v }

// Clip makes a reason one line of at most MaxReason characters.
func Clip(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > MaxReason {
		s = string(r[:MaxReason-1]) + "…"
	}
	return s
}

func fail(kind, reason string) Result { return Result{Kind: kind, Reason: Clip(reason)} }

// toFloat reads a JSON number in any of the shapes the engine or a JSON decoder produce.
func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

// hasResponse: the step got an HTTP response (a status) in this run.
func hasResponse(sr StepRun) bool {
	res, ok := sr.Data["response"].(map[string]any)
	if !ok {
		return false
	}
	status, _ := toFloat(res["status"])
	return status > 0
}

// Deterministic runs the schema, latency, ttft and token checks a spec declares, in that order.
func Deterministic(spec *Spec, step string, run Run) []Result {
	sr := run.Steps[step]
	responded := hasResponse(sr)
	var out []Result
	if spec.Schema != nil {
		out = append(out, checkSchema(spec, step, run, responded))
	}
	if spec.MaxLatencyMS != nil {
		out = append(out, checkLatency(*spec.MaxLatencyMS, sr, responded))
	}
	if spec.MaxTTFTMS != nil {
		out = append(out, checkTTFT(*spec.MaxTTFTMS, step, run, responded))
	}
	if spec.HasTokens() {
		out = append(out, checkTokens(spec, step, run, responded))
	}
	return out
}

func checkSchema(spec *Spec, step string, run Run, responded bool) Result {
	if !responded {
		return fail(KindSchema, "no response")
	}
	v, ok := Resolve(spec.SchemaAt, step, run)
	if !ok {
		return fail(KindSchema, spec.SchemaAt+" not found")
	}
	// The validator wants json.Number numbers: round-trip through its own decoder.
	b, err := json.Marshal(v)
	if err != nil {
		return fail(KindSchema, err.Error())
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		return fail(KindSchema, err.Error())
	}
	if err := spec.Schema.Validate(inst); err != nil {
		return fail(KindSchema, schemaReason(err))
	}
	return Result{Kind: KindSchema, Passed: true}
}

// schemaReason is the first leaf error: "at '/answer': minLength: got 3, want 20".
func schemaReason(err error) string {
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return err.Error()
	}
	for len(ve.Causes) > 0 {
		ve = ve.Causes[0]
	}
	msg := ve.Error()
	if strings.HasPrefix(msg, "at ''") {
		msg = "body" + strings.TrimPrefix(msg, "at ''")
	}
	return msg
}

func formatNum(v float64) string {
	if v == math.Trunc(v) {
		return strconv.FormatFloat(v, 'f', 0, 64)
	}
	return strconv.FormatFloat(v, 'f', 1, 64)
}

// FormatMS is a duration for check lines: 812 ms, 1.8 s.
func FormatMS(v float64) string {
	if v >= 1000 {
		return strings.TrimSuffix(strconv.FormatFloat(v/1000, 'f', 1, 64), ".0") + " s"
	}
	return strconv.FormatFloat(math.Round(v), 'f', 0, 64) + " ms"
}

func checkLatency(limit float64, sr StepRun, responded bool) Result {
	if sr.DurationMS == nil {
		return fail(KindLatency, "no duration")
	}
	return limitResult(KindLatency, *sr.DurationMS, limit, responded)
}

func limitResult(kind string, d, limit float64, responded bool) Result {
	r := Result{Kind: kind, Value: ptr(d), Limit: ptr(limit), Passed: responded && d <= limit}
	switch {
	case !responded:
		r.Reason = "no response"
	case r.Passed:
		r.Reason = fmt.Sprintf("%s ≤ %s", FormatMS(d), FormatMS(limit))
	default:
		r.Reason = fmt.Sprintf("%s > %s", FormatMS(d), FormatMS(limit))
	}
	return r
}

// checkTTFT compares response.ttft_ms (set on streamed responses) with the limit.
func checkTTFT(limit float64, step string, run Run, responded bool) Result {
	if !responded {
		return fail(KindTTFT, "no response")
	}
	v, ok := Resolve("response.ttft_ms", step, run)
	d, isNum := toFloat(v)
	if !ok || !isNum {
		return fail(KindTTFT, "no ttft_ms: the response was not streamed, or no token arrived")
	}
	return limitResult(KindTTFT, d, limit, responded)
}

// usage field names, by provider convention.
var usageNames = map[string][]string{
	"input":  {"input_tokens", "prompt_tokens", "inputTokens", "promptTokens"},
	"output": {"output_tokens", "completion_tokens", "outputTokens", "completionTokens"},
	"total":  {"total_tokens", "totalTokens"},
}

func usageCount(u map[string]any, which string) (float64, bool) {
	for _, k := range usageNames[which] {
		if v, ok := toFloat(u[k]); ok {
			return v, true
		}
	}
	if which == "total" {
		in, ok1 := usageCount(u, "input")
		out, ok2 := usageCount(u, "output")
		if ok1 && ok2 {
			return in + out, true
		}
	}
	return 0, false
}

func checkTokens(spec *Spec, step string, run Run, responded bool) Result {
	if !responded {
		return fail(KindTokens, "no response")
	}
	path := spec.Usage
	v, ok := Resolve(path, step, run)
	u, isMap := v.(map[string]any)
	if (!ok || !isMap) && !spec.UsageSet {
		// A streamed response has its usage in response.usage, not in the body.
		if sv, sok := Resolve(StreamUsage, step, run); sok {
			if su, sIsMap := sv.(map[string]any); sIsMap {
				path, u, ok, isMap = StreamUsage, su, true, true
			}
		}
	}
	if !ok || !isMap {
		return fail(KindTokens, spec.Usage+" not found")
	}
	r := Result{Kind: KindTokens, Passed: true}
	var parts, over []string
	for _, b := range []struct {
		which string
		limit *float64
	}{{"output", spec.MaxOutputTokens}, {"total", spec.MaxTotalTokens}, {"input", spec.MaxInputTokens}} {
		if b.limit == nil {
			continue
		}
		n, ok := usageCount(u, b.which)
		if !ok {
			return fail(KindTokens, fmt.Sprintf("no %s token count in %s", b.which, path))
		}
		if r.Value == nil {
			r.Value, r.Limit = ptr(n), ptr(*b.limit)
		}
		if n > *b.limit {
			r.Passed = false
			over = append(over, fmt.Sprintf("%s %s > %s", b.which, formatNum(n), formatNum(*b.limit)))
		} else {
			parts = append(parts, fmt.Sprintf("%s %s ≤ %s", b.which, formatNum(n), formatNum(*b.limit)))
		}
	}
	if len(over) > 0 {
		r.Reason = strings.Join(over, ", ")
	} else {
		r.Reason = strings.Join(parts, ", ")
	}
	return r
}
