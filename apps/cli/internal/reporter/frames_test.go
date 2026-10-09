package reporter

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/loadmetrics"
	load_metricsv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/load_metrics/v1"
)

// frameCollector is a fake frames endpoint. respond decides each response
// status from the 1-based attempt number.
type frameCollector struct {
	*httptest.Server
	mu        sync.Mutex
	bodies    []map[string]json.RawMessage
	auths     []string
	attempts  atomic.Int64
	respond   func(attempt int64) int
	blockFor  time.Duration
	userAgent string
}

func newFrameCollector(t *testing.T, respond func(attempt int64) int) *frameCollector {
	t.Helper()
	c := &frameCollector{respond: respond}
	c.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := c.attempts.Add(1)
		if c.blockFor > 0 {
			select {
			case <-time.After(c.blockFor):
			case <-r.Context().Done():
				return
			}
		}
		status := http.StatusAccepted
		if c.respond != nil {
			status = c.respond(attempt)
		}
		if status < 300 {
			data, _ := io.ReadAll(r.Body)
			var body map[string]json.RawMessage
			if err := json.Unmarshal(data, &body); err != nil {
				t.Errorf("frames endpoint got invalid JSON: %v\n%s", err, data)
			}
			c.mu.Lock()
			c.bodies = append(c.bodies, body)
			c.auths = append(c.auths, r.Header.Get("Authorization"))
			c.userAgent = r.Header.Get("User-Agent")
			c.mu.Unlock()
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(c.Close)
	return c
}

func (c *frameCollector) received() []map[string]json.RawMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]map[string]json.RawMessage(nil), c.bodies...)
}

func sampleFrame(t *testing.T) loadmetrics.Frame {
	t.Helper()
	agg := loadmetrics.NewAggregator(time.Second)
	start := time.Unix(1_700_000_000, 0)
	agg.Flush(start)
	for ms := 1; ms <= 50; ms++ {
		agg.Record(loadmetrics.Key{Step: "PostLogin", StatusClass: loadmetrics.StatusClass2xx}, time.Duration(ms)*time.Millisecond, 10, false)
	}
	agg.Record(loadmetrics.Key{Step: "PostLogin", StatusClass: loadmetrics.StatusClass5xx}, time.Second, 0, true)
	agg.Record(loadmetrics.Key{Step: "Search", StatusClass: loadmetrics.StatusClass2xx}, 3*time.Millisecond, 5, false)
	return agg.Flush(start.Add(5 * time.Second))
}

func fastSinkOptions(url string) FrameSinkOptions {
	return FrameSinkOptions{
		URL:          url,
		Token:        "secret-token",
		WorkerID:     "machine-7",
		RetryBase:    5 * time.Millisecond,
		FinalTimeout: 2 * time.Second,
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestParseReportSpecsFrames(t *testing.T) {
	specs, err := ParseReportSpecs([]string{"console", "frames:https://ingest.example.com/v1/runs/abc/frames"})
	if err != nil {
		t.Fatalf("ParseReportSpecs: %v", err)
	}
	if len(specs) != 2 || specs[1].Format != ReportFormatFrames || specs[1].Path != "https://ingest.example.com/v1/runs/abc/frames" {
		t.Errorf("specs = %+v", specs)
	}

	for _, bad := range []string{"frames", "frames:", "frames:ftp://x", "frames:not a url"} {
		if _, err := ParseReportSpecs([]string{bad}); err == nil {
			t.Errorf("ParseReportSpecs(%q) succeeded, want an error", bad)
		}
	}
}

func TestFrameSinkPostsFramesWithHistograms(t *testing.T) {
	collector := newFrameCollector(t, nil)
	sink := NewFrameSink(fastSinkOptions(collector.URL))
	sink.SetRunInfo("checkout-baseline", "Checkout", "v1.2.3")
	t.Cleanup(func() { _ = sink.Flush() })
	sink.Start()

	sink.SendFrame(0, sampleFrame(t), 4, 0, false)
	sink.SendFrame(1, sampleFrame(t), 2, 3, true)
	waitFor(t, "two frames", func() bool { return len(collector.received()) == 2 })

	collector.mu.Lock()
	for _, auth := range collector.auths {
		if auth != "Bearer secret-token" {
			t.Errorf("Authorization = %q", auth)
		}
	}
	if !strings.Contains(collector.userAgent, "v1.2.3") {
		t.Errorf("User-Agent = %q, want the worker version", collector.userAgent)
	}
	collector.mu.Unlock()

	bodies := collector.received()
	var envelope struct {
		Kind              string          `json:"kind"`
		Worker            string          `json:"worker"`
		Scenario          string          `json:"scenario"`
		Flow              string          `json:"flow"`
		Seq               int64           `json:"seq"`
		Final             bool            `json:"final"`
		ActiveVUs         int64           `json:"active_vus"`
		DroppedIterations int64           `json:"dropped_iterations"`
		Frame             json.RawMessage `json:"frame"`
	}
	raw, _ := json.Marshal(bodies[1])
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Kind != "frame" || envelope.Worker != "machine-7" || envelope.Scenario != "checkout-baseline" ||
		envelope.Flow != "Checkout" || envelope.Seq != 1 || !envelope.Final ||
		envelope.ActiveVUs != 2 || envelope.DroppedIterations != 3 {
		t.Errorf("envelope = %+v", envelope)
	}

	var frame load_metricsv1.LoadMetricFrame
	if err := protojson.Unmarshal(envelope.Frame, &frame); err != nil {
		t.Fatalf("frame is not a LoadMetricFrame: %v\n%s", err, envelope.Frame)
	}
	if frame.GetIntervalMs() != 5000 || len(frame.GetEntries()) != 3 {
		t.Fatalf("frame = %v", &frame)
	}

	// The frame decodes back to the same per-step histograms, which is what
	// lets a collector merge frames from many machines.
	decoded, err := FrameFromProto(&frame)
	if err != nil {
		t.Fatalf("FrameFromProto: %v", err)
	}
	want := loadmetrics.Merge([]loadmetrics.Frame{sampleFrame(t)})
	got := loadmetrics.Merge([]loadmetrics.Frame{decoded})
	if got.Total.Count != want.Total.Count || got.Total.P95 != want.Total.P95 || got.Total.Max != want.Total.Max {
		t.Errorf("decoded frame total = %+v, want %+v", got.Total, want.Total)
	}
	login := got.PerStep[loadmetrics.Key{Step: "PostLogin", StatusClass: loadmetrics.StatusClass2xx}]
	if login.Count != 50 || login.Bytes != 500 {
		t.Errorf("PostLogin 2xx = %+v", login)
	}
}

func TestFrameSinkRetriesWithBackoff(t *testing.T) {
	collector := newFrameCollector(t, func(attempt int64) int {
		if attempt <= 2 {
			return http.StatusServiceUnavailable
		}
		return http.StatusOK
	})
	sink := NewFrameSink(fastSinkOptions(collector.URL))
	t.Cleanup(func() { _ = sink.Flush() })
	sink.Start()
	sink.SendFrame(0, sampleFrame(t), 1, 0, false)

	waitFor(t, "the retried frame", func() bool { return len(collector.received()) == 1 })
	if got := collector.attempts.Load(); got != 3 {
		t.Errorf("attempts = %d, want 3 (two 503s, then success)", got)
	}
}

func TestFrameSinkDoesNotRetryClientErrors(t *testing.T) {
	collector := newFrameCollector(t, func(int64) int { return http.StatusUnauthorized })
	sink := NewFrameSink(fastSinkOptions(collector.URL))
	t.Cleanup(func() { _ = sink.Flush() })
	sink.Start()
	sink.SendFrame(0, sampleFrame(t), 1, 0, false)

	waitFor(t, "one attempt", func() bool { return collector.attempts.Load() >= 1 })
	time.Sleep(100 * time.Millisecond)
	if got := collector.attempts.Load(); got != 1 {
		t.Errorf("attempts = %d, want 1: a 401 will not fix itself", got)
	}
}

func TestFrameSinkNeverBlocksTheRun(t *testing.T) {
	collector := newFrameCollector(t, nil)
	collector.blockFor = 2 * time.Second
	opts := fastSinkOptions(collector.URL)
	opts.QueueSize = 4
	sink := NewFrameSink(opts)
	t.Cleanup(func() { _ = sink.Flush() })
	sink.Start()

	frame := sampleFrame(t)
	start := time.Now()
	for i := range 200 {
		sink.SendFrame(int64(i), frame, 1, 0, false)
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Errorf("200 SendFrame calls against a hung endpoint took %v; they must not block", elapsed)
	}
	if dropped := sink.Dropped(); dropped < 190 {
		t.Errorf("Dropped() = %d, want the overflow beyond a 4-frame queue counted", dropped)
	}
}

func TestFrameSinkDeadManStopsTheRun(t *testing.T) {
	collector := newFrameCollector(t, func(int64) int { return http.StatusBadGateway })
	opts := fastSinkOptions(collector.URL)
	opts.DeadManAfter = 150 * time.Millisecond

	var reasons []string
	var mu sync.Mutex
	opts.OnDeadMan = func(reason string) {
		mu.Lock()
		reasons = append(reasons, reason)
		mu.Unlock()
	}
	sink := NewFrameSink(opts)
	t.Cleanup(func() { _ = sink.Flush() })
	sink.Start()
	for i := range 3 {
		sink.SendFrame(int64(i), sampleFrame(t), 1, 0, false)
	}

	waitFor(t, "the dead-man switch", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(reasons) > 0
	})
	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if len(reasons) != 1 {
		t.Errorf("dead-man fired %d times, want once", len(reasons))
	}
	if !strings.Contains(reasons[0], "frames endpoint unreachable for 150ms") {
		t.Errorf("reason = %q", reasons[0])
	}
}

func TestFrameSinkDeadManStaysQuietWhileHealthy(t *testing.T) {
	collector := newFrameCollector(t, nil)
	opts := fastSinkOptions(collector.URL)
	opts.DeadManAfter = 100 * time.Millisecond
	var fired atomic.Bool
	opts.OnDeadMan = func(string) { fired.Store(true) }
	sink := NewFrameSink(opts)
	t.Cleanup(func() { _ = sink.Flush() })
	sink.Start()

	for i := range 8 {
		sink.SendFrame(int64(i), sampleFrame(t), 1, 0, false)
		time.Sleep(40 * time.Millisecond)
	}
	if fired.Load() {
		t.Error("dead-man fired although every frame was accepted")
	}
}

func TestFrameSinkPostsFinalReport(t *testing.T) {
	collector := newFrameCollector(t, nil)
	group, err := NewReporterGroup([]ReportSpec{{Format: ReportFormatFrames, Path: collector.URL}}, ReporterOptions{})
	if err != nil {
		t.Fatalf("NewReporterGroup: %v", err)
	}
	sink := group.FrameSink()
	if sink == nil {
		t.Fatal("FrameSink() = nil for a frames spec")
	}
	sink.SetRunInfo("checkout-baseline", "Checkout", "v1.2.3")
	sink.Start()
	sink.SendFrame(0, sampleFrame(t), 1, 0, true)

	report := sampleLoadReport()
	group.SetLoadReport(&report)
	if err := group.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	bodies := collector.received()
	if len(bodies) != 2 {
		t.Fatalf("endpoint received %d posts, want the frame then the report", len(bodies))
	}
	var kind string
	_ = json.Unmarshal(bodies[0]["kind"], &kind)
	if kind != "frame" {
		t.Errorf("first post kind = %q, want frame (frames drain before the report)", kind)
	}
	_ = json.Unmarshal(bodies[1]["kind"], &kind)
	if kind != "report" {
		t.Errorf("second post kind = %q, want report", kind)
	}
	var loadReport struct {
		Flow     string `json:"flow"`
		Requests int64  `json:"requests"`
	}
	if err := json.Unmarshal(bodies[1]["load_report"], &loadReport); err != nil {
		t.Fatalf("load_report: %v", err)
	}
	if loadReport.Flow != "Checkout" || loadReport.Requests != 1024 {
		t.Errorf("load_report = %+v", loadReport)
	}
	var sent int64
	_ = json.Unmarshal(bodies[1]["frames_sent"], &sent)
	if sent != 1 {
		t.Errorf("frames_sent = %d, want 1", sent)
	}
}

func TestFrameSinkFinalReportFailureIsAnError(t *testing.T) {
	collector := newFrameCollector(t, func(int64) int { return http.StatusInternalServerError })
	opts := fastSinkOptions(collector.URL)
	opts.FinalTimeout = 300 * time.Millisecond
	sink := NewFrameSink(opts)
	report := sampleLoadReport()
	sink.SetLoadReport(&report)

	err := sink.Flush()
	if err == nil || !strings.Contains(err.Error(), "final report") {
		t.Errorf("Flush() error = %v, want a final report failure", err)
	}
}

func TestFrameSinkWithoutLoadReportPostsNothing(t *testing.T) {
	collector := newFrameCollector(t, nil)
	sink := NewFrameSink(fastSinkOptions(collector.URL))
	if err := sink.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if collector.attempts.Load() != 0 {
		t.Error("a functional run must not post anything to the frames endpoint")
	}
}
