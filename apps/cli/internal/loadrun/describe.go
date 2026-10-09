package loadrun

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mload"
)

// Describe summarizes a ramping or arrival-rate schedule in one line for the
// report header, e.g. "VUs 0->20->0 over 1m0s" or
// "50 iterations/1s for 5m0s, VUs 20-200". It is "" for constant-vus, whose
// header already shows VUs and stop conditions.
func (c Config) Describe() string {
	var parts []string
	switch c.executor() {
	case mload.ExecutorRampingVUs:
		parts = append(parts, fmt.Sprintf("VUs %s over %s",
			stageTargets(float64(c.StartVUs), c.Stages), totalStages(c.Stages)))
	case mload.ExecutorConstantArrivalRate:
		parts = append(parts,
			fmt.Sprintf("%s iterations/%s for %s", formatNumber(c.Rate), c.timeUnit(), c.Duration),
			c.describePool())
	case mload.ExecutorRampingArrivalRate:
		parts = append(parts,
			fmt.Sprintf("%s iterations/%s over %s", stageTargets(c.StartRate, c.Stages), c.timeUnit(), totalStages(c.Stages)),
			c.describePool())
	default:
		return ""
	}

	if !c.ThinkTime.IsZero() {
		think := c.ThinkTime.Min.String()
		if c.ThinkTime.Max != c.ThinkTime.Min {
			think += "-" + c.ThinkTime.Max.String()
		}
		parts = append(parts, "think time "+think)
	}
	return strings.Join(parts, ", ")
}

func (c Config) timeUnit() time.Duration {
	if c.TimeUnit > 0 {
		return c.TimeUnit
	}
	return mload.DefaultTimeUnit
}

func (c Config) describePool() string {
	if c.MaxVUs > c.PreAllocatedVUs {
		return fmt.Sprintf("VUs %d-%d", c.PreAllocatedVUs, c.MaxVUs)
	}
	return fmt.Sprintf("VUs %d", c.PreAllocatedVUs)
}

func stageTargets(start float64, stages []mload.Stage) string {
	targets := []string{formatNumber(start)}
	for _, s := range stages {
		targets = append(targets, formatNumber(s.Target))
	}
	return strings.Join(targets, "->")
}

func totalStages(stages []mload.Stage) time.Duration {
	var total time.Duration
	for _, s := range stages {
		total += s.Duration
	}
	return total
}

func formatNumber(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
