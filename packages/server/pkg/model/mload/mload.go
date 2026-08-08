// Package mload holds the domain model for load-test scenarios: the `load:`
// block of a yamlflow document, decoded into engine-ready values.
//
// It deliberately knows nothing about YAML or about the load runner. The
// yamlflow translator produces these, and the CLI's load runner consumes
// them, so neither has to depend on the other.
package mload

import "time"

// Executor names the scheduling strategy a scenario uses.
type Executor string

const (
	// ExecutorConstantVUs holds a fixed number of virtual users for the
	// scenario's duration or iteration budget. It is the only executor this
	// build implements; ramping-vus and constant-arrival-rate are Phase 2.
	ExecutorConstantVUs Executor = "constant-vus"
)

// SupportedExecutors lists the executors this build accepts, for error
// messages that have to name the valid alternatives.
var SupportedExecutors = []Executor{ExecutorConstantVUs}

// Scenario is one entry of the `load:` block: a named load profile applied to
// an existing flow. Flows are never edited to be load-tested, so a Scenario
// refers to its flow by name rather than owning it.
//
// Duration and MaxIterations are stop conditions; at least one is set. When
// both are set, whichever is reached first ends the scenario.
type Scenario struct {
	// Name identifies the scenario, e.g. for `flow run --scenario <name>`.
	Name string
	// FlowName is the flow this scenario drives, by its `flows:` entry name.
	FlowName string
	// Executor is the scheduling strategy; always ExecutorConstantVUs today.
	Executor Executor
	// VUs is the number of concurrent virtual users. Always >= 1.
	VUs int
	// Duration bounds the window during which new iterations start. Zero
	// means unbounded, in which case MaxIterations is set.
	Duration time.Duration
	// MaxIterations bounds the total iterations issued. Zero means
	// unbounded, in which case Duration is set.
	MaxIterations int64
}
