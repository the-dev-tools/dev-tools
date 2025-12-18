package reporter

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"the-dev-tools/cli/internal/model"

	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/model/mflow"
)

type FlowStartInfo struct {
	FlowID     string
	FlowName   string
	TotalNodes int
	NodeNames  []string
}

type NodeStatusEvent struct {
	FlowID   string
	FlowName string
	Status   runner.FlowNodeStatus
}

type Reporter interface {
	HandleFlowStart(info FlowStartInfo)
	HandleNodeStatus(event NodeStatusEvent)
	HandleFlowResult(result model.FlowRunResult)
	Flush() error
}

type ReporterGroup struct {
	reporters      []Reporter
	consoleEnabled bool
}

func (g *ReporterGroup) HandleFlowStart(info FlowStartInfo) {
	for _, reporter := range g.reporters {
		reporter.HandleFlowStart(info)
	}
}

func (g *ReporterGroup) HandleNodeStatus(event NodeStatusEvent) {
	for _, reporter := range g.reporters {
		reporter.HandleNodeStatus(event)
	}
}

func (g *ReporterGroup) HandleFlowResult(result model.FlowRunResult) {
	for _, reporter := range g.reporters {
		reporter.HandleFlowResult(result)
	}
}

func (g *ReporterGroup) Flush() error {
	var firstErr error
	for _, reporter := range g.reporters {
		if err := reporter.Flush(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (g *ReporterGroup) HasConsole() bool {
	return g.consoleEnabled
}

type ReportSpec struct {
	Format string
	Path   string
}

const (
	ReportFormatConsole = "console"
	ReportFormatJSON    = "json"
	ReportFormatJUnit   = "junit"
)

func ParseReportSpecs(values []string) ([]ReportSpec, error) {
	if len(values) == 0 {
		return []ReportSpec{{Format: ReportFormatConsole}}, nil
	}

	specs := make([]ReportSpec, 0, len(values))
	for _, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}

		format := trimmed
		var path string
		if idx := strings.Index(trimmed, ":"); idx >= 0 {
			format = strings.TrimSpace(trimmed[:idx])
			path = strings.TrimSpace(trimmed[idx+1:])
		}

		format = strings.ToLower(format)

		switch format {
		case ReportFormatConsole:
			if path != "" {
				return nil, fmt.Errorf("console reporter does not accept a path")
			}
		case ReportFormatJSON, ReportFormatJUnit:
			if path == "" {
				return nil, fmt.Errorf("%s reporter requires a file path", format)
			}
		default:
			return nil, fmt.Errorf("unsupported report format %q", format)
		}

		specs = append(specs, ReportSpec{Format: format, Path: path})
	}

	if len(specs) == 0 {
		specs = append(specs, ReportSpec{Format: ReportFormatConsole})
	}

	return specs, nil
}

func NewReporterGroup(specs []ReportSpec) (*ReporterGroup, error) {
	reporters := make([]Reporter, 0, len(specs))
	hasConsole := false

	for _, spec := range specs {
		var reporter Reporter
		switch spec.Format {
		case ReportFormatConsole:
			reporter = newConsoleReporter()
			hasConsole = true
		case ReportFormatJSON:
			reporter = newJSONReporter(spec.Path)
		case ReportFormatJUnit:
			reporter = newJUnitReporter(spec.Path)
		default:
			return nil, fmt.Errorf("unsupported reporter format %q", spec.Format)
		}

		reporters = append(reporters, reporter)
	}

	return &ReporterGroup{
		reporters:      reporters,
		consoleEnabled: hasConsole,
	}, nil
}

// Internal implementations below...

type jsonReporter struct {
	path    string
	mu      sync.Mutex
	results []model.FlowRunResult
}

func newJSONReporter(path string) Reporter {
	return &jsonReporter{path: path, results: make([]model.FlowRunResult, 0)}
}

func (j *jsonReporter) HandleFlowStart(info FlowStartInfo) {}

func (j *jsonReporter) HandleNodeStatus(event NodeStatusEvent) {}

func (j *jsonReporter) HandleFlowResult(result model.FlowRunResult) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.results = append(j.results, result)
}

func (j *jsonReporter) Flush() error {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.path == "" {
		return fmt.Errorf("json reporter missing output path")
	}

	if err := os.MkdirAll(filepath.Dir(j.path), 0o755); err != nil {
		return fmt.Errorf("creating json report directory: %w", err)
	}

	data, err := json.MarshalIndent(j.results, "", "  ")
	if err != nil {
		return fmt.Errorf("serializing json report: %w", err)
	}

	if err := os.WriteFile(j.path, data, 0o644); err != nil {
		return fmt.Errorf("writing json report: %w", err)
	}

	return nil
}

type junitReporter struct {
	path    string
	mu      sync.Mutex
	results []model.FlowRunResult
}

func newJUnitReporter(path string) Reporter {
	return &junitReporter{path: path, results: make([]model.FlowRunResult, 0)}
}

func (j *junitReporter) HandleFlowStart(info FlowStartInfo) {}

func (j *junitReporter) HandleNodeStatus(event NodeStatusEvent) {}

func (j *junitReporter) HandleFlowResult(result model.FlowRunResult) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.results = append(j.results, result)
}

type junitTestSuites struct {
	XMLName xml.Name         `xml:"testsuites"`
	Suites  []junitTestSuite `xml:"testsuite"`
}

type junitTestSuite struct {
	XMLName  xml.Name        `xml:"testsuite"`
	Name     string          `xml:"name,attr"`
	Tests    int             `xml:"tests,attr"`
	Failures int             `xml:"failures,attr"`
	Time     string          `xml:"time,attr"`
	Cases    []junitTestCase `xml:"testcase"`
}

type junitTestCase struct {
	XMLName xml.Name      `xml:"testcase"`
	Name    string        `xml:"name,attr"`
	Time    string        `xml:"time,attr"`
	Failure *junitFailure `xml:"failure,omitempty"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr,omitempty"`
	Data    string `xml:",chardata"`
}

func (j *junitReporter) Flush() error {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.path == "" {
		return fmt.Errorf("junit reporter missing output path")
	}

	suites := make([]junitTestSuite, 0, len(j.results))
	for _, result := range j.results {
		suite := junitTestSuite{
			Name:     result.FlowName,
			Tests:    len(result.Nodes),
			Failures: 0,
			Time:     fmt.Sprintf("%.6f", result.Duration.Seconds()),
			Cases:    make([]junitTestCase, 0, len(result.Nodes)),
		}

		for _, node := range result.Nodes {
			testCase := junitTestCase{
				Name: node.Name,
				Time: fmt.Sprintf("%.6f", node.Duration.Seconds()),
			}

			if strings.EqualFold(node.State, mflow.StringNodeState(mflow.NODE_STATE_SUCCESS)) {
				// no failure
			} else {
				failureType := node.State
				if failureType == "" {
					failureType = "Failure"
				}
				testCase.Failure = &junitFailure{
					Message: failureType,
					Type:    failureType,
					Data:    node.Error,
				}
				suite.Failures++
			}

			suite.Cases = append(suite.Cases, testCase)
		}

		suites = append(suites, suite)
	}

	output := junitTestSuites{Suites: suites}
	data, err := xml.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("serializing junit report: %w", err)
	}

	header := []byte(xml.Header)
	data = append(header, data...)

	if err := os.MkdirAll(filepath.Dir(j.path), 0o755); err != nil {
		return fmt.Errorf("creating junit report directory: %w", err)
	}

	if err := os.WriteFile(j.path, data, 0o644); err != nil {
		return fmt.Errorf("writing junit report: %w", err)
	}

	return nil
}

type consoleReporter struct {
	mu    sync.Mutex
	flows map[string]*consoleFlowState
}

type consoleFlowState struct {
	rowFormat    string
	topBorder    string
	separator    string
	totalNodes   int
	successCount int
}

func newConsoleReporter() Reporter {
	return &consoleReporter{
		flows: make(map[string]*consoleFlowState),
	}
}

func (c *consoleReporter) flowKey(info FlowStartInfo) string {
	if info.FlowID != "" {
		return info.FlowID
	}
	return info.FlowName
}

func (c *consoleReporter) HandleFlowStart(info FlowStartInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	maxStepNameLen := len("Step")
	for _, name := range info.NodeNames {
		if len(name) > maxStepNameLen {
			maxStepNameLen = len(name)
		}
	}

	tableWidth := 2 + 20 + 3 + maxStepNameLen + 3 + 8 + 3 + 11 + 2
	topBottomBorder := strings.Repeat("=", tableWidth)
	separatorBorder := strings.Repeat("─", tableWidth)
	tableRowFmt := fmt.Sprintf("| %%-20s | %%-%ds | %%-8s | %%-11s |\n", maxStepNameLen)

	displayTitleContent := fmt.Sprintf(" Flow: %s", info.FlowName)
	maxContentWidthInTitle := tableWidth - 2
	if len(displayTitleContent) > maxContentWidthInTitle {
		if maxContentWidthInTitle > 3 {
			displayTitleContent = displayTitleContent[:maxContentWidthInTitle-3] + "..."
		} else if maxContentWidthInTitle >= 0 {
			displayTitleContent = displayTitleContent[:maxContentWidthInTitle]
		} else {
			displayTitleContent = ""
		}
	}

	paddingLength := maxContentWidthInTitle - len(displayTitleContent)
	if paddingLength < 0 {
		paddingLength = 0
	}

	fmt.Println(topBottomBorder)
	fmt.Printf("|%s%s|\n", displayTitleContent, strings.Repeat(" ", paddingLength))
	fmt.Println(separatorBorder)
	fmt.Printf(tableRowFmt, "Timestamp", "Step", "Duration", "Status")
	fmt.Println(separatorBorder)

	key := c.flowKey(info)
	c.flows[key] = &consoleFlowState{
		rowFormat:    tableRowFmt,
		topBorder:    topBottomBorder,
		separator:    separatorBorder,
		totalNodes:   info.TotalNodes,
		successCount: 0,
	}
}

func (c *consoleReporter) HandleNodeStatus(event NodeStatusEvent) {
	if event.Status.State == mflow.NODE_STATE_RUNNING {
		return
	}

	c.mu.Lock()
	state, ok := c.flows[c.flowKey(FlowStartInfo{FlowID: event.FlowID, FlowName: event.FlowName})]
	c.mu.Unlock()
	if !ok {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	statusStr := mflow.StringNodeStateWithIcons(event.Status.State)
	fmt.Printf(state.rowFormat, timestamp, event.Status.Name, FormatDuration(event.Status.RunDuration), statusStr)

	if event.Status.State == mflow.NODE_STATE_SUCCESS {
		c.mu.Lock()
		state.successCount++
		c.mu.Unlock()
	}
}

func (c *consoleReporter) HandleFlowResult(result model.FlowRunResult) {
	key := c.flowKey(FlowStartInfo{FlowID: result.FlowID, FlowName: result.FlowName})

	c.mu.Lock()
	state, ok := c.flows[key]
	if ok {
		delete(c.flows, key)
	}
	c.mu.Unlock()

	if !ok {
		return
	}

	fmt.Println(state.topBorder)
	fmt.Printf("Flow Duration: %v | Steps: %d/%d Successful\n", result.Duration, state.successCount, state.totalNodes)
}

func (c *consoleReporter) Flush() error {
	return nil
}

// FormatDuration formats a duration for display
func FormatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%.2fµs", float64(d.Nanoseconds())/1000)
	} else if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d.Nanoseconds())/1000000)
	} else if d < time.Minute {
		return fmt.Sprintf("%.2fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.2fm", d.Minutes())
	}
	return fmt.Sprintf("%.2fh", d.Hours())
}
