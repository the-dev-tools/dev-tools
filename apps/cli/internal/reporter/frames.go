package reporter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/the-dev-tools/dev-tools/apps/cli/internal/model"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/loadmetrics"
	load_metricsv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/load_metrics/v1"
)

// Environment variables the frame reporter reads.
const (
	// EnvFramesToken is sent as `Authorization: Bearer <token>` with every
	// frames request. Unset means no Authorization header.
	EnvFramesToken = "DEVTOOLS_FRAMES_TOKEN"
	// EnvWorkerID names this machine in every envelope; the hostname is
	// used when it is unset.
	EnvWorkerID = "DEVTOOLS_WORKER_ID"
)

// Frame reporter defaults.
const (
	DefaultDeadManAfter     = 60 * time.Second
	defaultFrameQueueSize   = 256
	defaultRetryBase        = 250 * time.Millisecond
	maxRetryBackoff         = 5 * time.Second
	defaultFrameAttempts    = 4
	defaultFinalAttempts    = 6
	defaultFinalTimeout     = 30 * time.Second
	defaultFrameHTTPTimeout = 10 * time.Second
)

// FrameSinkOptions configures a FrameSink. Zero values mean the defaults.
type FrameSinkOptions struct {
	// URL receives every frame and the final report, as JSON POSTs.
	URL string
	// Token is the bearer token; NewReporterGroup reads EnvFramesToken.
	Token string
	// WorkerID identifies this machine; NewReporterGroup reads EnvWorkerID,
	// falling back to the hostname.
	WorkerID string
	// Client is the HTTP client used for every request.
	Client *http.Client
	// QueueSize bounds the frames waiting to be sent. A frame that finds the
	// queue full is dropped (and counted), never waited for.
	QueueSize int
	// RetryBase is the first retry's backoff; each retry doubles it, up to
	// 5s.
	RetryBase time.Duration
	// FinalTimeout bounds Flush: draining queued frames plus posting the
	// final report.
	FinalTimeout time.Duration
	// DeadManAfter is how long the endpoint may go without accepting a
	// request, once the sink has started, before OnDeadMan fires.
	DeadManAfter time.Duration
	// OnDeadMan is called at most once, when the endpoint has been
	// unreachable for DeadManAfter. A load run uses it to stop itself, so a
	// generator whose controller has gone away cannot keep hitting a target.
	OnDeadMan func(reason string)
}

// FrameSink streams a load run's interval frames to an HTTP endpoint and
// posts the final report when the run is flushed. It implements Reporter, so
// it rides in a ReporterGroup like any other report target, and it is the
// reporter behind `--report frames:<url>`.
//
// Every POST body is a JSON envelope. A frame:
//
//	{"kind":"frame","worker":"...","scenario":"...","flow":"...","seq":0,
//	 "final":false,"active_vus":4,"dropped_iterations":0,
//	 "frame":<LoadMetricFrame as protojson, one entry per (step, status class)
//	          with its compressed HDR histogram>}
//
// and the final report:
//
//	{"kind":"report","worker":"...","scenario":"...","flow":"...",
//	 "frames_sent":12,"frames_dropped":0,"load_report":<the JSON reporter's load_report>}
//
// Sending never blocks the caller: frames are queued and posted by a single
// background goroutine, retried with exponential backoff on network errors,
// 408, 429 and 5xx responses.
type FrameSink struct {
	opts FrameSinkOptions

	mu         sync.Mutex
	scenario   string
	flow       string
	version    string
	loadReport *LoadReport
	started    bool
	closed     bool

	queue    chan frameItem
	ctx      context.Context
	cancel   context.CancelFunc
	sendDone chan struct{}

	lastOK     atomic.Int64 // unix nanos of the last accepted request
	sent       atomic.Int64
	dropped    atomic.Int64
	deadOnce   sync.Once
	watchDone  chan struct{}
	watchStart sync.Once
}

type frameItem struct {
	seq       int64
	frame     loadmetrics.Frame
	activeVUs int64
	dropped   int64
	final     bool
}

// NewFrameSink returns a sink for opts. Call Start when the run starts.
func NewFrameSink(opts FrameSinkOptions) *FrameSink {
	if opts.Client == nil {
		opts.Client = &http.Client{Timeout: defaultFrameHTTPTimeout}
	}
	if opts.QueueSize <= 0 {
		opts.QueueSize = defaultFrameQueueSize
	}
	if opts.RetryBase <= 0 {
		opts.RetryBase = defaultRetryBase
	}
	if opts.FinalTimeout <= 0 {
		opts.FinalTimeout = defaultFinalTimeout
	}
	if opts.DeadManAfter <= 0 {
		opts.DeadManAfter = DefaultDeadManAfter
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &FrameSink{
		opts:      opts,
		queue:     make(chan frameItem, opts.QueueSize),
		ctx:       ctx,
		cancel:    cancel,
		sendDone:  make(chan struct{}),
		watchDone: make(chan struct{}),
	}
	go s.sendLoop()
	return s
}

// SetRunInfo names the run in every envelope and the User-Agent.
func (s *FrameSink) SetRunInfo(scenario, flow, version string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scenario, s.flow, s.version = scenario, flow, version
}

// SetDeadMan installs the dead-man callback and its timeout; it must be
// called before Start. A zero after keeps the configured timeout.
func (s *FrameSink) SetDeadMan(after time.Duration, onDeadMan func(reason string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if after > 0 {
		s.opts.DeadManAfter = after
	}
	s.opts.OnDeadMan = onDeadMan
}

// Start begins the dead-man clock. Frames may be sent only after Start.
func (s *FrameSink) Start() {
	s.mu.Lock()
	s.started = true
	s.mu.Unlock()
	s.lastOK.Store(time.Now().UnixNano())
	s.watchStart.Do(func() { go s.watch() })
}

// SendFrame queues one interval frame for delivery and returns at once. A
// frame that finds the queue full is dropped and counted.
func (s *FrameSink) SendFrame(seq int64, frame loadmetrics.Frame, activeVUs, dropped int64, final bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		s.dropped.Add(1)
		return
	}
	select {
	case s.queue <- frameItem{seq: seq, frame: frame, activeVUs: activeVUs, dropped: dropped, final: final}:
	default:
		s.dropped.Add(1)
	}
}

// Sent is how many frames the endpoint accepted.
func (s *FrameSink) Sent() int64 { return s.sent.Load() }

// Dropped is how many frames were given up on: the queue was full, every
// retry failed, or the run ended before they could be sent.
func (s *FrameSink) Dropped() int64 { return s.dropped.Load() }

// SetLoadReport implements loadReportSink: the report is posted on Flush.
func (s *FrameSink) SetLoadReport(report *LoadReport) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadReport = report
}

// HandleFlowStart implements Reporter; functional runs stream nothing.
func (s *FrameSink) HandleFlowStart(FlowStartInfo) {}

// HandleNodeStatus implements Reporter; functional runs stream nothing.
func (s *FrameSink) HandleNodeStatus(NodeStatusEvent) {}

// HandleFlowResult implements Reporter; functional runs stream nothing.
func (s *FrameSink) HandleFlowResult(model.FlowRunResult) {}

// Flush drains the queued frames and posts the final report, within
// FinalTimeout. Without a load report (a run that never executed) it posts
// nothing. It returns an error when the final report could not be
// delivered; undelivered frames are only counted.
func (s *FrameSink) Flush() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	report := s.loadReport
	close(s.queue)
	s.mu.Unlock()

	defer close(s.watchDone)
	if report == nil {
		s.cancel()
		<-s.sendDone
		return nil
	}

	deadline := time.NewTimer(s.opts.FinalTimeout)
	defer deadline.Stop()
	select {
	case <-s.sendDone:
	case <-deadline.C:
		s.cancel()
		<-s.sendDone
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.opts.FinalTimeout)
	defer cancel()
	defer s.cancel()

	body, err := s.reportEnvelope(report)
	if err != nil {
		return fmt.Errorf("frames report: %w", err)
	}
	if err := s.post(ctx, body, defaultFinalAttempts); err != nil {
		return fmt.Errorf("frames report: final report to %s failed: %w", s.opts.URL, err)
	}
	return nil
}

func (s *FrameSink) sendLoop() {
	defer close(s.sendDone)
	for item := range s.queue {
		if s.ctx.Err() != nil {
			s.dropped.Add(1)
			continue
		}
		body, err := s.frameEnvelope(item)
		if err == nil {
			err = s.post(s.ctx, body, defaultFrameAttempts)
		}
		if err != nil {
			s.dropped.Add(1)
			continue
		}
		s.sent.Add(1)
	}
}

// watch fires the dead-man callback once the endpoint has gone
// DeadManAfter without accepting a request.
func (s *FrameSink) watch() {
	s.mu.Lock()
	after, onDeadMan := s.opts.DeadManAfter, s.opts.OnDeadMan
	s.mu.Unlock()
	if onDeadMan == nil {
		return
	}

	tick := min(max(after/10, 5*time.Millisecond), time.Second)
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			last := time.Unix(0, s.lastOK.Load())
			if time.Since(last) >= after {
				s.deadOnce.Do(func() {
					onDeadMan(fmt.Sprintf("frames endpoint unreachable for %s", after))
				})
				return
			}
		case <-s.watchDone:
			return
		}
	}
}

// errNotRetryable marks a response no retry can fix.
var errNotRetryable = errors.New("not retryable")

// post sends body, retrying retryable failures with exponential backoff
// until attempts run out or ctx ends.
func (s *FrameSink) post(ctx context.Context, body []byte, attempts int) error {
	backoff := s.opts.RetryBase
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		lastErr = s.postOnce(ctx, body)
		if lastErr == nil {
			s.lastOK.Store(time.Now().UnixNano())
			return nil
		}
		if errors.Is(lastErr, errNotRetryable) || attempt == attempts {
			break
		}
		timer := time.NewTimer(backoff)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return errors.Join(lastErr, ctx.Err())
		}
		backoff = min(backoff*2, maxRetryBackoff)
	}
	return lastErr
}

func (s *FrameSink) postOnce(ctx context.Context, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.opts.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("%w: %w", errNotRetryable, err)
	}
	req.Header.Set("Content-Type", "application/json")
	s.mu.Lock()
	version := s.version
	s.mu.Unlock()
	req.Header.Set("User-Agent", "devtoolscli/"+version)
	if s.opts.Token != "" {
		req.Header.Set("Authorization", "Bearer "+s.opts.Token)
	}

	resp, err := s.opts.Client.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == http.StatusRequestTimeout, resp.StatusCode == http.StatusTooManyRequests, resp.StatusCode >= 500:
		return fmt.Errorf("frames endpoint answered %s", resp.Status)
	default:
		return fmt.Errorf("%w: frames endpoint answered %s", errNotRetryable, resp.Status)
	}
}

type frameEnvelope struct {
	Kind              string          `json:"kind"`
	Worker            string          `json:"worker"`
	Scenario          string          `json:"scenario,omitempty"`
	Flow              string          `json:"flow"`
	Seq               int64           `json:"seq"`
	Final             bool            `json:"final,omitempty"`
	ActiveVUs         int64           `json:"active_vus"`
	DroppedIterations int64           `json:"dropped_iterations"`
	Frame             json.RawMessage `json:"frame"`
}

type reportEnvelope struct {
	Kind          string          `json:"kind"`
	Worker        string          `json:"worker"`
	Scenario      string          `json:"scenario,omitempty"`
	Flow          string          `json:"flow"`
	FramesSent    int64           `json:"frames_sent"`
	FramesDropped int64           `json:"frames_dropped"`
	LoadReport    *jsonLoadReport `json:"load_report"`
}

func (s *FrameSink) frameEnvelope(item frameItem) ([]byte, error) {
	proto, err := LoadMetricFrameProto(item.frame)
	if err != nil {
		return nil, err
	}
	frame, err := protojson.Marshal(proto)
	if err != nil {
		return nil, fmt.Errorf("serializing frame: %w", err)
	}
	s.mu.Lock()
	scenario, flow := s.scenario, s.flow
	s.mu.Unlock()
	return json.Marshal(frameEnvelope{
		Kind:              "frame",
		Worker:            s.opts.WorkerID,
		Scenario:          scenario,
		Flow:              flow,
		Seq:               item.seq,
		Final:             item.final,
		ActiveVUs:         item.activeVUs,
		DroppedIterations: item.dropped,
		Frame:             frame,
	})
}

func (s *FrameSink) reportEnvelope(report *LoadReport) ([]byte, error) {
	loadReport, err := buildJSONLoadReport(report)
	if err != nil {
		return nil, err
	}
	return json.Marshal(reportEnvelope{
		Kind:          "report",
		Worker:        s.opts.WorkerID,
		Scenario:      report.Meta.ScenarioName,
		Flow:          report.Meta.FlowName,
		FramesSent:    s.sent.Load(),
		FramesDropped: s.dropped.Load(),
		LoadReport:    loadReport,
	})
}

// LoadMetricFrameProto converts an interval frame to the wire model: one
// entry per (step, status class), sorted, each carrying its compressed HDR
// histogram alongside the derived percentiles.
func LoadMetricFrameProto(frame loadmetrics.Frame) (*load_metricsv1.LoadMetricFrame, error) {
	keys := make([]loadmetrics.Key, 0, len(frame.Entries))
	for key := range frame.Entries {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Step != keys[j].Step {
			return keys[i].Step < keys[j].Step
		}
		return keys[i].StatusClass < keys[j].StatusClass
	})

	out := &load_metricsv1.LoadMetricFrame{
		IntervalStart: timestamppb.New(frame.IntervalStart),
		IntervalMs:    frame.Interval.Milliseconds(),
		Entries:       make([]*load_metricsv1.LoadMetricEntry, 0, len(keys)),
	}
	for _, key := range keys {
		entry := frame.Entries[key]
		pb := &load_metricsv1.LoadMetricEntry{
			Step:        key.Step,
			StatusClass: LoadStatusClassToProto(key.StatusClass),
			Count:       entry.Count,
			ErrorCount:  entry.ErrorCount,
			Bytes:       entry.Bytes,
		}
		if entry.Hist != nil {
			encoded, err := loadmetrics.EncodeHistogram(entry.Hist)
			if err != nil {
				return nil, err
			}
			pb.HdrHistogram = encoded
			pb.P50Us = entry.Hist.ValueAtPercentile(50)
			pb.P90Us = entry.Hist.ValueAtPercentile(90)
			pb.P95Us = entry.Hist.ValueAtPercentile(95)
			pb.P99Us = entry.Hist.ValueAtPercentile(99)
			pb.MaxUs = entry.Hist.Max()
		}
		out.Entries = append(out.Entries, pb)
	}
	return out, nil
}

// FrameFromProto is the inverse of LoadMetricFrameProto, for a collector
// that merges frames from several machines with loadmetrics.Combine or
// loadmetrics.Merge.
func FrameFromProto(pb *load_metricsv1.LoadMetricFrame) (loadmetrics.Frame, error) {
	frame := loadmetrics.Frame{
		IntervalStart: pb.GetIntervalStart().AsTime(),
		Interval:      time.Duration(pb.GetIntervalMs()) * time.Millisecond,
		Entries:       make(map[loadmetrics.Key]loadmetrics.Entry, len(pb.GetEntries())),
	}
	for _, e := range pb.GetEntries() {
		entry := loadmetrics.Entry{Count: e.GetCount(), ErrorCount: e.GetErrorCount(), Bytes: e.GetBytes()}
		if len(e.GetHdrHistogram()) > 0 {
			hist, err := loadmetrics.DecodeHistogram(e.GetHdrHistogram())
			if err != nil {
				return loadmetrics.Frame{}, fmt.Errorf("step %q: %w", e.GetStep(), err)
			}
			entry.Hist = hist
		}
		frame.Entries[loadmetrics.Key{Step: e.GetStep(), StatusClass: LoadStatusClassFromProto(e.GetStatusClass())}] = entry
	}
	return frame, nil
}

// defaultWorkerID is EnvWorkerID, or the hostname.
func defaultWorkerID() string {
	if id := os.Getenv(EnvWorkerID); id != "" {
		return id
	}
	host, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return host
}
