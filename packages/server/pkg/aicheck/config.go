// Package aicheck evaluates a flow's AI checks: the `expect:` block of request and graphql
// steps (a JSON schema, latency, time-to-first-token and token budgets, a G-Eval rubric judge).
// It runs after a flow's steps finish, so checks never add to step or flow timings. Ported from
// the Stresseur CLI's stress/internal/expect with its behavior; see docs/specs/AI_CHECKS.md.
package aicheck

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mexpect"
)

// Check kinds, in the order they are evaluated and printed.
const (
	KindSchema  = "schema"
	KindLatency = "latency"
	KindTTFT    = "ttft"
	KindTokens  = "tokens"
	KindJudge   = "judge"
)

// Kinds is every check kind in order.
var Kinds = []string{KindSchema, KindLatency, KindTTFT, KindTokens, KindJudge}

// Providers.
const (
	ProviderAnthropic = "anthropic"
	ProviderOpenAI    = "openai"
)

// DefaultModels is the judge model when the config names none.
var DefaultModels = map[string]string{
	ProviderAnthropic: "claude-haiku-4-5-20251001",
	ProviderOpenAI:    "gpt-4.1-mini",
}

// DefaultBaseURLs are the provider APIs.
var DefaultBaseURLs = map[string]string{
	ProviderAnthropic: "https://api.anthropic.com",
	ProviderOpenAI:    "https://api.openai.com/v1",
}

// Default paths of an expect: block.
const (
	DefaultSchemaAt = "response.body"
	DefaultUsage    = "response.body.usage"
	// StreamUsage is the usage a streamed response reported; the fallback when usage: is unset.
	StreamUsage = "response.usage"
)

// JudgeConfig is the judge: which model, where, with which key.
type JudgeConfig = mexpect.JudgeConfig

// JudgeSpec is one step's rubric.
type JudgeSpec = mexpect.JudgeSpec

// MergeJudge applies a flow's judge over the file's, key by key.
func MergeJudge(base JudgeConfig, over JudgeConfig) JudgeConfig {
	if over.Provider != "" {
		base.Provider = over.Provider
		if over.Model == "" && base.Model != "" && DefaultModels[over.Provider] != "" {
			base.Model = "" // the file's model belongs to the file's provider
		}
		if over.BaseURL == "" {
			base.BaseURL = ""
		}
	}
	if over.Model != "" {
		base.Model = over.Model
	}
	if over.BaseURL != "" {
		base.BaseURL = over.BaseURL
	}
	if over.APIKeyEnv != "" {
		base.APIKeyEnv = over.APIKeyEnv
	}
	if over.Samples != 0 {
		base.Samples = over.Samples
	}
	return base
}

// FoldFileSettings is a flow's settings with the file's judge: and quality: folded in, as a
// workspace keeps them per flow: the judge merged key by key, the flow's quality: over the
// file's. ok is false when there is nothing to keep.
func FoldFileSettings(c *mexpect.Checks, flowID idwrap.IDWrap) (mexpect.FlowSettings, bool) {
	if c == nil {
		return mexpect.FlowSettings{}, false
	}
	s := c.Flows[flowID]
	if c.Judge != nil || s.Judge != nil {
		var base, over JudgeConfig
		if c.Judge != nil {
			base = *c.Judge
		}
		if s.Judge != nil {
			over = *s.Judge
		}
		j := MergeJudge(base, over)
		s.Judge = &j
	}
	if (s.Quality == nil || strings.TrimSpace(s.Quality.FailBelow) == "") && c.Quality != nil {
		q := *c.Quality
		s.Quality = &q
	}
	return s, s.Iterations != nil || s.Judge != nil || s.Quality != nil
}

// JudgeDefaults fills the provider's defaults.
func JudgeDefaults(c JudgeConfig) JudgeConfig {
	if c.Provider == "" {
		c.Provider = ProviderAnthropic
	}
	if c.Model == "" {
		c.Model = DefaultModels[c.Provider]
	}
	if c.BaseURL == "" {
		c.BaseURL = DefaultBaseURLs[c.Provider]
	}
	c.BaseURL = strings.TrimRight(c.BaseURL, "/")
	if c.Samples == 0 {
		c.Samples = 3
	}
	return c
}

// ValidateJudge checks a judge: block.
func ValidateJudge(c JudgeConfig) error {
	if c.Provider != "" && DefaultModels[c.Provider] == "" {
		return fmt.Errorf("provider must be anthropic or openai (got %q)", c.Provider)
	}
	if c.Samples < 0 || c.Samples > 10 {
		return fmt.Errorf("samples must be 1 to 10 (got %d)", c.Samples)
	}
	return nil
}

// Quality is the pass-rate gate.
type Quality struct {
	// FailBelow (0–1): a check passing in a smaller share of runs fails the command. Nil: report only.
	FailBelow *float64
}

// ParseRate reads 90%, "90%" or 0.9 as 0.9.
func ParseRate(s string) (float64, error) {
	s = strings.TrimSpace(s)
	pct := strings.HasSuffix(s, "%")
	v, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(s, "%")), 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not a rate (write 90%% or 0.9)", s)
	}
	if pct {
		v /= 100
	}
	if v < 0 || v > 1 {
		return 0, fmt.Errorf("%q is not between 0%% and 100%%", s)
	}
	return v, nil
}

// ParseQuality reads a quality: block; nil is no gate.
func ParseQuality(q *mexpect.Quality) (Quality, error) {
	var out Quality
	if q == nil || strings.TrimSpace(q.FailBelow) == "" {
		return out, nil
	}
	v, err := ParseRate(q.FailBelow)
	if err != nil {
		return out, fmt.Errorf("fail_below: %w", err)
	}
	out.FailBelow = &v
	return out, nil
}

// Spec is one step's compiled expect: block.
type Spec struct {
	Schema       *jsonschema.Schema
	SchemaAt     string
	MaxLatencyMS *float64
	MaxTTFTMS    *float64
	Usage        string
	// UsageSet: the step names its usage path, so there is no fallback to the stream's.
	UsageSet        bool
	MaxInputTokens  *float64
	MaxOutputTokens *float64
	MaxTotalTokens  *float64
	Judge           *JudgeSpec
}

// HasTokens says whether any token budget is set.
func (s *Spec) HasTokens() bool {
	return s.MaxInputTokens != nil || s.MaxOutputTokens != nil || s.MaxTotalTokens != nil
}

func nonNegative(name string, v *float64) error {
	if v != nil && *v < 0 {
		return fmt.Errorf("%s must be at least 0 (got %v)", name, *v)
	}
	return nil
}

// CompileSpec validates an expect: block and compiles its schema. dir resolves a schema path;
// with dir "" a schema path is checked only for being a string, not read.
func CompileSpec(e mexpect.Expect, dir string) (*Spec, error) {
	spec := &Spec{SchemaAt: e.SchemaAt, MaxLatencyMS: e.MaxLatencyMS, MaxTTFTMS: e.MaxTTFTMS, Usage: e.Usage,
		UsageSet: e.Usage != "", MaxInputTokens: e.MaxInputTokens, MaxOutputTokens: e.MaxOutputTokens,
		MaxTotalTokens: e.MaxTotalTokens}
	for _, b := range []struct {
		name string
		v    *float64
	}{{"max_latency_ms", e.MaxLatencyMS}, {"max_ttft_ms", e.MaxTTFTMS}, {"max_input_tokens", e.MaxInputTokens},
		{"max_output_tokens", e.MaxOutputTokens}, {"max_total_tokens", e.MaxTotalTokens}} {
		if err := nonNegative(b.name, b.v); err != nil {
			return nil, err
		}
	}
	if spec.SchemaAt == "" {
		spec.SchemaAt = DefaultSchemaAt
	}
	if spec.Usage == "" {
		spec.Usage = DefaultUsage
	}
	hasSchema := e.Schema != nil
	if hasSchema {
		sch, err := compileSchema(e.Schema, dir)
		if err != nil {
			return nil, fmt.Errorf("schema: %w", err)
		}
		spec.Schema = sch
	} else if e.SchemaAt != "" {
		return nil, errors.New("schema_at needs a schema")
	}
	if e.Judge != nil {
		j := *e.Judge
		if strings.TrimSpace(j.Criteria) == "" {
			return nil, errors.New("judge: criteria is required")
		}
		if j.MinScore == 0 {
			j.MinScore = 4
		}
		if j.MinScore < 1 || j.MinScore > 5 {
			return nil, fmt.Errorf("judge: min_score is on the 1–5 scale (got %v)", j.MinScore)
		}
		spec.Judge = &j
	}
	if !hasSchema && spec.MaxLatencyMS == nil && spec.MaxTTFTMS == nil && !spec.HasTokens() && spec.Judge == nil {
		return nil, errors.New("no check: add schema, max_latency_ms, max_ttft_ms, a max_*_tokens budget or judge")
	}
	return spec, nil
}

// compileSchema compiles an inline schema (a mapping) or a schema file (a path, relative to
// dir). With dir "" a path is not read: the schema is compiled when the flow runs.
func compileSchema(schema any, dir string) (*jsonschema.Schema, error) {
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	if p, isPath := schema.(string); isPath {
		if strings.TrimSpace(p) == "" {
			return nil, errors.New("empty schema path")
		}
		if dir == "" && !filepath.IsAbs(p) {
			return nil, nil
		}
		path := p
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, path)
		}
		b, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return nil, fmt.Errorf("could not read %s", p)
		}
		var doc any
		if strings.HasSuffix(path, ".json") {
			doc, err = jsonschema.UnmarshalJSON(bytes.NewReader(b))
		} else {
			err = yaml.Unmarshal(b, &doc)
			doc = normalize(doc)
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		if err := c.AddResource("file:///schema.json", doc); err != nil {
			return nil, err
		}
	} else if err := c.AddResource("file:///schema.json", normalize(deepCopy(schema))); err != nil {
		return nil, err
	}
	sch, err := c.Compile("file:///schema.json")
	if err != nil {
		msg := err.Error()
		if i := strings.Index(msg, "\n"); i > 0 {
			msg = msg[:i]
		}
		return nil, errors.New(msg)
	}
	return sch, nil
}

// deepCopy copies maps and slices, so normalizing a schema never changes the model it came from.
func deepCopy(v any) any {
	switch t := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(t))
		for k, x := range t {
			m[k] = deepCopy(x)
		}
		return m
	case map[any]any:
		m := make(map[any]any, len(t))
		for k, x := range t {
			m[k] = deepCopy(x)
		}
		return m
	case []any:
		s := make([]any, len(t))
		for i, x := range t {
			s[i] = deepCopy(x)
		}
		return s
	}
	return v
}

// normalize turns YAML-decoded values into JSON-shaped ones (map[string]any, float64 numbers).
func normalize(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, x := range t {
			t[k] = normalize(x)
		}
		return t
	case map[any]any:
		m := map[string]any{}
		for k, x := range t {
			m[fmt.Sprint(k)] = normalize(x)
		}
		return m
	case []any:
		for i, x := range t {
			t[i] = normalize(x)
		}
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case uint64:
		return float64(t)
	}
	return v
}

// StrictDecode decodes a YAML node into out, rejecting unknown keys.
func StrictDecode(n *yaml.Node, out any) error {
	b, err := yaml.Marshal(n)
	if err != nil {
		return err
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil {
		return errors.New(strings.TrimPrefix(err.Error(), "yaml: unmarshal errors:\n  "))
	}
	return nil
}

// Flow is one flow's checks.
type Flow struct {
	Name string
	// Iterations is the flow's iterations: (for `stress ci`); 0 when unset.
	Iterations int
	Judge      JudgeConfig // merged and defaulted
	Quality    Quality     // merged
	// Steps are the steps with an expect: block, in file order.
	Steps []string
	Specs map[string]*Spec
}

// NewFlow resolves a flow's judge and quality from the file's and the flow's settings.
func NewFlow(name string, file *mexpect.Checks, flow mexpect.FlowSettings) (*Flow, error) {
	fl := &Flow{Name: name, Specs: map[string]*Spec{}}
	var fileJudge, flowJudge JudgeConfig
	var fileQuality *mexpect.Quality
	if file != nil {
		if file.Judge != nil {
			fileJudge = *file.Judge
		}
		fileQuality = file.Quality
	}
	if err := ValidateJudge(fileJudge); err != nil {
		return nil, fmt.Errorf("file judge: %w", err)
	}
	if flow.Judge != nil {
		flowJudge = *flow.Judge
	}
	if err := ValidateJudge(flowJudge); err != nil {
		return nil, fmt.Errorf("flow %s judge: %w", name, err)
	}
	fl.Judge = JudgeDefaults(MergeJudge(fileJudge, flowJudge))
	q, err := ParseQuality(fileQuality)
	if err != nil {
		return nil, fmt.Errorf("file quality.%w", err)
	}
	fl.Quality = q
	fq, err := ParseQuality(flow.Quality)
	if err != nil {
		return nil, fmt.Errorf("flow %s quality.%w", name, err)
	}
	if fq.FailBelow != nil {
		fl.Quality = fq
	}
	if flow.Iterations != nil {
		if *flow.Iterations < 1 {
			return nil, fmt.Errorf("flow %s: iterations must be a whole number of at least 1 (got %d)", name, *flow.Iterations)
		}
		fl.Iterations = *flow.Iterations
	}
	return fl, nil
}

// Add compiles a step's expect: block into the flow.
func (fl *Flow) Add(step string, e mexpect.Expect, dir string) error {
	spec, err := CompileSpec(e, dir)
	if err != nil {
		return fmt.Errorf("flow %s, step %s: expect: %w", fl.Name, step, err)
	}
	fl.Steps = append(fl.Steps, step)
	fl.Specs[step] = spec
	return nil
}

// File is a flow file's checks, by flow name.
type File struct {
	Flows map[string]*Flow
}

// HasChecks says whether any flow has a step with expect:.
func (f File) HasChecks() bool {
	for _, fl := range f.Flows {
		if len(fl.Steps) > 0 {
			return true
		}
	}
	return false
}

// Flow returns the named flow's checks, nil when it has none.
func (f File) Flow(name string) *Flow {
	if fl := f.Flows[name]; fl != nil && len(fl.Steps) > 0 {
		return fl
	}
	return nil
}

type rawFile struct {
	Judge   yaml.Node `yaml:"judge"`
	Quality yaml.Node `yaml:"quality"`
	Flows   []struct {
		Name       string               `yaml:"name"`
		Iterations yaml.Node            `yaml:"iterations"`
		Judge      yaml.Node            `yaml:"judge"`
		Quality    yaml.Node            `yaml:"quality"`
		Steps      []map[string]rawStep `yaml:"steps"`
		Cleanup    []map[string]rawStep `yaml:"cleanup"`
	} `yaml:"flows"`
}

type rawStep struct {
	Name   string    `yaml:"name"`
	Expect yaml.Node `yaml:"expect"`
}

// Present says whether a YAML node holds a value (not absent, not null).
func Present(n yaml.Node) bool {
	return n.Kind != 0 && (n.Kind != yaml.ScalarNode || n.Tag != "!!null")
}

// DecodeJudge strictly decodes and validates a judge: block.
func DecodeJudge(n *yaml.Node) (*JudgeConfig, error) {
	var c JudgeConfig
	if err := StrictDecode(n, &c); err != nil {
		return nil, err
	}
	if err := ValidateJudge(c); err != nil {
		return nil, err
	}
	return &c, nil
}

// DecodeQuality strictly decodes and validates a quality: block.
func DecodeQuality(n *yaml.Node) (*mexpect.Quality, error) {
	var q mexpect.Quality
	if err := StrictDecode(n, &q); err != nil {
		return nil, err
	}
	if _, err := ParseQuality(&q); err != nil {
		return nil, err
	}
	return &q, nil
}

// DecodeExpect strictly decodes an expect: block. It is validated by CompileSpec.
func DecodeExpect(n *yaml.Node) (mexpect.Expect, error) {
	var e mexpect.Expect
	err := StrictDecode(n, &e)
	return e, err
}

// Parse reads the checks of a flow file on its own (dir resolves schema file paths). A file
// without any expect: block parses to a File without checks; a malformed one is an error
// naming the step.
func Parse(content []byte, dir string) (File, error) {
	out := File{Flows: map[string]*Flow{}}
	var doc rawFile
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return out, err
	}
	file := &mexpect.Checks{}
	if Present(doc.Judge) {
		j, err := DecodeJudge(&doc.Judge)
		if err != nil {
			return out, fmt.Errorf("file judge: %w", err)
		}
		file.Judge = j
	}
	if Present(doc.Quality) {
		q, err := DecodeQuality(&doc.Quality)
		if err != nil {
			return out, fmt.Errorf("file quality: %w", err)
		}
		file.Quality = q
	}
	for _, f := range doc.Flows {
		if f.Name == "" {
			continue
		}
		var settings mexpect.FlowSettings
		if Present(f.Iterations) {
			n, err := strconv.Atoi(f.Iterations.Value)
			if err != nil {
				return out, fmt.Errorf("flow %s: iterations must be a whole number of at least 1 (got %q)", f.Name, f.Iterations.Value)
			}
			settings.Iterations = &n
		}
		if Present(f.Judge) {
			j, err := DecodeJudge(&f.Judge)
			if err != nil {
				return out, fmt.Errorf("flow %s judge: %w", f.Name, err)
			}
			settings.Judge = j
		}
		if Present(f.Quality) {
			q, err := DecodeQuality(&f.Quality)
			if err != nil {
				return out, fmt.Errorf("flow %s quality: %w", f.Name, err)
			}
			settings.Quality = q
		}
		fl, err := NewFlow(f.Name, file, settings)
		if err != nil {
			return out, err
		}
		for _, list := range [][]map[string]rawStep{f.Steps, f.Cleanup} {
			for _, wrapper := range list {
				for kind, body := range wrapper {
					if !Present(body.Expect) {
						continue
					}
					if kind != "request" && kind != "graphql" {
						return out, fmt.Errorf("flow %s, step %s: expect: works on request and graphql steps only", f.Name, body.Name)
					}
					e, err := DecodeExpect(&body.Expect)
					if err != nil {
						return out, fmt.Errorf("flow %s, step %s: expect: %w", f.Name, body.Name, err)
					}
					if err := fl.Add(body.Name, e, dir); err != nil {
						return out, err
					}
				}
			}
		}
		out.Flows[f.Name] = fl
	}
	return out, nil
}
