package runner

import (
	"strings"
	"testing"
)

// namesOf extracts the flow names from a sorted run: block, for compact
// order assertions.
func namesOf(entries []runEntry) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.flowName
	}
	return names
}

// TestTopoSortRunEntries exercises topoSortRunEntries directly against
// hand-built runEntry values: no YAML, no flow execution, no I/O. The
// integration-shaped coverage (actual RunMultipleFlows calls against real
// flows and a mock HTTP server) lives in runner_test.go; these cases pin the
// sort/cycle-detection algorithm itself, which was previously only exercised
// indirectly through that heavier seam.
func TestTopoSortRunEntries(t *testing.T) {
	tests := []struct {
		name    string
		entries []runEntry
		// wantOrder is checked when wantErrSubstrs is empty.
		wantOrder []string
		// wantErrSubstrs are all required to appear in the returned error;
		// when non-empty, an error is required and wantOrder is ignored.
		wantErrSubstrs []string
	}{
		{
			// Kahn's algorithm processes the initial "ready" set (every
			// flow with no dependencies) in a FIFO queue seeded in
			// declaration order, so with no edges to reorder anything, the
			// output is exactly the input order. This is the tie-break
			// that makes the sort deterministic across runs of the same
			// file.
			name: "3+ dependency-free flows preserve declaration order",
			entries: []runEntry{
				{flowName: "Charlie"},
				{flowName: "Alpha"},
				{flowName: "Bravo"},
			},
			wantOrder: []string{"Charlie", "Alpha", "Bravo"},
		},
		{
			name: "self-dependency is reported as a cycle naming the flow",
			entries: []runEntry{
				{flowName: "A", dependsOn: []string{"A"}},
			},
			wantErrSubstrs: []string{"dependency cycle in run block", "A"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := topoSortRunEntries(tt.entries)

			if len(tt.wantErrSubstrs) > 0 {
				if err == nil {
					t.Fatalf("expected an error, got order %v", namesOf(got))
				}
				for _, substr := range tt.wantErrSubstrs {
					if !strings.Contains(err.Error(), substr) {
						t.Errorf("error %q does not contain %q", err.Error(), substr)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			gotOrder := namesOf(got)
			if len(gotOrder) != len(tt.wantOrder) {
				t.Fatalf("order = %v, want %v", gotOrder, tt.wantOrder)
			}
			for i, want := range tt.wantOrder {
				if gotOrder[i] != want {
					t.Errorf("order = %v, want %v", gotOrder, tt.wantOrder)
					break
				}
			}
		})
	}
}

// TestParseRunEntries_DuplicateFlowName covers a defect found in review:
// topoSortRunEntries's byName/inDegree/declOrder maps are keyed by flow
// name, so a run: block that declares the same flow twice silently
// collapses onto the last-declared entry in those maps while `names` (a
// plain slice) still holds both occurrences. Depending on the dependency
// shape this either silently drops one declared run (no error at all, the
// entries collapsing case) or - the case pinned here - makes findCycle's
// "remaining" set land empty, producing a bare "dependency cycle in run
// block: " error that names nothing useful.
//
// parseRunEntries now rejects a duplicate flow name outright, before
// topoSortRunEntries ever runs, so every duplicate produces the same clear,
// named error instead of either failure mode.
func TestParseRunEntries_DuplicateFlowName(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "plain duplicate, no dependencies",
			yaml: "run:\n  - flow: A\n  - flow: A\n",
		},
		{
			// The specific repro from review: both "A" entries depend on
			// X, so dependents["X"] lists "A" twice for an inDegree
			// counter that only starts at 1 (the last-declared "A"
			// entry's dependency count). Resolving X decrements it twice,
			// overshooting to -1 - inDegree never reads as "still
			// pending" (>0), so findCycle's remaining set is empty and the
			// old code produced "dependency cycle in run block: " with
			// nothing after the colon.
			name: "duplicate that used to blank out the cycle message",
			yaml: "run:\n  - flow: X\n  - flow: A\n    depends_on: X\n  - flow: A\n    depends_on: X\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseRunEntries([]byte(tt.yaml))
			if err == nil {
				t.Fatal("expected an error for a duplicate flow name, got nil")
			}
			if !strings.Contains(err.Error(), "duplicate") {
				t.Errorf("error %q does not mention 'duplicate'", err.Error())
			}
			if !strings.Contains(err.Error(), `"A"`) {
				t.Errorf("error %q does not name the duplicated flow %q", err.Error(), "A")
			}
			if strings.Contains(err.Error(), "dependency cycle in run block:") {
				t.Errorf("error regressed to the blank cycle message: %q", err.Error())
			}
		})
	}
}
