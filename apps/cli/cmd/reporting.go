package cmd

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/model/mnnode"
)

type IterationContextResult struct {
	IterationPath  []int    `json:"iteration_path,omitempty"`
	ExecutionIndex int      `json:"execution_index,omitempty"`
	ParentNodes    []string `json:"parent_nodes,omitempty"`
}

type NodeRunResult struct {
	NodeID           string                  `json:"node_id"`
	ExecutionID      string                  `json:"execution_id"`
	Name             string                  `json:"name"`
	State            string                  `json:"state"`
	Duration         time.Duration           `json:"duration"`
	Error            string                  `json:"error,omitempty"`
	IterationContext *IterationContextResult `json:"iteration_context,omitempty"`
}

type FlowRunResult struct {
	FlowID   string          `json:"flow_id"`
	FlowName string          `json:"flow_name"`
	Started  time.Time       `json:"started_at"`
	Duration time.Duration   `json:"duration"`
	Status   string          `json:"status"`
	Error    string          `json:"error,omitempty"`
	Nodes    []NodeRunResult `json:"nodes"`
}

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
	HandleFlowResult(result FlowRunResult)
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

func (g *ReporterGroup) HandleFlowResult(result FlowRunResult) {
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

type reportSpec struct {
	format string
	path   string
}

const (
	reportFormatConsole = "console"
	reportFormatJSON    = "json"
	reportFormatJUnit   = "junit"
)

func parseReportSpecs(values []string) ([]reportSpec, error) {
	if len(values) == 0 {
		return []reportSpec{{format: reportFormatConsole}}, nil
	}

	specs := make([]reportSpec, 0, len(values))
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
		case reportFormatConsole:
			if path != "" {
				return nil, fmt.Errorf("console reporter does not accept a path")
			}
		case reportFormatJSON, reportFormatJUnit:
			if path == "" {
				return nil, fmt.Errorf("%s reporter requires a file path", format)
			}
		default:
			return nil, fmt.Errorf("unsupported report format %q", format)
		}

		specs = append(specs, reportSpec{format: format, path: path})
	}

	if len(specs) == 0 {
		specs = append(specs, reportSpec{format: reportFormatConsole})
	}

	return specs, nil
}

func newReporterGroup(specs []reportSpec) (*ReporterGroup, error) {
	reporters := make([]Reporter, 0, len(specs))
	hasConsole := false

	for _, spec := range specs {
		var reporter Reporter
		switch spec.format {
		case reportFormatConsole:
			reporter = newConsoleReporter()
			hasConsole = true
		case reportFormatJSON:
			reporter = newJSONReporter(spec.path)
		case reportFormatJUnit:
			reporter = newJUnitReporter(spec.path)
		default:
			return nil, fmt.Errorf("unsupported reporter format %q", spec.format)
		}

		reporters = append(reporters, reporter)
	}

	return &ReporterGroup{
		reporters:      reporters,
		consoleEnabled: hasConsole,
	}, nil
}

type jsonReporter struct {
	path    string
	mu      sync.Mutex
	results []FlowRunResult
}

func newJSONReporter(path string) Reporter {
	return &jsonReporter{path: path, results: make([]FlowRunResult, 0)}
}

func (j *jsonReporter) HandleFlowStart(info FlowStartInfo) {}

func (j *jsonReporter) HandleNodeStatus(event NodeStatusEvent) {}

func (j *jsonReporter) HandleFlowResult(result FlowRunResult) {
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
	results []FlowRunResult
}

func newJUnitReporter(path string) Reporter {
	return &junitReporter{path: path, results: make([]FlowRunResult, 0)}
}

func (j *junitReporter) HandleFlowStart(info FlowStartInfo) {}

func (j *junitReporter) HandleNodeStatus(event NodeStatusEvent) {}

func (j *junitReporter) HandleFlowResult(result FlowRunResult) {
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

			if strings.EqualFold(node.State, mnnode.StringNodeState(mnnode.NODE_STATE_SUCCESS)) {
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
	if event.Status.State == mnnode.NODE_STATE_RUNNING {
		return
	}

	c.mu.Lock()
	state, ok := c.flows[c.flowKey(FlowStartInfo{FlowID: event.FlowID, FlowName: event.FlowName})]
	c.mu.Unlock()
	if !ok {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	statusStr := mnnode.StringNodeStateWithIcons(event.Status.State)
	fmt.Printf(state.rowFormat, timestamp, event.Status.Name, formatDuration(event.Status.RunDuration), statusStr)

	if event.Status.State == mnnode.NODE_STATE_SUCCESS {
		c.mu.Lock()
		state.successCount++
		c.mu.Unlock()
	}
}

func (c *consoleReporter) HandleFlowResult(result FlowRunResult) {
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

func buildNodeRunResult(status runner.FlowNodeStatus) NodeRunResult {
	nodeResult := NodeRunResult{
		NodeID:      status.NodeID.String(),
		ExecutionID: status.ExecutionID.String(),
		Name:        status.Name,
		State:       mnnode.StringNodeState(status.State),
		Duration:    status.RunDuration,
	}

	if status.Error != nil {
		nodeResult.Error = status.Error.Error()
	}

	if status.IterationContext != nil {
		ctx := &IterationContextResult{
			IterationPath:  append([]int(nil), status.IterationContext.IterationPath...),
			ExecutionIndex: status.IterationContext.ExecutionIndex,
		}
		if len(status.IterationContext.ParentNodes) > 0 {
			parents := make([]string, 0, len(status.IterationContext.ParentNodes))
			for _, parent := range status.IterationContext.ParentNodes {
				parents = append(parents, parent.String())
			}
			ctx.ParentNodes = parents
		}
		nodeResult.IterationContext = ctx
	}

	return nodeResult
}
