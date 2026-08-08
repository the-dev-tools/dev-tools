// Package loadmetrics aggregates load-test execution results into interval
// frames using HDR histograms, keyed by (step, status-class).
//
// The usage shape is: an Aggregator collects Records concurrently during an
// interval, Flush drains that interval into an immutable Frame, and Merge
// lossily-free combines any number of Frames - whether successive flushes
// from one Aggregator, or one flush each from many concurrent Aggregators -
// into a single Report with derived percentiles.
//
// This package has no dependency on the rest of the server; its exported
// surface is a frozen contract consumed by the load-test ingest pipeline.
package loadmetrics

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	hdrhistogram "github.com/HdrHistogram/hdrhistogram-go"
)

// StatusClass buckets a recorded outcome for aggregation. Values are the
// literal strings carried on the wire (see the LoadMetricEntry TypeSpec
// model) - do not change them without updating that mapping.
type StatusClass string

const (
	StatusClass2xx     StatusClass = "2xx"
	StatusClass3xx     StatusClass = "3xx"
	StatusClass4xx     StatusClass = "4xx"
	StatusClass5xx     StatusClass = "5xx"
	StatusClassError   StatusClass = "error"
	StatusClassTimeout StatusClass = "timeout"
)

// ClassifyStatus buckets an HTTP-ish status code and/or transport error into
// a StatusClass. A non-nil err always wins over code (whatever code happens
// to be at that point, e.g. zero, is irrelevant once the request failed at
// the transport level). Timeouts - deadline-exceeded context errors, or any
// error the standard library recognizes via os.IsTimeout - classify as
// StatusClassTimeout rather than the more generic StatusClassError.
func ClassifyStatus(code int, err error) StatusClass {
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err) {
			return StatusClassTimeout
		}
		return StatusClassError
	}

	switch {
	case code >= 200 && code < 300:
		return StatusClass2xx
	case code >= 300 && code < 400:
		return StatusClass3xx
	case code >= 400 && code < 500:
		return StatusClass4xx
	case code >= 500 && code < 600:
		return StatusClass5xx
	default:
		return StatusClassError
	}
}

// Key identifies one aggregation bucket: a load-test step at a particular
// outcome class.
type Key struct {
	Step        string
	StatusClass StatusClass
}

// Entry is one (step, status-class) bucket's accumulated stats for a single
// interval.
type Entry struct {
	Count      int64
	ErrorCount int64
	Bytes      int64
	Hist       *hdrhistogram.Histogram // 1us..10min, 3 significant figures
}

// Frame is one flushed interval's worth of aggregated entries.
type Frame struct {
	IntervalStart time.Time
	Interval      time.Duration
	Entries       map[Key]Entry
}

const (
	// hdrLowestDiscernibleValue and hdrHighestTrackableValue bound recorded
	// latencies to 1 microsecond .. 10 minutes at 3 significant figures, per
	// the frozen contract. Values are clamped into this range before being
	// recorded (see clampMicros) so RecordValue can never fail on us.
	hdrLowestDiscernibleValue = int64(1)
	hdrSignificantFigures     = 3
)

var hdrHighestTrackableValue = (10 * time.Minute).Microseconds()

func newHistogram() *hdrhistogram.Histogram {
	return hdrhistogram.New(hdrLowestDiscernibleValue, hdrHighestTrackableValue, hdrSignificantFigures)
}

func clampMicros(us int64) int64 {
	switch {
	case us < hdrLowestDiscernibleValue:
		return hdrLowestDiscernibleValue
	case us > hdrHighestTrackableValue:
		return hdrHighestTrackableValue
	default:
		return us
	}
}

// Aggregator accumulates Records into per-key HDR histograms for the current
// interval. It is safe for concurrent use.
type Aggregator struct {
	mu            sync.Mutex
	intervalStart time.Time
	entries       map[Key]*Entry
}

// NewAggregator creates an Aggregator. interval documents the caller's
// intended flush cadence; it is not baked into Frame.Interval, which instead
// reflects the actual elapsed time between flushes (see Flush) so reported
// RPS stays correct even if the caller flushes early, late, or irregularly.
func NewAggregator(interval time.Duration) *Aggregator {
	_ = interval
	return &Aggregator{
		intervalStart: time.Now(),
		entries:       make(map[Key]*Entry),
	}
}

// Record adds one observation to the bucket identified by k. It is
// goroutine-safe.
func (a *Aggregator) Record(k Key, latency time.Duration, bytes int64, isErr bool) {
	us := clampMicros(latency.Microseconds())

	a.mu.Lock()
	defer a.mu.Unlock()

	e, ok := a.entries[k]
	if !ok {
		e = &Entry{Hist: newHistogram()}
		a.entries[k] = e
	}

	e.Count++
	if isErr {
		e.ErrorCount++
	}
	e.Bytes += bytes

	// us is always clamped into the histogram's configured range above, so
	// RecordValue cannot fail here.
	_ = e.Hist.RecordValue(us)
}

// Flush drains the current interval into a Frame and starts a fresh interval
// for subsequent Records. now becomes the next interval's start, so
// successive Flush calls describe contiguous, non-overlapping time ranges.
func (a *Aggregator) Flush(now time.Time) Frame {
	a.mu.Lock()
	defer a.mu.Unlock()

	frame := Frame{
		IntervalStart: a.intervalStart,
		Interval:      now.Sub(a.intervalStart),
		Entries:       make(map[Key]Entry, len(a.entries)),
	}
	for k, e := range a.entries {
		frame.Entries[k] = *e
	}

	a.entries = make(map[Key]*Entry)
	a.intervalStart = now

	return frame
}

// Stats is a set of derived, percentile-level statistics for a bucket, or
// for the whole run in Report.Total.
type Stats struct {
	Count, ErrorCount, Bytes int64
	P50, P90, P95, P99, Max  time.Duration
	RPS                      float64 // Count / covered wall time
}

// Report is the fully-merged result of one or more Frames: overall Stats
// plus a per-(step,status-class) breakdown.
type Report struct {
	Total   Stats
	PerStep map[Key]Stats
}

// counts accumulates the plain (non-histogram) fields of an Entry while
// merging; kept separate from Entry so a nil/absent Hist is never implied.
type counts struct {
	Count, ErrorCount, Bytes int64
}

// Merge combines any number of Frames - from one Aggregator's successive
// flushes, or from many concurrent Aggregators - into a single Report.
// Merging is lossless: percentiles on the merged histograms are equivalent
// (within HDR's significant-figure precision) to what a single Aggregator
// fed all the same observations would have produced.
//
// The wall time used for RPS is the union of the supplied frames' time
// ranges (earliest IntervalStart to latest IntervalStart+Interval), not the
// sum of their durations - this keeps RPS correct both for a single
// Aggregator's successive contiguous flushes and for many Aggregators
// flushing over the same overlapping window.
func Merge(frames []Frame) Report {
	perStepCounts := make(map[Key]*counts)
	perStepHist := make(map[Key]*hdrhistogram.Histogram)

	var start, end time.Time
	for i, f := range frames {
		if i == 0 || f.IntervalStart.Before(start) {
			start = f.IntervalStart
		}
		if fEnd := f.IntervalStart.Add(f.Interval); i == 0 || fEnd.After(end) {
			end = fEnd
		}

		for k, e := range f.Entries {
			c, ok := perStepCounts[k]
			if !ok {
				c = &counts{}
				perStepCounts[k] = c
			}
			c.Count += e.Count
			c.ErrorCount += e.ErrorCount
			c.Bytes += e.Bytes

			h, ok := perStepHist[k]
			if !ok {
				h = newHistogram()
				perStepHist[k] = h
			}
			if e.Hist != nil {
				h.Merge(e.Hist)
			}
		}
	}

	wallSeconds := end.Sub(start).Seconds()

	totalHist := newHistogram()
	var total counts
	perStep := make(map[Key]Stats, len(perStepCounts))
	for k, c := range perStepCounts {
		h := perStepHist[k]
		perStep[k] = statsFrom(*c, h, wallSeconds)

		total.Count += c.Count
		total.ErrorCount += c.ErrorCount
		total.Bytes += c.Bytes
		totalHist.Merge(h)
	}

	return Report{
		Total:   statsFrom(total, totalHist, wallSeconds),
		PerStep: perStep,
	}
}

func statsFrom(c counts, h *hdrhistogram.Histogram, wallSeconds float64) Stats {
	stats := Stats{
		Count:      c.Count,
		ErrorCount: c.ErrorCount,
		Bytes:      c.Bytes,
		P50:        microseconds(h.ValueAtPercentile(50)),
		P90:        microseconds(h.ValueAtPercentile(90)),
		P95:        microseconds(h.ValueAtPercentile(95)),
		P99:        microseconds(h.ValueAtPercentile(99)),
		Max:        microseconds(h.Max()),
	}
	if wallSeconds > 0 {
		stats.RPS = float64(c.Count) / wallSeconds
	}
	return stats
}

func microseconds(us int64) time.Duration {
	return time.Duration(us) * time.Microsecond
}
