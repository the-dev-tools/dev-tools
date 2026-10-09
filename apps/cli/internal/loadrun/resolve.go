package loadrun

import (
	"fmt"
	"strings"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mload"
)

// Options is the load-mode command line, before it has been reconciled with
// the workflow file.
type Options struct {
	// Requested is set by the command layer when the user passed any load
	// flag at all - including one whose value happens to be a zero, like
	// `--vus 0` or `--duration 30s` with no --vus. Those are load runs the
	// user got wrong, and they deserve a load-mode error rather than a
	// silent fall-through to a functional run.
	Requested bool
	// Scenario names an entry of the file's `load:` block.
	Scenario string
	// VUs, Duration and Iterations describe a profile inline, for runs that
	// do not want a scenario in the file.
	VUs        int
	Duration   time.Duration
	Iterations int64
	// VUsScale and RateScale size the profile for one machine of a run
	// split across several: VU counts are multiplied by VUsScale and
	// arrival rates by RateScale (see mload.Scenario.Scaled). Zero means 1.
	VUsScale  float64
	RateScale float64
}

// scales returns the effective scales, rejecting negative ones.
func (o Options) scales() (float64, float64, error) {
	vus, rate := o.VUsScale, o.RateScale
	if vus < 0 || rate < 0 {
		return 0, 0, fmt.Errorf("--vus-scale and --rate-scale must be > 0, got %v and %v", vus, rate)
	}
	if vus == 0 {
		vus = 1
	}
	if rate == 0 {
		rate = 1
	}
	return vus, rate, nil
}

// Enabled reports whether the user asked for a load run at all. Everything
// else about the invocation behaves exactly as it did before load mode
// existed when this is false.
//
// Values are honoured as well as Requested, so Options assembled without a
// command line - in a test, or by a future caller - still work.
func (o Options) Enabled() bool {
	return o.Requested || o.Scenario != "" || o.VUs > 0
}

// ResolveConfig turns the command line plus the workflow file into a runnable
// profile.
//
// flowNameArg is the optional positional flow argument. It is only consulted
// for flag-driven runs: a scenario already names its flow, and --scenario is
// mutually exclusive with the profile flags.
//
// The returned Config points into flows, so callers keep the identity of the
// flow they passed in.
func ResolveConfig(opts Options, scenarios []mload.Scenario, flows []mflow.Flow, flowNameArg string) (Config, error) {
	vusScale, rateScale, err := opts.scales()
	if err != nil {
		return Config{}, err
	}

	var (
		scenario mload.Scenario
		flow     *mflow.Flow
	)
	if opts.Scenario != "" {
		scenario, flow, err = resolveScenario(opts.Scenario, scenarios, flows)
	} else {
		scenario, flow, err = resolveFlagScenario(opts, flows, flowNameArg)
	}
	if err != nil {
		return Config{}, err
	}
	return ConfigFromScenario(scenario.Scaled(vusScale, rateScale), flow), nil
}

func resolveScenario(name string, scenarios []mload.Scenario, flows []mflow.Flow) (mload.Scenario, *mflow.Flow, error) {
	if len(scenarios) == 0 {
		return mload.Scenario{}, nil, fmt.Errorf(
			"unknown load scenario %q: this workflow file has no load: block", name)
	}

	names := make([]string, 0, len(scenarios))
	for _, s := range scenarios {
		names = append(names, s.Name)
	}

	for _, scenario := range scenarios {
		if scenario.Name != name {
			continue
		}
		flow := findFlow(flows, scenario.FlowName)
		if flow == nil {
			return mload.Scenario{}, nil, fmt.Errorf(
				"load scenario %q targets flow %q, which is not in this workflow file (flows: %s)",
				name, scenario.FlowName, flowNames(flows))
		}
		return scenario, flow, nil
	}

	return mload.Scenario{}, nil, fmt.Errorf(
		"unknown load scenario %q (scenarios in this file: %s)", name, strings.Join(names, ", "))
}

// resolveFlagScenario assembles a constant-vus scenario from the inline
// profile flags.
func resolveFlagScenario(opts Options, flows []mflow.Flow, flowNameArg string) (mload.Scenario, *mflow.Flow, error) {
	if opts.VUs == 0 {
		return mload.Scenario{}, nil, fmt.Errorf(
			"a load run needs virtual users: pass --vus N, or --scenario NAME to run an entry of the file's load: block")
	}
	if opts.VUs < 0 {
		return mload.Scenario{}, nil, fmt.Errorf("--vus must be >= 1, got %d", opts.VUs)
	}
	if opts.Duration <= 0 && opts.Iterations <= 0 {
		return mload.Scenario{}, nil, fmt.Errorf(
			"a load run needs a stop condition: pass --duration (e.g. --duration 60s), --iterations, or both")
	}
	if opts.Iterations < 0 {
		return mload.Scenario{}, nil, fmt.Errorf("--iterations must be >= 0, got %d", opts.Iterations)
	}

	flow, err := selectFlow(flows, flowNameArg)
	if err != nil {
		return mload.Scenario{}, nil, err
	}

	return mload.Scenario{
		FlowName:      flow.Name,
		Executor:      mload.ExecutorConstantVUs,
		VUs:           opts.VUs,
		Duration:      opts.Duration,
		MaxIterations: opts.Iterations,
	}, flow, nil
}

// selectFlow picks the flow a flag-driven load run should drive. A load run
// drives exactly one flow, so an ambiguous file is an error rather than a
// guess.
func selectFlow(flows []mflow.Flow, flowNameArg string) (*mflow.Flow, error) {
	if flowNameArg != "" {
		flow := findFlow(flows, flowNameArg)
		if flow == nil {
			return nil, fmt.Errorf("flow %q is not in this workflow file (flows: %s)", flowNameArg, flowNames(flows))
		}
		return flow, nil
	}
	if len(flows) == 1 {
		return &flows[0], nil
	}
	return nil, fmt.Errorf(
		"a load run drives one flow, but this file has %d: name one as an argument, or use --scenario to run a load: block entry (flows: %s)",
		len(flows), flowNames(flows))
}

func findFlow(flows []mflow.Flow, name string) *mflow.Flow {
	for i := range flows {
		if flows[i].Name == name {
			return &flows[i]
		}
	}
	return nil
}

func flowNames(flows []mflow.Flow) string {
	names := make([]string, 0, len(flows))
	for _, f := range flows {
		names = append(names, f.Name)
	}
	return strings.Join(names, ", ")
}
