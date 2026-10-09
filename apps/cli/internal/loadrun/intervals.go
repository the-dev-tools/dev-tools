package loadrun

import (
	"fmt"
	"sync"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner/scenariorunner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/loadmetrics"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mload"
)

// Stopper ends a load run early. The first Stop wins: its reason is the one
// the run reports. A Stopper is safe for concurrent use and for use before
// the run starts.
type Stopper struct {
	once   sync.Once
	done   chan struct{}
	mu     sync.Mutex
	reason string
}

// NewStopper returns a Stopper that has not been stopped.
func NewStopper() *Stopper {
	return &Stopper{done: make(chan struct{})}
}

// Stop ends the run: no new iteration starts, and in-flight ones are allowed
// the executor's graceful stop. Later calls are no-ops.
func (s *Stopper) Stop(reason string) {
	s.once.Do(func() {
		s.mu.Lock()
		s.reason = reason
		s.mu.Unlock()
		close(s.done)
	})
}

// Done is closed once Stop has been called.
func (s *Stopper) Done() <-chan struct{} { return s.done }

// Reason is the first Stop's reason, or "" while the run has not been
// stopped.
func (s *Stopper) Reason() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reason
}

// intervalMetrics flushes every VU's aggregator once per frame interval,
// combines the flushes into one IntervalFrame, keeps a running cumulative
// frame for the final report, and evaluates abort rules over a trailing
// window of recent frames.
//
// Memory stays bounded however long the run: the cumulative frame holds one
// histogram per (step, status class), and the abort window only the frames
// that fit in the longest rule's window.
type intervalMetrics struct {
	cfg       Config
	pool      *workerPool
	live      *scenariorunner.Live
	stopper   *Stopper
	startedAt time.Time
	interval  time.Duration
	maxWindow time.Duration

	// mu serializes cuts: the ticker's and the final one.
	mu         sync.Mutex
	seq        int64
	cumulative loadmetrics.Frame
	recent     []loadmetrics.Frame

	quit chan struct{}
	done chan struct{}
}

func newIntervalMetrics(cfg Config, pool *workerPool, live *scenariorunner.Live, stopper *Stopper, startedAt time.Time) *intervalMetrics {
	interval := cfg.FrameInterval
	if interval <= 0 {
		interval = DefaultFrameInterval
	}
	var maxWindow time.Duration
	for _, rule := range cfg.Abort {
		maxWindow = max(maxWindow, abortWindow(rule))
	}
	return &intervalMetrics{
		cfg:        cfg,
		pool:       pool,
		live:       live,
		stopper:    stopper,
		startedAt:  startedAt,
		interval:   interval,
		maxWindow:  maxWindow,
		cumulative: loadmetrics.Frame{IntervalStart: startedAt, Entries: map[loadmetrics.Key]loadmetrics.Entry{}},
		quit:       make(chan struct{}),
		done:       make(chan struct{}),
	}
}

func abortWindow(rule mload.AbortRule) time.Duration {
	if rule.Window > 0 {
		return rule.Window
	}
	return mload.DefaultAbortWindow
}

// start begins cutting a frame every interval.
func (m *intervalMetrics) start() {
	go func() {
		defer close(m.done)
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()
		for {
			select {
			case now := <-ticker.C:
				m.cut(now, false)
			case <-m.quit:
				return
			}
		}
	}()
}

// finish stops the ticker, cuts the final partial frame at now, and returns
// the cumulative frame plus the number of frames produced.
func (m *intervalMetrics) finish(now time.Time) (loadmetrics.Frame, int64) {
	close(m.quit)
	<-m.done
	m.cut(now, true)

	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cumulative, m.seq
}

// cut flushes every built worker at now and publishes the combined frame.
func (m *intervalMetrics) cut(now time.Time, final bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	workers := m.pool.built()
	flushed := make([]loadmetrics.Frame, 0, len(workers))
	for _, w := range workers {
		flushed = append(flushed, w.agg.Flush(now))
	}
	frame := loadmetrics.Combine(flushed)
	if len(flushed) == 0 {
		frame = loadmetrics.Frame{IntervalStart: now, Entries: map[loadmetrics.Key]loadmetrics.Entry{}}
	}

	m.cumulative = loadmetrics.Combine([]loadmetrics.Frame{m.cumulative, frame})

	if m.cfg.OnFrame != nil {
		m.cfg.OnFrame(IntervalFrame{
			Seq:       m.seq,
			Frame:     frame,
			ActiveVUs: m.live.ActiveVUs.Load(),
			Dropped:   m.live.Dropped.Load(),
			Final:     final,
		})
	}
	m.seq++

	if final || len(m.cfg.Abort) == 0 {
		return
	}
	m.recent = append(m.recent, frame)
	cutoff := now.Add(-m.maxWindow)
	for len(m.recent) > 0 && !m.recent[0].IntervalStart.Add(m.recent[0].Interval).After(cutoff) {
		m.recent = m.recent[1:]
	}
	if reason := m.checkAbort(now); reason != "" {
		m.stopper.Stop(reason)
	}
}

// checkAbort returns the reason of the first abort rule that holds, or "".
func (m *intervalMetrics) checkAbort(now time.Time) string {
	elapsed := now.Sub(m.startedAt)
	for _, rule := range m.cfg.Abort {
		if elapsed < rule.Delay {
			continue
		}
		window := abortWindow(rule)
		cutoff := now.Add(-window)
		inWindow := make([]loadmetrics.Frame, 0, len(m.recent))
		for _, f := range m.recent {
			if f.IntervalStart.Add(f.Interval).After(cutoff) {
				inWindow = append(inWindow, f)
			}
		}
		byStep := loadmetrics.Merge(foldByStep(inWindow))
		observed, ok := observe(rule.Condition, byStep)
		if ok && rule.Condition.Holds(observed) {
			return fmt.Sprintf("abort rule %s held (observed %s)", rule, rule.Condition.FormatObserved(observed))
		}
	}
	return ""
}
