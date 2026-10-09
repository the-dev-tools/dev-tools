package scenariorunner

import (
	"math"
	"testing"
	"time"
)

func TestRampingTarget(t *testing.T) {
	stages := []Stage{
		{Duration: 10 * time.Second, Target: 10}, // 0 -> 10
		{Duration: 10 * time.Second, Target: 10}, // hold
		{Duration: 10 * time.Second, Target: 0},  // 10 -> 0
	}
	cases := []struct {
		at   time.Duration
		want int
	}{
		{0, 0},
		{999 * time.Millisecond, 0},
		{time.Second, 1},
		{5 * time.Second, 5},
		{10 * time.Second, 10},
		{15 * time.Second, 10},
		{20 * time.Second, 10},
		{25 * time.Second, 5},
		{29*time.Second + 999*time.Millisecond, 0},
		{30 * time.Second, 0},
		{time.Hour, 0},
	}
	for _, tc := range cases {
		if got := rampingTarget(0, stages, tc.at); got != tc.want {
			t.Errorf("rampingTarget(%v) = %d, want %d", tc.at, got, tc.want)
		}
	}

	if got := rampingTarget(3, []Stage{{Duration: time.Second, Target: 3}}, 500*time.Millisecond); got != 3 {
		t.Errorf("a hold at the start value = %d, want 3", got)
	}
}

func TestArrivalScheduleConstant(t *testing.T) {
	// 10 iterations per second for one second: starts at 0, 100ms, ... 900ms.
	sched := newArrivalSchedule(10, []Stage{{Duration: time.Second, Target: 10}}, time.Second)

	var got []time.Duration
	for k := int64(0); ; k++ {
		at, ok := sched.at(k)
		if !ok {
			break
		}
		got = append(got, at)
	}
	if len(got) != 10 {
		t.Fatalf("got %d starts, want 10: %v", len(got), got)
	}
	for i, at := range got {
		want := time.Duration(i) * 100 * time.Millisecond
		if diff := at - want; diff < -time.Microsecond || diff > time.Microsecond {
			t.Errorf("start %d at %v, want %v", i, at, want)
		}
	}
}

func TestArrivalScheduleTimeUnit(t *testing.T) {
	// 30 per minute is one every two seconds.
	sched := newArrivalSchedule(30, []Stage{{Duration: 10 * time.Second, Target: 30}}, time.Minute)
	at, ok := sched.at(1)
	if !ok || at != 2*time.Second {
		t.Errorf("second start at %v (ok=%v), want 2s", at, ok)
	}
	if n := countStarts(sched); n != 5 {
		t.Errorf("starts = %d, want 5", n)
	}
}

func TestArrivalScheduleRamping(t *testing.T) {
	// 0 -> 10/s over 1s delivers 5 iterations (the area under the ramp), the
	// k-th at sqrt(2k/10) seconds, then 10/s held for 1s delivers 10 more.
	sched := newArrivalSchedule(0, []Stage{
		{Duration: time.Second, Target: 10},
		{Duration: time.Second, Target: 10},
	}, time.Second)

	if n := countStarts(sched); n != 15 {
		t.Errorf("starts = %d, want 15", n)
	}
	for k := int64(0); k < 5; k++ {
		at, _ := sched.at(k)
		want := time.Duration(math.Sqrt(float64(2*k)/10) * float64(time.Second))
		if diff := at - want; diff < -time.Microsecond || diff > time.Microsecond {
			t.Errorf("ramp start %d at %v, want %v", k, at, want)
		}
	}

	var prev time.Duration
	for k := int64(0); k < 15; k++ {
		at, _ := sched.at(k)
		if at < prev {
			t.Fatalf("start %d at %v precedes start %d at %v", k, at, k-1, prev)
		}
		prev = at
	}
}

func TestArrivalScheduleRampDown(t *testing.T) {
	// 10/s -> 0 over 2s delivers 10 iterations, front-loaded.
	sched := newArrivalSchedule(10, []Stage{{Duration: 2 * time.Second, Target: 0}}, time.Second)
	if n := countStarts(sched); n != 10 {
		t.Errorf("starts = %d, want 10", n)
	}
	first, _ := sched.at(1)
	last, _ := sched.at(9)
	if first >= 200*time.Millisecond || last <= time.Second {
		t.Errorf("ramp-down is not front-loaded: start 1 at %v, start 9 at %v", first, last)
	}
}

func TestArrivalScheduleZeroRateStage(t *testing.T) {
	sched := newArrivalSchedule(0, []Stage{
		{Duration: time.Second, Target: 0},
		{Duration: time.Second, Target: 5},
	}, time.Second)
	first, ok := sched.at(0)
	if !ok || first != time.Second {
		t.Errorf("first start at %v (ok=%v), want 1s - a zero-rate stage starts nothing", first, ok)
	}
}

func countStarts(s *arrivalSchedule) int {
	n := 0
	for k := int64(0); ; k++ {
		if _, ok := s.at(k); !ok {
			return n
		}
		n++
	}
}
