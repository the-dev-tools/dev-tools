package mload

import (
	"reflect"
	"testing"
	"time"
)

func TestScaleIdentity(t *testing.T) {
	s := Scenario{Name: "a", Executor: ExecutorConstantVUs, VUs: 7, Duration: time.Minute, MaxIterations: 100}
	if got := s.Scaled(1, 1); !reflect.DeepEqual(got, s) {
		t.Errorf("Scaled(1, 1) = %+v, want unchanged %+v", got, s)
	}
}

func TestScaleConstantVUs(t *testing.T) {
	s := Scenario{Executor: ExecutorConstantVUs, VUs: 10, MaxIterations: 1000}
	got := s.Scaled(1.0/3, 5) // the rate scale does not touch a closed-model scenario
	if got.VUs != 3 {
		t.Errorf("VUs = %d, want 3 (10/3 rounded)", got.VUs)
	}
	if got.MaxIterations != 333 {
		t.Errorf("MaxIterations = %d, want 333", got.MaxIterations)
	}

	tiny := Scenario{Executor: ExecutorConstantVUs, VUs: 1, MaxIterations: 1}.Scaled(0.1, 1)
	if tiny.VUs != 1 || tiny.MaxIterations != 1 {
		t.Errorf("scaling must never take a configured count to zero, got %+v", tiny)
	}
}

func TestScaleRampingVUs(t *testing.T) {
	s := Scenario{
		Executor: ExecutorRampingVUs,
		StartVUs: 0,
		Stages:   []Stage{{Duration: time.Second, Target: 20}, {Duration: time.Second, Target: 0}},
	}
	got := s.Scaled(0.5, 1)
	if got.StartVUs != 0 {
		t.Errorf("StartVUs = %d, want 0", got.StartVUs)
	}
	want := []Stage{{Duration: time.Second, Target: 10}, {Duration: time.Second, Target: 0}}
	if !reflect.DeepEqual(got.Stages, want) {
		t.Errorf("Stages = %+v, want %+v", got.Stages, want)
	}
	if s.Stages[0].Target != 20 {
		t.Error("Scaled must not mutate the receiver's stages")
	}
}

func TestScaleArrivalRate(t *testing.T) {
	s := Scenario{
		Executor:        ExecutorRampingArrivalRate,
		StartRate:       10,
		Stages:          []Stage{{Duration: time.Second, Target: 100}},
		PreAllocatedVUs: 10,
		MaxVUs:          50,
	}
	got := s.Scaled(0.5, 0.25)
	if got.StartRate != 2.5 {
		t.Errorf("StartRate = %v, want 2.5", got.StartRate)
	}
	if got.Stages[0].Target != 25 {
		t.Errorf("stage target = %v, want 25", got.Stages[0].Target)
	}
	if got.PreAllocatedVUs != 5 || got.MaxVUs != 25 {
		t.Errorf("VU pool = %d/%d, want 5/25", got.PreAllocatedVUs, got.MaxVUs)
	}

	constant := Scenario{Executor: ExecutorConstantArrivalRate, Rate: 30, PreAllocatedVUs: 3}.Scaled(1, 1.0/3)
	if constant.Rate != 10 {
		t.Errorf("Rate = %v, want 10", constant.Rate)
	}
}
