package mload

import (
	"math"
	"slices"
)

// Scaled returns a copy of s sized for one machine of a split run: VU counts
// (and constant-vus iteration budgets) are multiplied by vusScale, and
// arrival rates by rateScale. Running the scaled scenario on 1/scale machines
// reproduces the original profile in aggregate.
//
// VU counts and iteration budgets are rounded to the nearest whole number,
// but a positive count never rounds down to zero - a machine asked to take
// part runs at least one VU. Rates are not rounded. A scale of 1 returns s
// unchanged; callers validate that scales are positive.
func (s Scenario) Scaled(vusScale, rateScale float64) Scenario {
	out := s
	out.Stages = slices.Clone(s.Stages)

	if vusScale != 1 {
		out.VUs = scaleCount(s.VUs, vusScale)
		out.StartVUs = scaleCount(s.StartVUs, vusScale)
		out.PreAllocatedVUs = scaleCount(s.PreAllocatedVUs, vusScale)
		out.MaxVUs = scaleCount(s.MaxVUs, vusScale)
		if s.MaxIterations > 0 {
			out.MaxIterations = max(int64(math.Round(float64(s.MaxIterations)*vusScale)), 1)
		}
		if s.Executor == ExecutorRampingVUs {
			for i := range out.Stages {
				out.Stages[i].Target = float64(scaleCount(int(s.Stages[i].Target), vusScale))
			}
		}
	}

	if rateScale != 1 {
		out.Rate = s.Rate * rateScale
		out.StartRate = s.StartRate * rateScale
		if s.Executor == ExecutorRampingArrivalRate {
			for i := range out.Stages {
				out.Stages[i].Target = s.Stages[i].Target * rateScale
			}
		}
	}

	return out
}

func scaleCount(n int, scale float64) int {
	if n <= 0 {
		return n
	}
	return max(int(math.Round(float64(n)*scale)), 1)
}
