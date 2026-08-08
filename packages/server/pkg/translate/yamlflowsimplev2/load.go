package yamlflowsimplev2

import (
	"fmt"
	"strings"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mload"
)

// convertLoadScenarios validates the `load:` block and decodes it into the
// engine-ready domain model, preserving declaration order.
//
// Every error names the offending scenario and, where there is a closed set of
// legal values, spells that set out - a load block is usually written by hand
// (or by an agent) and a nameless "invalid executor" is useless to both.
func convertLoadScenarios(yamlFormat *YamlFlowFormatV2) ([]mload.Scenario, error) {
	if len(yamlFormat.Load) == 0 {
		return nil, nil
	}

	flowNames := make([]string, 0, len(yamlFormat.Flows))
	knownFlows := make(map[string]bool, len(yamlFormat.Flows))
	for _, flow := range yamlFormat.Flows {
		flowNames = append(flowNames, flow.Name)
		knownFlows[flow.Name] = true
	}

	scenarios := make([]mload.Scenario, 0, len(yamlFormat.Load))
	seen := make(map[string]bool, len(yamlFormat.Load))

	for i, entry := range yamlFormat.Load {
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
	if executor != mload.ExecutorConstantVUs {
		return fail(
			"unsupported executor %q (this build supports: %s; ramping-vus and constant-arrival-rate arrive in Phase 2)",
			entry.Executor, joinExecutors(mload.SupportedExecutors))
	}

	if entry.VUs < 1 {
		return fail("vus must be >= 1, got %d", entry.VUs)
	}
	if entry.Iterations < 0 {
		return fail("iterations must be >= 0, got %d", entry.Iterations)
	}

	var duration time.Duration
	if entry.Duration != "" {
		parsed, err := time.ParseDuration(entry.Duration)
		if err != nil {
			return fail("duration %q is not a valid Go duration (e.g. 30s, 2m, 1h30m)", entry.Duration)
		}
		if parsed <= 0 {
			return fail("duration %q must be positive", entry.Duration)
		}
		duration = parsed
	}

	if duration == 0 && entry.Iterations == 0 {
		return fail("needs a stop condition: set duration, iterations, or both")
	}

	return mload.Scenario{
		Name:          entry.Name,
		FlowName:      entry.Flow,
		Executor:      executor,
		VUs:           entry.VUs,
		Duration:      duration,
		MaxIterations: entry.Iterations,
	}, nil
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
			Name:       s.Name,
			Flow:       s.FlowName,
			Executor:   string(s.Executor),
			VUs:        s.VUs,
			Iterations: s.MaxIterations,
		}
		if s.Duration > 0 {
			entry.Duration = s.Duration.String()
		}
		out = append(out, entry)
	}
	return out
}
