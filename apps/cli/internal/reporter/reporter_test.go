package reporter

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"
	"time"

	"the-dev-tools/cli/internal/model"
	"the-dev-tools/server/pkg/model/mflow"
)

func TestParseReportSpecsDefault(t *testing.T) {
	specs, err := ParseReportSpecs(nil)
	if err != nil {
		t.Fatalf("ParseReportSpecs returned error: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}
	if specs[0].Format != ReportFormatConsole {
		t.Fatalf("expected console format, got %s", specs[0].Format)
	}
}

func TestParseReportSpecsRequiresPath(t *testing.T) {
	if _, err := ParseReportSpecs([]string{"json"}); err == nil {
		t.Fatalf("expected error when path missing for json reporter")
	}

	if _, err := ParseReportSpecs([]string{"console:/tmp/out"}); err == nil {
		t.Fatalf("expected error when console reporter includes path")
	}
}

func TestJSONReporterFlush(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "report.json")

	specs := []ReportSpec{{Format: ReportFormatJSON, Path: outputPath}}
	group, err := NewReporterGroup(specs)
	if err != nil {
		t.Fatalf("failed to create reporter group: %v", err)
	}

	sample := model.FlowRunResult{
		FlowID:   "01HZXPM0Q8",
		FlowName: "Sample",
		Started:  time.Unix(0, 0).UTC(),
		Duration: time.Second,
		Status:   "success",
		Nodes: []model.NodeRunResult{
			{
				NodeID:      "Node1",
				ExecutionID: "Exec1",
				Name:        "Step 1",
				State:       mflow.StringNodeState(mflow.NODE_STATE_SUCCESS),
				Duration:    50 * time.Millisecond,
			},
			{
				NodeID:      "Node2",
				ExecutionID: "Exec2",
				Name:        "Step 2",
				State:       mflow.StringNodeState(mflow.NODE_STATE_FAILURE),
				Duration:    25 * time.Millisecond,
				Error:       "boom",
			},
		},
	}

	group.HandleFlowResult(sample)
	if err := group.Flush(); err != nil {
		t.Fatalf("flush failed: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read report: %v", err)
	}

	var results []model.FlowRunResult
	if err := json.Unmarshal(data, &results); err != nil {
		t.Fatalf("failed to unmarshal report: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 flow result, got %d", len(results))
	}

	got := results[0]
	if got.FlowID != sample.FlowID || got.FlowName != sample.FlowName {
		t.Fatalf("unexpected flow metadata: %+v", got)
	}
	if got.Status != sample.Status {
		t.Fatalf("expected status %s, got %s", sample.Status, got.Status)
	}
	if got.Duration != sample.Duration {
		t.Fatalf("expected duration %s, got %s", sample.Duration, got.Duration)
	}
	if len(got.Nodes) != len(sample.Nodes) {
		t.Fatalf("expected %d nodes, got %d", len(sample.Nodes), len(got.Nodes))
	}
	if got.Nodes[1].Error != "boom" {
		t.Fatalf("expected error 'boom', got %q", got.Nodes[1].Error)
	}
}

func TestJUnitReporterFlush(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "report.xml")

	specs := []ReportSpec{{Format: ReportFormatJUnit, Path: outputPath}}
	group, err := NewReporterGroup(specs)
	if err != nil {
		t.Fatalf("failed to create reporter group: %v", err)
	}

	sample := model.FlowRunResult{
		FlowID:   "01HZXPM0Q8",
		FlowName: "Sample",
		Started:  time.Unix(0, 0).UTC(),
		Duration: 1500 * time.Millisecond,
		Status:   "failed",
		Nodes: []model.NodeRunResult{
			{
				NodeID:      "Node1",
				ExecutionID: "Exec1",
				Name:        "Step 1",
				State:       mflow.StringNodeState(mflow.NODE_STATE_SUCCESS),
				Duration:    100 * time.Millisecond,
			},
			{
				NodeID:      "Node2",
				ExecutionID: "Exec2",
				Name:        "Step 2",
				State:       mflow.StringNodeState(mflow.NODE_STATE_FAILURE),
				Duration:    200 * time.Millisecond,
				Error:       "fail",
			},
		},
	}

	group.HandleFlowResult(sample)
	if err := group.Flush(); err != nil {
		t.Fatalf("flush failed: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read report: %v", err)
	}

	var suites junitTestSuites
	if err := xml.Unmarshal(data, &suites); err != nil {
		t.Fatalf("failed to unmarshal junit report: %v", err)
	}

	if len(suites.Suites) != 1 {
		t.Fatalf("expected 1 suite, got %d", len(suites.Suites))
	}
	suite := suites.Suites[0]
	if suite.Failures != 1 {
		t.Fatalf("expected 1 failure, got %d", suite.Failures)
	}
	if len(suite.Cases) != len(sample.Nodes) {
		t.Fatalf("expected %d cases, got %d", len(sample.Nodes), len(suite.Cases))
	}
	if suite.Cases[1].Failure == nil {
		t.Fatalf("expected failure entry for second node")
	}
	if suite.Cases[1].Failure.Data != "fail" {
		t.Fatalf("expected failure message 'fail', got %q", suite.Cases[1].Failure.Data)
	}
}
