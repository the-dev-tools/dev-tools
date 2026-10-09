package yamlflowsimplev2

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// YamlLoadStage is one leg of a ramping profile:
//
//	stages:
//	  - { duration: 30s, target: 20 }
//
// For ramping-vus the target is a VU count; for ramping-arrival-rate it is a
// rate per time_unit.
type YamlLoadStage struct {
	Duration string  `yaml:"duration"`
	Target   float64 `yaml:"target"`
}

// YamlThinkTime is the pause after each iteration. It is written either as a
// single duration (`think_time: 1s`) or as a uniform range
// (`think_time: { min: 1s, max: 3s }`).
type YamlThinkTime struct {
	Min string `yaml:"min"`
	Max string `yaml:"max"`
}

// UnmarshalYAML accepts both the scalar and the range form.
func (t *YamlThinkTime) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		t.Min, t.Max = value.Value, value.Value
		return nil
	case yaml.MappingNode:
		type plain YamlThinkTime
		var decoded plain
		if err := value.Decode(&decoded); err != nil {
			return err
		}
		*t = YamlThinkTime(decoded)
		return nil
	default:
		return fmt.Errorf("line %d: think_time must be a duration or { min: <duration>, max: <duration> }", value.Line)
	}
}

// MarshalYAML writes a fixed think time back as a single duration.
func (t YamlThinkTime) MarshalYAML() (any, error) {
	if t.Min == t.Max {
		return t.Min, nil
	}
	type plain YamlThinkTime
	return plain(t), nil
}

// YamlThresholdEntry is one threshold as written. List-form entries carry the
// whole expression in Expr ("p95(PostLogin)<500ms"); map-form entries carry
// the metric and optional step from the keys, and the comparison ("<500ms")
// from the value.
type YamlThresholdEntry struct {
	Expr       string
	Step       string
	Metric     string
	Comparison string
	// Line is the source line, for error messages. Zero when unknown.
	Line int
}

// YamlLoadThresholds holds a scenario's thresholds. Two spellings are
// accepted. The map form:
//
//	thresholds:
//	  p95: <300ms
//	  errors: <1%
//	  steps:
//	    PostLogin:
//	      p95: <500ms
//
// and the list form, one expression per entry:
//
//	thresholds:
//	  - p95<300ms
//	  - p95(PostLogin)<500ms
//	  - errors<1%
//
// Export always uses the map form, unless two thresholds share a metric and
// step, which only the list form can express.
type YamlLoadThresholds struct {
	Entries []YamlThresholdEntry
}

// thresholdStepsKey is the map-form key that introduces per-step thresholds.
const thresholdStepsKey = "steps"

// UnmarshalYAML accepts both the map and the list form.
func (th *YamlLoadThresholds) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.SequenceNode:
		for _, item := range value.Content {
			if item.Kind != yaml.ScalarNode {
				return fmt.Errorf("line %d: each list-form threshold must be an expression such as p95<300ms", item.Line)
			}
			th.Entries = append(th.Entries, YamlThresholdEntry{Expr: item.Value, Line: item.Line})
		}
		return nil

	case yaml.MappingNode:
		for i := 0; i+1 < len(value.Content); i += 2 {
			key, val := value.Content[i], value.Content[i+1]
			if key.Value != thresholdStepsKey {
				if val.Kind != yaml.ScalarNode {
					return fmt.Errorf("line %d: threshold %q must be a comparison such as <300ms", val.Line, key.Value)
				}
				th.Entries = append(th.Entries, YamlThresholdEntry{Metric: key.Value, Comparison: val.Value, Line: val.Line})
				continue
			}
			if val.Kind != yaml.MappingNode {
				return fmt.Errorf("line %d: thresholds.steps must map step names to their thresholds", val.Line)
			}
			for j := 0; j+1 < len(val.Content); j += 2 {
				step, metrics := val.Content[j], val.Content[j+1]
				if metrics.Kind != yaml.MappingNode {
					return fmt.Errorf("line %d: thresholds for step %q must map metrics to comparisons, e.g. p95: <500ms", metrics.Line, step.Value)
				}
				for k := 0; k+1 < len(metrics.Content); k += 2 {
					metric, comparison := metrics.Content[k], metrics.Content[k+1]
					if comparison.Kind != yaml.ScalarNode {
						return fmt.Errorf("line %d: threshold %s for step %q must be a comparison such as <500ms", comparison.Line, metric.Value, step.Value)
					}
					th.Entries = append(th.Entries, YamlThresholdEntry{
						Step: step.Value, Metric: metric.Value, Comparison: comparison.Value, Line: comparison.Line,
					})
				}
			}
		}
		return nil

	default:
		return fmt.Errorf("line %d: thresholds must be a map (p95: <300ms) or a list of expressions (- p95<300ms)", value.Line)
	}
}

// MarshalYAML writes the map form, in entry order, falling back to the list
// form when a metric repeats for the same scope. Entries are expected to be
// map-form (Metric/Comparison set), which is what the exporter produces.
func (th YamlLoadThresholds) MarshalYAML() (any, error) {
	type scope struct{ step, metric string }
	seen := make(map[scope]bool, len(th.Entries))
	listForm := false
	for _, e := range th.Entries {
		key := scope{e.Step, e.Metric}
		if seen[key] || e.Expr != "" {
			listForm = true
			break
		}
		seen[key] = true
	}

	if listForm {
		list := &yaml.Node{Kind: yaml.SequenceNode}
		for _, e := range th.Entries {
			list.Content = append(list.Content, scalarNode(e.expression()))
		}
		return list, nil
	}

	root := &yaml.Node{Kind: yaml.MappingNode}
	var steps *yaml.Node
	stepNodes := make(map[string]*yaml.Node)
	for _, e := range th.Entries {
		if e.Step == "" {
			root.Content = append(root.Content, scalarNode(e.Metric), scalarNode(e.Comparison))
			continue
		}
		if steps == nil {
			steps = &yaml.Node{Kind: yaml.MappingNode}
		}
		node, ok := stepNodes[e.Step]
		if !ok {
			node = &yaml.Node{Kind: yaml.MappingNode}
			stepNodes[e.Step] = node
			steps.Content = append(steps.Content, scalarNode(e.Step), node)
		}
		node.Content = append(node.Content, scalarNode(e.Metric), scalarNode(e.Comparison))
	}
	if steps != nil {
		root.Content = append(root.Content, scalarNode(thresholdStepsKey), steps)
	}
	return root, nil
}

// expression renders the entry as a single list-form expression.
func (e YamlThresholdEntry) expression() string {
	switch {
	case e.Expr != "":
		return e.Expr
	case e.Step != "":
		return fmt.Sprintf("%s(%s)%s", e.Metric, e.Step, e.Comparison)
	default:
		return e.Metric + e.Comparison
	}
}

func scalarNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

// YamlAbortRule stops a load run early once its condition holds over the
// trailing window. The shorthand `- errors>20%` is a rule with the default
// window and no delay.
type YamlAbortRule struct {
	When   string `yaml:"when"`
	Window string `yaml:"window,omitempty"`
	Delay  string `yaml:"delay,omitempty"`
}

// UnmarshalYAML accepts the shorthand scalar form as well as the mapping.
func (r *YamlAbortRule) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		r.When = value.Value
		return nil
	case yaml.MappingNode:
		type plain YamlAbortRule
		var decoded plain
		if err := value.Decode(&decoded); err != nil {
			return err
		}
		*r = YamlAbortRule(decoded)
		return nil
	default:
		return fmt.Errorf("line %d: an abort rule is an expression (- errors>20%%) or { when, window, delay }", value.Line)
	}
}

// MarshalYAML writes a rule with neither window nor delay in its shorthand
// form.
func (r YamlAbortRule) MarshalYAML() (any, error) {
	if r.Window == "" && r.Delay == "" {
		return r.When, nil
	}
	type plain YamlAbortRule
	return plain(r), nil
}
