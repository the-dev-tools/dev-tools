package yamlflowsimplev2

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mload"
)

// convertLoadScenarios validates the `load:` block and decodes it into the
// engine-ready domain model, preserving declaration order.
func convertLoadScenarios(yamlFormat *YamlFlowFormatV2) ([]mload.Scenario, error) {
	flowNames := make([]string, 0, len(yamlFormat.Flows))
	for _, flow := range yamlFormat.Flows {
		flowNames = append(flowNames, flow.Name)
	}
	return convertLoadEntries(yamlFormat.Load, flowNames)
}

// ParseLoadScenarios decodes load scenarios kept outside the flow file - for
// example a stresseur.load.yaml beside the api tests. The document is either
// a mapping with a `load:` key, exactly like the block in a flow file, or a
// bare list of the same entries. Entries are validated exactly as the inline
// block is; flowNames are the flows they may reference.
func ParseLoadScenarios(data []byte, flowNames []string) ([]mload.Scenario, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("failed to parse load file: %w", err)
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return nil, errors.New("load file is empty: expected a load: block or a list of load scenarios")
	}

	var entries []YamlLoadScenario
	switch doc := root.Content[0]; doc.Kind {
	case yaml.SequenceNode:
		if err := doc.Decode(&entries); err != nil {
			return nil, fmt.Errorf("failed to parse load file: %w", err)
		}
	case yaml.MappingNode:
		var wrapper struct {
			Load []YamlLoadScenario `yaml:"load"`
		}
		if err := doc.Decode(&wrapper); err != nil {
			return nil, fmt.Errorf("failed to parse load file: %w", err)
		}
		if wrapper.Load == nil {
			return nil, errors.New("load file has no load: block: expected a load: block or a list of load scenarios")
		}
		entries = wrapper.Load
	default:
		return nil, errors.New("load file must be a load: block or a list of load scenarios")
	}

	return convertLoadEntries(entries, flowNames)
}

// convertLoadEntries validates load entries against the flows they may
// reference.
//
// Every error names the offending scenario and, where there is a closed set of
// legal values, spells that set out - a load block is usually written by hand
// (or by an agent) and a nameless "invalid executor" is useless to both.
func convertLoadEntries(entries []YamlLoadScenario, flowNames []string) ([]mload.Scenario, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	knownFlows := make(map[string]bool, len(flowNames))
	for _, name := range flowNames {
		knownFlows[name] = true
	}

	scenarios := make([]mload.Scenario, 0, len(entries))
	seen := make(map[string]bool, len(entries))

	for i, entry := range entries {
		if entry.Name == "" {
			return nil, NewYamlFlowErrorWithLineV2("load scenario name is required", "load.name", nil, i)
		}
		if seen[entry.Name] {
			return nil, NewYamlFlowErrorV2(
				fmt.Sprintf("duplicate load scenario name: %s", entry.Name), "load.name", entry.Name)
		}
		seen[entry.Name] = true

		scenario, err := convertLoadScenario(entry, knownFlows, flowNames)
		if err != nil {
			return nil, err
		}
		scenarios = append(scenarios, scenario)
	}

	return scenarios, nil
}

// executorFields lists, per executor, the executor-specific keys it accepts.
// Keys shared by every executor (name, flow, executor, think_time, thresholds,
// abort) are not listed.
var executorFields = map[mload.Executor][]string{
	mload.ExecutorConstantVUs:         {"vus", "duration", "iterations"},
	mload.ExecutorRampingVUs:          {"start_vus", "stages", "graceful_ramp_down", "graceful_stop"},
	mload.ExecutorConstantArrivalRate: {"rate", "time_unit", "duration", "pre_allocated_vus", "max_vus", "graceful_stop"},
	mload.ExecutorRampingArrivalRate:  {"start_rate", "time_unit", "stages", "pre_allocated_vus", "max_vus", "graceful_stop"},
}

// setFields lists the executor-specific keys an entry sets, in a fixed order.
func setFields(entry YamlLoadScenario) []string {
	var set []string
	add := func(name string, isSet bool) {
		if isSet {
			set = append(set, name)
		}
	}
	add("vus", entry.VUs != 0)
	add("duration", entry.Duration != "")
	add("iterations", entry.Iterations != 0)
	add("start_vus", entry.StartVUs != 0)
	add("stages", len(entry.Stages) > 0)
	add("graceful_ramp_down", entry.GracefulRampDown != "")
	add("rate", entry.Rate != 0)
	add("start_rate", entry.StartRate != 0)
	add("time_unit", entry.TimeUnit != "")
	add("pre_allocated_vus", entry.PreAllocatedVUs != 0)
	add("max_vus", entry.MaxVUs != 0)
	add("graceful_stop", entry.GracefulStop != "")
	return set
}

// executorsAccepting names the executors that accept field, for errors.
func executorsAccepting(field string) string {
	var names []string
	for _, executor := range mload.SupportedExecutors {
		if slices.Contains(executorFields[executor], field) {
			names = append(names, string(executor))
		}
	}
	return strings.Join(names, ", ")
}

func convertLoadScenario(entry YamlLoadScenario, knownFlows map[string]bool, flowNames []string) (mload.Scenario, error) {
	fail := func(format string, args ...any) (mload.Scenario, error) {
		return mload.Scenario{}, NewYamlFlowErrorV2(
			fmt.Sprintf("load scenario %q: ", entry.Name)+fmt.Sprintf(format, args...),
			"load", entry.Name)
	}

	if entry.Flow == "" {
		return fail("flow is required (known flows: %s)", strings.Join(flowNames, ", "))
	}
	if !knownFlows[entry.Flow] {
		return fail("references unknown flow %q (known flows: %s)", entry.Flow, strings.Join(flowNames, ", "))
	}

	executor := mload.Executor(entry.Executor)
	if entry.Executor == "" {
		executor = mload.ExecutorConstantVUs
	}
	allowed, ok := executorFields[executor]
	if !ok {
		return fail("unsupported executor %q (this build supports: %s)",
			entry.Executor, joinExecutors(mload.SupportedExecutors))
	}
	for _, field := range setFields(entry) {
		if !slices.Contains(allowed, field) {
			hint := ""
			if field == "vus" && executor.IsArrivalRate() {
				hint = "; arrival-rate executors size their VU pool with pre_allocated_vus and max_vus"
			}
			return fail("%s does not apply to the %s executor (it applies to: %s)%s",
				field, executor, executorsAccepting(field), hint)
		}
	}

	scenario := mload.Scenario{
		Name:     entry.Name,
		FlowName: entry.Flow,
		Executor: executor,
	}

	var err error
	parse := func(field, value string) time.Duration {
		if err != nil || value == "" {
			return 0
		}
		parsed, perr := time.ParseDuration(value)
		switch {
		case perr != nil:
			err = fmt.Errorf("%s %q is not a valid Go duration (e.g. 30s, 2m, 1h30m)", field, value)
		case parsed <= 0:
			err = fmt.Errorf("%s %q must be positive", field, value)
		}
		return parsed
	}
	scenario.Duration = parse("duration", entry.Duration)
	scenario.GracefulRampDown = parse("graceful_ramp_down", entry.GracefulRampDown)
	scenario.GracefulStop = parse("graceful_stop", entry.GracefulStop)
	scenario.TimeUnit = parse("time_unit", entry.TimeUnit)
	if err != nil {
		return fail("%v", err)
	}

	switch executor {
	case mload.ExecutorConstantVUs:
		if entry.VUs < 1 {
			return fail("vus must be >= 1, got %d", entry.VUs)
		}
		if entry.Iterations < 0 {
			return fail("iterations must be >= 0, got %d", entry.Iterations)
		}
		if scenario.Duration == 0 && entry.Iterations == 0 {
			return fail("needs a stop condition: set duration, iterations, or both")
		}
		scenario.VUs = entry.VUs
		scenario.MaxIterations = entry.Iterations

	case mload.ExecutorRampingVUs:
		if entry.StartVUs < 0 {
			return fail("start_vus must be >= 0, got %d", entry.StartVUs)
		}
		stages, serr := convertStages(executor, entry.Stages)
		if serr != nil {
			return fail("%v", serr)
		}
		highest := entry.StartVUs
		for _, s := range stages {
			highest = max(highest, int(s.Target))
		}
		if highest < 1 {
			return fail("ramping-vus never reaches a VU: start_vus or a stage target must be at least one VU")
		}
		scenario.StartVUs = entry.StartVUs
		scenario.Stages = stages

	case mload.ExecutorConstantArrivalRate:
		if entry.Rate <= 0 {
			return fail("rate must be > 0 iterations per time_unit, got %v", entry.Rate)
		}
		if scenario.Duration == 0 {
			return fail("duration is required for constant-arrival-rate")
		}
		if perr := checkPool(entry); perr != nil {
			return fail("%v", perr)
		}
		scenario.Rate = entry.Rate
		scenario.PreAllocatedVUs = entry.PreAllocatedVUs
		scenario.MaxVUs = entry.MaxVUs

	case mload.ExecutorRampingArrivalRate:
		if entry.StartRate < 0 {
			return fail("start_rate must be >= 0, got %v", entry.StartRate)
		}
		stages, serr := convertStages(executor, entry.Stages)
		if serr != nil {
			return fail("%v", serr)
		}
		if perr := checkPool(entry); perr != nil {
			return fail("%v", perr)
		}
		scenario.StartRate = entry.StartRate
		scenario.Stages = stages
		scenario.PreAllocatedVUs = entry.PreAllocatedVUs
		scenario.MaxVUs = entry.MaxVUs
	}

	if entry.ThinkTime != nil {
		think, terr := convertThinkTime(*entry.ThinkTime)
		if terr != nil {
			return fail("%v", terr)
		}
		scenario.ThinkTime = think
	}

	if entry.Thresholds != nil {
		thresholds, terr := convertThresholds(*entry.Thresholds)
		if terr != nil {
			return fail("%v", terr)
		}
		scenario.Thresholds = thresholds
	}

	for i, rule := range entry.Abort {
		converted, aerr := convertAbortRule(rule)
		if aerr != nil {
			return fail("abort rule %d: %v", i+1, aerr)
		}
		scenario.Abort = append(scenario.Abort, converted)
	}

	return scenario, nil
}

func checkPool(entry YamlLoadScenario) error {
	if entry.PreAllocatedVUs < 1 {
		return fmt.Errorf("pre_allocated_vus must be >= 1, got %d", entry.PreAllocatedVUs)
	}
	if entry.MaxVUs != 0 && entry.MaxVUs < entry.PreAllocatedVUs {
		return fmt.Errorf("max_vus (%d) must be >= pre_allocated_vus (%d)", entry.MaxVUs, entry.PreAllocatedVUs)
	}
	return nil
}

// convertStages validates a stage list. VU stages need whole-number targets.
func convertStages(executor mload.Executor, entries []YamlLoadStage) ([]mload.Stage, error) {
	wholeTargets := executor == mload.ExecutorRampingVUs
	if len(entries) == 0 {
		return nil, fmt.Errorf("stages are required for %s, e.g. stages: [{ duration: 30s, target: 10 }]", executor)
	}
	stages := make([]mload.Stage, 0, len(entries))
	var total time.Duration
	for i, entry := range entries {
		d, err := time.ParseDuration(entry.Duration)
		if err != nil {
			return nil, fmt.Errorf("stage %d: duration %q is not a valid Go duration (e.g. 30s, 2m)", i+1, entry.Duration)
		}
		if d < 0 {
			return nil, fmt.Errorf("stage %d: duration %q must not be negative", i+1, entry.Duration)
		}
		if entry.Target < 0 {
			return nil, fmt.Errorf("stage %d: target %v must not be negative", i+1, entry.Target)
		}
		if wholeTargets && entry.Target != math.Trunc(entry.Target) {
			return nil, fmt.Errorf("stage %d: target %v must be a whole number of VUs", i+1, entry.Target)
		}
		total += d
		stages = append(stages, mload.Stage{Duration: d, Target: entry.Target})
	}
	if total <= 0 {
		return nil, errors.New("stages must add up to a positive duration")
	}
	return stages, nil
}

func convertThinkTime(entry YamlThinkTime) (mload.ThinkTime, error) {
	parse := func(value string) (time.Duration, error) {
		d, err := time.ParseDuration(value)
		if err != nil {
			return 0, fmt.Errorf("think_time %q is not a valid Go duration (e.g. 1s, 500ms)", value)
		}
		if d < 0 {
			return 0, fmt.Errorf("think_time %q must not be negative", value)
		}
		return d, nil
	}
	if entry.Min == "" || entry.Max == "" {
		return mload.ThinkTime{}, errors.New("think_time is a duration, or a range with both min and max")
	}
	lo, err := parse(entry.Min)
	if err != nil {
		return mload.ThinkTime{}, err
	}
	hi, err := parse(entry.Max)
	if err != nil {
		return mload.ThinkTime{}, err
	}
	if lo > hi {
		return mload.ThinkTime{}, fmt.Errorf("think_time min (%s) must not exceed max (%s)", entry.Min, entry.Max)
	}
	return mload.ThinkTime{Min: lo, Max: hi}, nil
}

// convertThresholds parses both threshold spellings and returns them in
// canonical order: run-wide thresholds first, then per step by step name,
// each in metric order. Declaration order breaks ties, so a document that
// repeats a metric keeps its own order.
func convertThresholds(th YamlLoadThresholds) ([]mload.Condition, error) {
	conditions := make([]mload.Condition, 0, len(th.Entries))
	for _, entry := range th.Entries {
		var (
			cond mload.Condition
			err  error
		)
		if entry.Expr != "" {
			cond, err = mload.ParseCondition(entry.Expr)
		} else {
			cond, err = mload.ParseComparison(entry.Step, entry.Metric, entry.Comparison)
		}
		if err != nil {
			if entry.Line > 0 {
				return nil, fmt.Errorf("threshold on line %d: %w", entry.Line, err)
			}
			return nil, fmt.Errorf("threshold: %w", err)
		}
		conditions = append(conditions, cond)
	}

	slices.SortStableFunc(conditions, func(a, b mload.Condition) int {
		if a.Step != b.Step {
			// "" (run-wide) sorts first.
			return strings.Compare(a.Step, b.Step)
		}
		return slices.Index(mload.Metrics, a.Metric) - slices.Index(mload.Metrics, b.Metric)
	})
	return conditions, nil
}

func convertAbortRule(rule YamlAbortRule) (mload.AbortRule, error) {
	cond, err := mload.ParseCondition(rule.When)
	if err != nil {
		return mload.AbortRule{}, err
	}
	out := mload.AbortRule{Condition: cond}
	if rule.Window != "" {
		window, werr := time.ParseDuration(rule.Window)
		if werr != nil || window <= 0 {
			return mload.AbortRule{}, fmt.Errorf("window %q must be a positive Go duration (e.g. 30s, 1m)", rule.Window)
		}
		out.Window = window
	}
	if rule.Delay != "" {
		delay, derr := time.ParseDuration(rule.Delay)
		if derr != nil || delay < 0 {
			return mload.AbortRule{}, fmt.Errorf("delay %q must be a non-negative Go duration (e.g. 30s)", rule.Delay)
		}
		out.Delay = delay
	}
	return out, nil
}

func joinExecutors(executors []mload.Executor) string {
	names := make([]string, 0, len(executors))
	for _, e := range executors {
		names = append(names, string(e))
	}
	return strings.Join(names, ", ")
}

// buildLoadScenarios renders the domain scenarios back to their YAML shape.
// Declaration order is preserved and durations are emitted in Go's canonical
// form, so exporting an already-exported document is a no-op.
func buildLoadScenarios(scenarios []mload.Scenario) []YamlLoadScenario {
	if len(scenarios) == 0 {
		return nil
	}

	out := make([]YamlLoadScenario, 0, len(scenarios))
	for _, s := range scenarios {
		entry := YamlLoadScenario{
			Name:             s.Name,
			Flow:             s.FlowName,
			Executor:         string(s.Executor),
			VUs:              s.VUs,
			Iterations:       s.MaxIterations,
			StartVUs:         s.StartVUs,
			Rate:             s.Rate,
			StartRate:        s.StartRate,
			PreAllocatedVUs:  s.PreAllocatedVUs,
			MaxVUs:           s.MaxVUs,
			Duration:         durationString(s.Duration),
			GracefulRampDown: durationString(s.GracefulRampDown),
			GracefulStop:     durationString(s.GracefulStop),
			TimeUnit:         durationString(s.TimeUnit),
		}
		for _, stage := range s.Stages {
			entry.Stages = append(entry.Stages, YamlLoadStage{Duration: stage.Duration.String(), Target: stage.Target})
		}
		if !s.ThinkTime.IsZero() {
			entry.ThinkTime = &YamlThinkTime{Min: s.ThinkTime.Min.String(), Max: s.ThinkTime.Max.String()}
		}
		if len(s.Thresholds) > 0 {
			th := &YamlLoadThresholds{}
			for _, c := range s.Thresholds {
				th.Entries = append(th.Entries, YamlThresholdEntry{
					Step: c.Step, Metric: string(c.Metric), Comparison: string(c.Op) + c.Value,
				})
			}
			entry.Thresholds = th
		}
		for _, rule := range s.Abort {
			entry.Abort = append(entry.Abort, YamlAbortRule{
				When:   rule.Condition.String(),
				Window: durationString(rule.Window),
				Delay:  durationString(rule.Delay),
			})
		}
		out = append(out, entry)
	}
	return out
}

// durationString renders d in Go's canonical form, or "" when unset.
func durationString(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	return d.String()
}
