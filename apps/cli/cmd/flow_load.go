package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"slices"
	"time"

	"github.com/the-dev-tools/dev-tools/apps/cli/internal/loadrun"
	"github.com/the-dev-tools/dev-tools/apps/cli/internal/reporter"
	"github.com/the-dev-tools/dev-tools/apps/cli/internal/runner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mload"
	yamlflowsimplev2 "github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/yamlflowsimplev2"
)

// Exit codes for load runs that executed but did not pass. They follow k6's,
// so CI tooling written for one reads the other.
const (
	// ExitThresholdsFailed: the run completed and a threshold did not hold.
	ExitThresholdsFailed = 99
	// ExitAborted: the run was stopped early by an abort rule or by the
	// frame reporter's dead-man switch.
	ExitAborted = 108
)

// ExitCode maps an error returned by the root command to a process exit
// code: ExitAborted and ExitThresholdsFailed for load runs that executed but
// did not pass (an abort wins when both apply), 1 for every other failure,
// and 0 for nil. Programs embedding Root() use it to exit the way this
// binary does.
func ExitCode(err error) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, loadrun.ErrAborted):
		return ExitAborted
	case errors.Is(err, loadrun.ErrThresholdsFailed):
		return ExitThresholdsFailed
	default:
		return 1
	}
}

// loadFileScenarios reads the --load-file entries and appends them to the
// flow file's own load: block. The file uses exactly the load: block schema
// (or a bare list of its entries); a name defined in both places is an
// error, since --scenario could not tell them apart.
func loadFileScenarios(path string, inline []mload.Scenario, flows []mflow.Flow) ([]mload.Scenario, error) {
	if path == "" {
		return inline, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("--load-file: %w", err)
	}
	names := make([]string, 0, len(flows))
	for _, f := range flows {
		names = append(names, f.Name)
	}
	fromFile, err := yamlflowsimplev2.ParseLoadScenarios(data, names)
	if err != nil {
		return nil, fmt.Errorf("--load-file %s: %w", path, err)
	}

	merged := slices.Clone(inline)
	for _, s := range fromFile {
		if slices.ContainsFunc(inline, func(existing mload.Scenario) bool { return existing.Name == s.Name }) {
			return nil, fmt.Errorf("--load-file %s: load scenario %q is also defined in the workflow file", path, s.Name)
		}
		merged = append(merged, s)
	}
	return merged, nil
}

// runLoad executes the workflow file as a load test instead of a functional
// run.
//
// Exit codes follow the load-testing convention rather than the functional
// one: a run that completed is a success even if requests inside it failed,
// because deciding whether an error rate is acceptable is what thresholds are
// for. A run whose thresholds fail exits ExitThresholdsFailed; one stopped
// early by an abort rule or the frames dead-man switch exits ExitAborted.
// A run that could not happen - a bad scenario name, an unusable profile, or
// a target that was never reachable - exits 1.
func runLoad(
	ctx context.Context,
	opts loadrun.Options,
	frameInterval time.Duration,
	scenarios []mload.Scenario,
	flows []mflow.Flow,
	flowNameArg string,
	services runner.RunnerServices,
	logger *slog.Logger,
	reporters *reporter.ReporterGroup,
) error {
	cfg, err := loadrun.ResolveConfig(opts, scenarios, flows, flowNameArg)
	if err != nil {
		releaseFrameSink(reporters)
		return err
	}
	cfg.FrameInterval = frameInterval
	stopper := loadrun.NewStopper()
	cfg.Stopper = stopper

	if sink := reporters.FrameSink(); sink != nil {
		interval := frameInterval
		if interval <= 0 {
			interval = loadrun.DefaultFrameInterval
		}
		sink.SetRunInfo(cfg.ScenarioName, cfg.Flow.Name, version)
		// The endpoint gets at least three frame intervals to answer, so a
		// long --frame-interval cannot trip the switch between frames.
		sink.SetDeadMan(max(reporter.DefaultDeadManAfter, 3*interval), stopper.Stop)
		cfg.OnFrame = func(f loadrun.IntervalFrame) {
			sink.SendFrame(f.Seq, f.Frame, f.ActiveVUs, f.Dropped, f.Final)
		}
		sink.Start()
	}

	if !quietMode {
		if profile := cfg.Describe(); profile != "" {
			log.Printf("Load run: flow %q, %s: %s", cfg.Flow.Name, cfg.Executor, profile)
		} else {
			log.Printf("Load run: flow %q with %d VUs", cfg.Flow.Name, cfg.VUs)
		}
	}

	result, runErr := loadrun.Run(ctx, cfg, services, logger)

	// A run that executed gets reported even when it also failed - the table
	// and the JSON are how anyone works out what went wrong. The failure still
	// decides the exit code, below.
	var flushErr error
	if result.Ran() {
		thresholds := make([]reporter.LoadThresholdResult, 0, len(result.Thresholds))
		for _, t := range result.Thresholds {
			thresholds = append(thresholds, reporter.LoadThresholdResult{
				Expression: t.Condition.String(),
				Passed:     t.Passed,
				Observed:   t.ObservedText(),
			})
		}
		reporters.SetLoadReport(&reporter.LoadReport{
			Meta: reporter.LoadRunMeta{
				ScenarioName:  result.Config.ScenarioName,
				FlowName:      cfg.Flow.Name,
				VUs:           result.Config.VUs,
				Duration:      result.Config.Duration,
				MaxIterations: result.Config.MaxIterations,
				Iterations:    result.Summary.Iterations,
				Errors:        result.Summary.Errors,
				Elapsed:       result.Summary.Elapsed,
				Requests:      result.Report.Total.Count,
				Interrupted:   result.Summary.Interrupted,
				Dropped:       result.Summary.Dropped,
				WorkerVersion: version,
				Executor:      string(result.Config.Executor),
				Profile:       result.Config.Describe(),
				Thresholds:    thresholds,
				AbortReason:   result.AbortReason,
			},
			Report: result.Report,
			ByStep: result.ByStep,
		})
		flushErr = reporters.Flush()
	} else {
		releaseFrameSink(reporters)
	}

	if runErr != nil {
		return runErr
	}
	return flushErr
}

// releaseFrameSink stops the frames reporter's background sender for a run
// that never executed; with no load report it posts nothing.
func releaseFrameSink(reporters *reporter.ReporterGroup) {
	if sink := reporters.FrameSink(); sink != nil {
		_ = sink.Flush()
	}
}
