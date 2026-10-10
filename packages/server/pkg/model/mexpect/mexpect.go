// Package mexpect holds a flow file's AI checks as written: the `expect:` block of request and
// graphql steps, and the `judge:` and `quality:` settings at file and flow level. The structs
// carry the YAML keys of the Stresseur AI checks spec; pkg/aicheck validates and evaluates them.
// See docs/specs/AI_CHECKS.md.
package mexpect

import "github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"

// JudgeConfig picks the judge model. A flow's judge overrides the file's key by key.
type JudgeConfig struct {
	Provider  string `yaml:"provider,omitempty"`
	Model     string `yaml:"model,omitempty"`
	BaseURL   string `yaml:"base_url,omitempty"`
	APIKeyEnv string `yaml:"api_key_env,omitempty"`
	// Samples is k for re-judging a borderline score when the provider gives no logprobs.
	Samples int `yaml:"samples,omitempty"`
}

// Quality is the pass-rate gate. FailBelow is kept as written: "90%" or "0.9".
type Quality struct {
	FailBelow string `yaml:"fail_below,omitempty"`
}

// JudgeSpec is one step's G-Eval rubric.
type JudgeSpec struct {
	Criteria string   `yaml:"criteria"`
	Steps    []string `yaml:"steps,omitempty"`
	Input    string   `yaml:"input,omitempty"`
	Output   string   `yaml:"output,omitempty"`
	// MinScore is on the 1–5 scale; 0 means the default, 4.
	MinScore float64 `yaml:"min_score,omitempty"`
}

// Expect is one step's expect: block.
type Expect struct {
	// Schema is an inline JSON Schema (a mapping) or a path relative to the flow file.
	Schema          any        `yaml:"schema,omitempty"`
	SchemaAt        string     `yaml:"schema_at,omitempty"`
	MaxLatencyMS    *float64   `yaml:"max_latency_ms,omitempty"`
	MaxTTFTMS       *float64   `yaml:"max_ttft_ms,omitempty"`
	Usage           string     `yaml:"usage,omitempty"`
	MaxInputTokens  *float64   `yaml:"max_input_tokens,omitempty"`
	MaxOutputTokens *float64   `yaml:"max_output_tokens,omitempty"`
	MaxTotalTokens  *float64   `yaml:"max_total_tokens,omitempty"`
	Judge           *JudgeSpec `yaml:"judge,omitempty"`
}

// FlowSettings are one flow's overrides.
type FlowSettings struct {
	// Iterations is read by `stress ci`; the engine keeps it for the round trip only.
	Iterations *int
	Judge      *JudgeConfig
	Quality    *Quality
}

// Checks are a flow file's AI checks.
type Checks struct {
	Judge   *JudgeConfig
	Quality *Quality
	// Flows are the flow-level settings, by flow ID.
	Flows map[idwrap.IDWrap]FlowSettings
	// Steps are the expect: blocks, by flow node ID.
	Steps map[idwrap.IDWrap]Expect
}

// IsEmpty reports whether the file declares nothing about AI checks.
func (c *Checks) IsEmpty() bool {
	return c == nil || (c.Judge == nil && c.Quality == nil && len(c.Flows) == 0 && len(c.Steps) == 0)
}
