package aicheck

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// KeyEnvs are the variables a judge key is read from, in order.
func KeyEnvs(cfg JudgeConfig) []string {
	if cfg.APIKeyEnv != "" {
		return []string{cfg.APIKeyEnv}
	}
	if cfg.Provider == ProviderOpenAI {
		return []string{"STRESSEUR_JUDGE_API_KEY", "OPENAI_API_KEY"}
	}
	return []string{"STRESSEUR_JUDGE_API_KEY", "ANTHROPIC_API_KEY"}
}

// NoKeyReason is why judge checks were skipped without a key.
func NoKeyReason(cfg JudgeConfig) string {
	envs := KeyEnvs(cfg)
	if len(envs) == 2 {
		envs[0], envs[1] = envs[1], envs[0] // the provider's own name first
	}
	return "no judge API key: set " + strings.Join(envs, " or ")
}

// Stats is the judge's work over a command.
type Stats struct {
	Calls        int // API calls (borderline samples included)
	Judged       int // verdicts that needed calls
	Cached       int // verdicts from the cache or an identical output this run
	Skipped      int
	InputTokens  int
	OutputTokens int
	// Latencies are each verdict's wall time (its calls, samples in parallel).
	Latencies []time.Duration
	// Elapsed is the wall time of the judging phases.
	Elapsed time.Duration
	// NoKey are the reasons judge checks were skipped for a missing key, once each.
	NoKey []string
}

// Evaluator runs the checks of flows after their timed runs. One per command.
type Evaluator struct {
	Env         map[string]string
	HTTP        *http.Client
	Cache       *Cache
	Concurrency int
	Backoff     func(n int) time.Duration

	mu       sync.Mutex
	clients  map[string]*Client
	inflight map[string]*call
	Stats    Stats
}

type call struct {
	done    chan struct{}
	verdict Verdict
	err     error
}

func (e *Evaluator) client(cfg JudgeConfig) (*Client, string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	key := cfg.Provider + "\x00" + cfg.Model + "\x00" + cfg.BaseURL + "\x00" + cfg.APIKeyEnv
	if c, ok := e.clients[key]; ok {
		return c, ""
	}
	apiKey := ""
	for _, name := range KeyEnvs(cfg) {
		if v := strings.TrimSpace(e.Env[name]); v != "" {
			apiKey = v
			break
		}
	}
	if apiKey == "" {
		reason := NoKeyReason(cfg)
		found := false
		for _, r := range e.Stats.NoKey {
			found = found || r == reason
		}
		if !found {
			e.Stats.NoKey = append(e.Stats.NoKey, reason)
		}
		return nil, reason
	}
	h := e.HTTP
	if h == nil {
		h = &http.Client{Timeout: 60 * time.Second}
	}
	c := &Client{Config: cfg, APIKey: apiKey, HTTP: h, Backoff: e.Backoff}
	if e.clients == nil {
		e.clients = map[string]*Client{}
	}
	e.clients[key] = c
	return c, ""
}

// FlowResult is a flow's checks: per run (parallel to the runs given), per step, the results;
// per step the summaries.
type FlowResult struct {
	Runs      []map[string][]Result
	Summaries map[string][]Summary
}

type job struct {
	run, step int
	spec      *Spec
	input     string
	output    string
}

// EvaluateFlow runs every check of a flow over its measured runs: the deterministic ones
// inline, the judge calls in parallel (Concurrency at a time), through the cache.
func (e *Evaluator) EvaluateFlow(ctx context.Context, fl *Flow, runs []Run) FlowResult {
	out := FlowResult{Runs: make([]map[string][]Result, len(runs)), Summaries: map[string][]Summary{}}
	var jobs []job
	slots := map[[2]int]int{} // (run, step) → index of the judge result in out.Runs[run][step]
	for ri, run := range runs {
		out.Runs[ri] = map[string][]Result{}
		for si, name := range fl.Steps {
			sr, ok := run.Steps[name]
			if !ok || !sr.Ran {
				continue
			}
			spec := fl.Specs[name]
			results := Deterministic(spec, name, run)
			if spec.Judge != nil {
				model := fl.Judge.Model
				r := Result{Kind: KindJudge, JudgeModel: &model}
				input, missIn := Render(orDefault(spec.Judge.Input, "{{ request.body }}"), name, run)
				output, missOut := Render(orDefault(spec.Judge.Output, "{{ response.body }}"), name, run)
				switch {
				case !hasResponse(sr):
					r.Reason = "no response"
				case missOut != "":
					r.Reason = "output: " + missOut + " not found"
				case missIn != "" && spec.Judge.Input != "":
					r.Reason = "input: " + missIn + " not found"
				default:
					if missIn != "" {
						input = "" // a request without a body
					}
					slots[[2]int{ri, si}] = len(results)
					jobs = append(jobs, job{run: ri, step: si, spec: spec, input: input, output: output})
				}
				results = append(results, r)
			}
			out.Runs[ri][name] = results
		}
	}

	if len(jobs) > 0 {
		start := time.Now()
		n := e.Concurrency
		if n <= 0 {
			n = 8
		}
		sem := make(chan struct{}, n)
		var wg sync.WaitGroup
		results := make([]Result, len(jobs))
		for i, j := range jobs {
			wg.Add(1)
			go func(i int, j job) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				results[i] = e.judge(ctx, fl.Judge, j)
			}(i, j)
		}
		wg.Wait()
		e.mu.Lock()
		e.Stats.Elapsed += time.Since(start)
		e.mu.Unlock()
		for i, j := range jobs {
			name := fl.Steps[j.step]
			out.Runs[j.run][name][slots[[2]int{j.run, j.step}]] = results[i]
		}
	}

	for _, name := range fl.Steps {
		var perRun [][]Result
		for _, r := range out.Runs {
			if rs, ok := r[name]; ok {
				perRun = append(perRun, rs)
			}
		}
		out.Summaries[name] = Summarize(fl.Specs[name], fl.Judge.Model, fl.Quality, perRun)
	}
	return out
}

func orDefault(s, d string) string {
	if strings.TrimSpace(s) == "" {
		return d
	}
	return s
}

func (e *Evaluator) judge(ctx context.Context, cfg JudgeConfig, j job) Result {
	model := cfg.Model
	r := Result{Kind: KindJudge, JudgeModel: &model}
	key := CacheKey(cfg, j.spec.Judge, j.input, j.output)
	verdictResult := func(v Verdict, cached bool) Result {
		r.Score = ptr(v.Score)
		r.Passed = v.Score >= j.spec.Judge.MinScore
		r.Reason = Clip(v.Reason)
		r.Cached = cached
		return r
	}
	if e.Cache != nil {
		if hit, ok := e.Cache.Get(key); ok {
			e.count(func(s *Stats) { s.Cached++ })
			return verdictResult(Verdict{Score: hit.Score, Reason: hit.Reason}, true)
		}
	}
	// One call per distinct output: the others wait for it and count as cached.
	e.mu.Lock()
	if e.inflight == nil {
		e.inflight = map[string]*call{}
	}
	if c, ok := e.inflight[key]; ok {
		e.mu.Unlock()
		<-c.done
		if c.err != nil {
			r.Skipped, r.Reason = true, Clip("judge error: "+c.err.Error())
			var nk *noKeyError
			if errors.As(c.err, &nk) {
				r.Reason = nk.reason
			}
			e.count(func(s *Stats) { s.Skipped++ })
			return r
		}
		e.count(func(s *Stats) { s.Cached++ })
		return verdictResult(c.verdict, true)
	}
	c := &call{done: make(chan struct{})}
	e.inflight[key] = c
	e.mu.Unlock()
	defer close(c.done)

	client, noKey := e.client(cfg)
	if client == nil {
		c.err = &noKeyError{noKey}
		r.Skipped, r.Reason = true, noKey
		e.count(func(s *Stats) { s.Skipped++ })
		return r
	}
	start := time.Now()
	v, err := client.Judge(ctx, j.spec.Judge, j.input, j.output)
	took := time.Since(start)
	e.count(func(s *Stats) {
		s.Calls += v.Calls
		s.InputTokens += v.InputTokens
		s.OutputTokens += v.OutputTokens
	})
	r.Calls, r.InputTokens, r.OutputTokens = v.Calls, v.InputTokens, v.OutputTokens
	if err != nil {
		c.err = err
		r.Skipped, r.Reason = true, Clip("judge error: "+err.Error())
		e.count(func(s *Stats) { s.Skipped++ })
		return r
	}
	c.verdict = v
	e.count(func(s *Stats) {
		s.Judged++
		s.Latencies = append(s.Latencies, took)
	})
	if e.Cache != nil {
		e.Cache.Put(key, CacheEntry{Score: v.Score, Reason: v.Reason, Model: cfg.Model})
	}
	res := verdictResult(v, false)
	res.Calls, res.InputTokens, res.OutputTokens = v.Calls, v.InputTokens, v.OutputTokens
	return res
}

type noKeyError struct{ reason string }

func (e *noKeyError) Error() string { return e.reason }

func (e *Evaluator) count(f func(*Stats)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	f(&e.Stats)
}

// Statuses, worst last.
const (
	StatusPassed  = "passed"
	StatusSkipped = "skipped"
	StatusWarn    = "warn"
	StatusFailed  = "failed"
)

var statusRank = map[string]int{StatusPassed: 1, StatusSkipped: 2, StatusWarn: 3, StatusFailed: 4}

// Worst is the worst of some statuses; "" when there are none.
func Worst(statuses ...string) string {
	w := ""
	for _, s := range statuses {
		if statusRank[s] > statusRank[w] {
			w = s
		}
	}
	return w
}

// Summary is one check over a step's measured runs (ingest-v1 steps[].checks[]).
type Summary struct {
	Kind              string   `json:"kind"`
	Runs              int      `json:"runs"`
	Passed            int      `json:"passed"`
	Skipped           int      `json:"skipped"`
	PassRate          *float64 `json:"pass_rate"`
	Status            string   `json:"status"`
	FailBelow         *float64 `json:"fail_below"`
	Threshold         *float64 `json:"threshold"`
	MeanScore         *float64 `json:"mean_score"`
	JudgeModel        *string  `json:"judge_model"`
	Criteria          *string  `json:"criteria"`
	Reason            string   `json:"reason"`
	JudgeCalls        int      `json:"judge_calls"`
	JudgeInputTokens  int      `json:"judge_input_tokens"`
	JudgeOutputTokens int      `json:"judge_output_tokens"`
	JudgeCached       int      `json:"-"`

	// Values are the measured latency or token values, for the console's p95.
	Values []float64 `json:"-"`
}

func round(v float64, places int) float64 {
	p := 1.0
	for range places {
		p *= 10
	}
	return float64(int64(v*p+0.5)) / p
}

// Summarize computes each declared check's pass rate over a step's runs (one []Result per run).
func Summarize(spec *Spec, judgeModel string, q Quality, perRun [][]Result) []Summary {
	var out []Summary
	for _, kind := range Kinds {
		if !declares(spec, kind) {
			continue
		}
		s := Summary{Kind: kind, FailBelow: q.FailBelow}
		switch kind {
		case KindLatency:
			s.Threshold = ptr(*spec.MaxLatencyMS)
		case KindTTFT:
			s.Threshold = ptr(*spec.MaxTTFTMS)
		case KindJudge:
			s.Threshold = ptr(spec.Judge.MinScore)
			s.JudgeModel = &judgeModel
			c := strings.TrimSpace(spec.Judge.Criteria)
			s.Criteria = &c
		}
		var scoreSum float64
		var worst *Result
		skipReason := ""
		for _, rs := range perRun {
			for i := range rs {
				r := rs[i]
				if r.Kind != kind {
					continue
				}
				s.JudgeCalls += r.Calls
				s.JudgeInputTokens += r.InputTokens
				s.JudgeOutputTokens += r.OutputTokens
				if r.Cached {
					s.JudgeCached++
				}
				if r.Skipped {
					s.Skipped++
					if skipReason == "" {
						skipReason = r.Reason
					}
					continue
				}
				s.Runs++
				if kind == KindTokens && s.Threshold == nil && r.Limit != nil {
					s.Threshold = ptr(*r.Limit)
				}
				if r.Value != nil {
					s.Values = append(s.Values, *r.Value)
				}
				if r.Score != nil {
					scoreSum += *r.Score
				}
				if r.Passed {
					s.Passed++
					continue
				}
				if worst == nil || (r.Score != nil && worst.Score != nil && *r.Score < *worst.Score) {
					worst = &r
				}
			}
		}
		if s.Runs > 0 {
			s.PassRate = ptr(round(float64(s.Passed)/float64(s.Runs), 4))
			if kind == KindJudge {
				s.MeanScore = ptr(round(scoreSum/float64(s.Runs), 2))
			}
		}
		switch {
		case s.Runs == 0:
			s.Status = StatusSkipped
			s.Reason = skipReason
		case s.Passed == s.Runs:
			s.Status = StatusPassed
		case q.FailBelow != nil && *s.PassRate < *q.FailBelow:
			s.Status = StatusFailed
		default:
			s.Status = StatusWarn
		}
		if worst != nil {
			s.Reason = worst.Reason
		}
		out = append(out, s)
	}
	return out
}

func declares(spec *Spec, kind string) bool {
	switch kind {
	case KindSchema:
		return spec.Schema != nil
	case KindLatency:
		return spec.MaxLatencyMS != nil
	case KindTTFT:
		return spec.MaxTTFTMS != nil
	case KindTokens:
		return spec.HasTokens()
	case KindJudge:
		return spec.Judge != nil
	}
	return false
}

// Percentile is the nearest-rank percentile of values (p in (0, 100]).
func Percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	s := append([]float64(nil), values...)
	sort.Float64s(s)
	rank := int(p/100*float64(len(s)) + 0.999999)
	if rank < 1 {
		rank = 1
	}
	if rank > len(s) {
		rank = len(s)
	}
	return s[rank-1]
}
