package cmd

import (
	"context"
	"log"
	"log/slog"

	"github.com/the-dev-tools/dev-tools/apps/cli/internal/loadrun"
	"github.com/the-dev-tools/dev-tools/apps/cli/internal/reporter"
	"github.com/the-dev-tools/dev-tools/apps/cli/internal/runner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mload"
)

// runLoad executes the workflow file as a load test instead of a functional
// run.
//
// Exit codes follow the load-testing convention rather than the functional
// one: a run that completed is a success even if requests inside it failed,
// because deciding whether an error rate is acceptable is what thresholds are
// for (Phase 2). Only a run that could not happen - a bad scenario name, an
// unusable profile, or a target that was never reachable - is an error, and
// therefore a non-zero exit.
func runLoad(
	ctx context.Context,
	opts loadrun.Options,
	scenarios []mload.Scenario,
	flows []mflow.Flow,
	flowNameArg string,
	services runner.RunnerServices,
	logger *slog.Logger,
	reporters *reporter.ReporterGroup,
) error {
	cfg, err := loadrun.ResolveConfig(opts, scenarios, flows, flowNameArg)
	if err != nil {
		return err
	}

	if !quietMode {
		log.Printf("Load run: flow %q with %d VUs", cfg.Flow.Name, cfg.VUs)
	}

	result, runErr := loadrun.Run(ctx, cfg, services, logger)

	// A run that executed gets reported even when it also failed - the table
	// and the JSON are how anyone works out what went wrong. The failure still
	// decides the exit code, below.
	var flushErr error
	if result.Ran() {
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
				WorkerVersion: version,
			},
			Report: result.Report,
			ByStep: result.ByStep,
		})
		flushErr = reporters.Flush()
	}

	if runErr != nil {
		return runErr
	}
	return flushErr
}
