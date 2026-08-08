package loadmetrics

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyStatus(t *testing.T) {
	tests := []struct {
		name string
		code int
		err  error
		want StatusClass
	}{
		{"200 OK classifies as 2xx", 200, nil, StatusClass2xx},
		{"299 upper boundary classifies as 2xx", 299, nil, StatusClass2xx},
		{"301 redirect classifies as 3xx", 301, nil, StatusClass3xx},
		{"404 not found classifies as 4xx", 404, nil, StatusClass4xx},
		{"500 server error classifies as 5xx", 500, nil, StatusClass5xx},
		{"599 upper boundary classifies as 5xx", 599, nil, StatusClass5xx},
		{"zero code with no error classifies as error", 0, nil, StatusClassError},
		{"transport error with no status classifies as error", 0, errors.New("connection reset by peer"), StatusClassError},
		{"context deadline exceeded classifies as timeout", 0, context.DeadlineExceeded, StatusClassTimeout},
		{"wrapped context deadline exceeded classifies as timeout", 0, fmt.Errorf("dial: %w", context.DeadlineExceeded), StatusClassTimeout},
		{"os deadline exceeded classifies as timeout", 0, os.ErrDeadlineExceeded, StatusClassTimeout},
		{"error takes precedence over a successful code", 200, errors.New("boom"), StatusClassError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyStatus(tt.code, tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestAggregatorKeysByStatusClass checks that ClassifyStatus's output, used
// as the Key.StatusClass for Record, buckets outcomes for the same step into
// separate Frame entries rather than clobbering a single counter.
func TestAggregatorKeysByStatusClass(t *testing.T) {
	agg := NewAggregator(time.Second)
	const step = "checkout"

	outcomes := []struct {
		code int
		err  error
	}{
		{200, nil},
		{200, nil},
		{404, nil},
		{500, nil},
		{500, nil},
		{500, nil},
		{0, context.DeadlineExceeded},
	}

	for _, o := range outcomes {
		class := ClassifyStatus(o.code, o.err)
		agg.Record(Key{Step: step, StatusClass: class}, time.Millisecond, 0, o.err != nil || o.code >= 400)
	}

	frame := agg.Flush(time.Now())

	assert.Equal(t, int64(2), frame.Entries[Key{Step: step, StatusClass: StatusClass2xx}].Count)
	assert.Equal(t, int64(1), frame.Entries[Key{Step: step, StatusClass: StatusClass4xx}].Count)
	assert.Equal(t, int64(3), frame.Entries[Key{Step: step, StatusClass: StatusClass5xx}].Count)
	assert.Equal(t, int64(1), frame.Entries[Key{Step: step, StatusClass: StatusClassTimeout}].Count)
	assert.Len(t, frame.Entries, 4, "one bucket per distinct status class, not one bucket per step")

	fiveXx := frame.Entries[Key{Step: step, StatusClass: StatusClass5xx}]
	assert.Equal(t, int64(3), fiveXx.ErrorCount, "5xx outcomes should also count as errors")
}

// TestAggregatorPercentileCorrectness records a uniform 1..1000ms distribution
// and checks the HDR-derived P50/P99 land within 1% of the true values,
// which is well outside HDR's 3-significant-figure quantization error at
// this magnitude.
func TestAggregatorPercentileCorrectness(t *testing.T) {
	agg := NewAggregator(time.Minute)
	k := Key{Step: "load-step", StatusClass: StatusClass2xx}

	for ms := 1; ms <= 1000; ms++ {
		agg.Record(k, time.Duration(ms)*time.Millisecond, 0, false)
	}

	frame := agg.Flush(time.Now())
	entry, ok := frame.Entries[k]
	require.True(t, ok, "expected an entry for the recorded key")
	require.NotNil(t, entry.Hist)
	require.Equal(t, int64(1000), entry.Count)

	wantP50 := 500 * time.Millisecond
	wantP99 := 990 * time.Millisecond

	gotP50 := time.Duration(entry.Hist.ValueAtPercentile(50)) * time.Microsecond
	gotP99 := time.Duration(entry.Hist.ValueAtPercentile(99)) * time.Microsecond

	assert.InDelta(t, wantP50, gotP50, float64(wantP50)*0.01, "P50 should be within 1%% of 500ms")
	assert.InDelta(t, wantP99, gotP99, float64(wantP99)*0.01, "P99 should be within 1%% of 990ms")
}

// TestMergeEquivalence feeds the same 1000-sample distribution into a single
// Aggregator and, separately, into two Aggregators fed disjoint (even/odd)
// halves. Merging the two half-frames must reproduce the same counts and
// percentiles (within HDR tolerance) as the single full aggregator - proving
// Merge genuinely combines independent histograms rather than trivially
// matching a self-merge.
func TestMergeEquivalence(t *testing.T) {
	k := Key{Step: "load-step", StatusClass: StatusClass2xx}

	full := NewAggregator(time.Minute)
	evens := NewAggregator(time.Minute)
	odds := NewAggregator(time.Minute)

	for ms := 1; ms <= 1000; ms++ {
		latency := time.Duration(ms) * time.Millisecond
		isErr := ms%13 == 0
		size := int64(ms * 7)

		full.Record(k, latency, size, isErr)
		if ms%2 == 0 {
			evens.Record(k, latency, size, isErr)
		} else {
			odds.Record(k, latency, size, isErr)
		}
	}

	now := time.Now()
	fullFrame := full.Flush(now)
	evensFrame := evens.Flush(now)
	oddsFrame := odds.Flush(now)

	// Sanity: the two halves are genuinely disjoint subsets, not duplicates
	// of the full data - otherwise this test would pass trivially.
	evensOnly := Merge([]Frame{evensFrame}).PerStep[k]
	require.Equal(t, int64(500), evensOnly.Count)

	fullStats := Merge([]Frame{fullFrame}).PerStep[k]
	splitStats := Merge([]Frame{evensFrame, oddsFrame}).PerStep[k]

	require.Equal(t, int64(1000), fullStats.Count)
	assert.Equal(t, fullStats.Count, splitStats.Count)
	assert.Equal(t, fullStats.ErrorCount, splitStats.ErrorCount)
	assert.Equal(t, fullStats.Bytes, splitStats.Bytes)
	assert.Equal(t, fullStats.Max, splitStats.Max)

	tolerance := func(want time.Duration) float64 { return float64(want) * 0.01 }
	assert.InDelta(t, fullStats.P50, splitStats.P50, tolerance(fullStats.P50))
	assert.InDelta(t, fullStats.P90, splitStats.P90, tolerance(fullStats.P90))
	assert.InDelta(t, fullStats.P95, splitStats.P95, tolerance(fullStats.P95))
	assert.InDelta(t, fullStats.P99, splitStats.P99, tolerance(fullStats.P99))
}

// TestMergeRPSMath constructs a Frame directly (bypassing wall-clock timing
// entirely) so the RPS = Count / covered-wall-time formula can be checked
// exactly: 100 records covering a 5s frame must yield 20.0 RPS.
func TestMergeRPSMath(t *testing.T) {
	k := Key{Step: "load-step", StatusClass: StatusClass2xx}

	hist := newHistogram()
	require.NoError(t, hist.RecordValues(10_000, 100)) // 100 samples at 10ms, arbitrary

	frame := Frame{
		IntervalStart: time.Unix(1_700_000_000, 0),
		Interval:      5 * time.Second,
		Entries: map[Key]Entry{
			k: {Count: 100, ErrorCount: 3, Bytes: 12_345, Hist: hist},
		},
	}

	report := Merge([]Frame{frame})

	assert.Equal(t, int64(100), report.Total.Count)
	assert.Equal(t, int64(3), report.Total.ErrorCount)
	assert.Equal(t, int64(12_345), report.Total.Bytes)
	assert.InDelta(t, 20.0, report.Total.RPS, 1e-9)
	assert.InDelta(t, 20.0, report.PerStep[k].RPS, 1e-9)
}

// TestMergeEmptyFrames guards the RPS division-by-zero edge case: merging no
// frames must not produce NaN/Inf.
func TestMergeEmptyFrames(t *testing.T) {
	report := Merge(nil)

	assert.Equal(t, int64(0), report.Total.Count)
	assert.InDelta(t, 0.0, report.Total.RPS, 1e-9)
	assert.Empty(t, report.PerStep)
}

// TestAggregatorFlushDrainsAndResets checks that Flush both returns the
// interval's data and starts a fresh interval for subsequent Records.
func TestAggregatorFlushDrainsAndResets(t *testing.T) {
	agg := NewAggregator(time.Second)
	k := Key{Step: "step", StatusClass: StatusClass2xx}

	agg.Record(k, 10*time.Millisecond, 100, false)

	first := agg.Flush(time.Now())
	require.Len(t, first.Entries, 1)
	assert.Equal(t, int64(1), first.Entries[k].Count)

	second := agg.Flush(time.Now())
	assert.Empty(t, second.Entries)
}

// TestAggregatorFlushIntervalReflectsElapsedTime drives the real Flush path
// (not a hand-built Frame, unlike TestMergeRPSMath) and checks Frame.Interval
// tracks actual elapsed wall-clock time between flushes, not the nominal
// interval passed to NewAggregator. NewAggregator is given a 5-minute nominal
// interval specifically so a blind echo of it would be trivially detectable
// (5m is nowhere near either sleep window below).
func TestAggregatorFlushIntervalReflectsElapsedTime(t *testing.T) {
	agg := NewAggregator(5 * time.Minute)
	k := Key{Step: "step", StatusClass: StatusClass2xx}

	agg.Record(k, time.Millisecond, 0, false)
	time.Sleep(20 * time.Millisecond)
	first := agg.Flush(time.Now())

	assert.GreaterOrEqual(t, first.Interval, 20*time.Millisecond, "Interval should be at least the real elapsed sleep")
	assert.Less(t, first.Interval, 5*time.Second, "Interval should not echo the nominal 5m interval")

	agg.Record(k, time.Millisecond, 0, false)
	time.Sleep(80 * time.Millisecond)
	second := agg.Flush(time.Now())

	assert.GreaterOrEqual(t, second.Interval, 80*time.Millisecond)
	assert.Less(t, second.Interval, 5*time.Second)

	// Different sleep windows must produce different Interval magnitudes -
	// proves Interval is derived per-flush, not a single constant (nominal or
	// otherwise) repeated every time.
	assert.Greater(t, second.Interval, first.Interval, "the second flush slept longer, so its Interval should be larger")

	// Each Flush starts the next interval at `now`, so IntervalStart must
	// advance between successive flushes.
	assert.True(t, second.IntervalStart.After(first.IntervalStart), "IntervalStart should advance between successive flushes")
}

// TestAggregatorRecordConcurrentRace exercises Record from many goroutines
// concurrently; run with -race to prove there's no data race, and assert the
// final counts to also catch lost-update bugs.
func TestAggregatorRecordConcurrentRace(t *testing.T) {
	agg := NewAggregator(time.Second)
	const goroutines = 8
	const perGoroutine = 10_000

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := range goroutines {
		go func(g int) {
			defer wg.Done()
			k := Key{Step: fmt.Sprintf("step-%d", g%3), StatusClass: StatusClass2xx}
			for i := range perGoroutine {
				latency := time.Duration(i%1000+1) * time.Microsecond
				agg.Record(k, latency, int64(i%256), i%97 == 0)
			}
		}(g)
	}
	wg.Wait()

	frame := agg.Flush(time.Now())

	var total int64
	for _, e := range frame.Entries {
		total += e.Count
	}
	assert.Equal(t, int64(goroutines*perGoroutine), total)
}
