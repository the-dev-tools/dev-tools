package yamlflowsimplev2

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

// update rewrites the .golden files with freshly generated output. Run with:
//
//	go test ./pkg/translate/yamlflowsimplev2/ -run TestGoldenRoundTrip -update
var update = flag.Bool("update", false, "rewrite .golden files")

const goldenDir = "testdata/golden"

// goldenCases returns the sorted list of golden case names, one per *.yaml
// fixture under testdata/golden (the paired *.golden snapshot is excluded
// since it doesn't carry the .yaml suffix).
func goldenCases(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(goldenDir)
	if err != nil {
		t.Fatalf("failed to read %s: %v", goldenDir, err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if name, ok := strings.CutSuffix(entry.Name(), ".yaml"); ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// exportAfterImport runs the full Import -> Export pipeline and returns the
// resulting YAML bytes.
func exportAfterImport(t *testing.T, in []byte) []byte {
	t.Helper()

	opts := GetDefaultOptions(idwrap.NewNow())
	bundle, err := ConvertSimplifiedYAML(in, opts)
	if err != nil {
		t.Fatalf("ConvertSimplifiedYAML failed: %v", err)
	}

	out, err := MarshalSimplifiedYAML(bundle)
	if err != nil {
		t.Fatalf("MarshalSimplifiedYAML failed: %v", err)
	}
	return out
}

func readGoldenFile(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return data
}

func writeGoldenFile(t *testing.T, path string, data []byte) {
	t.Helper()

	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

// diff renders a line-oriented listing of every line that differs between
// want and got, to keep golden mismatches readable in test output.
func diff(want, got []byte) string {
	wantLines := strings.Split(string(want), "\n")
	gotLines := strings.Split(string(got), "\n")

	maxLines := len(wantLines)
	if len(gotLines) > maxLines {
		maxLines = len(gotLines)
	}

	var b strings.Builder
	for i := 0; i < maxLines; i++ {
		var w, g string
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if w != g {
			fmt.Fprintf(&b, "line %d:\n  want: %q\n  got:  %q\n", i+1, w, g)
		}
	}
	if len(wantLines) != len(gotLines) {
		fmt.Fprintf(&b, "(want has %d lines, got has %d lines)\n", len(wantLines), len(gotLines))
	}
	return b.String()
}

// TestGoldenRoundTrip locks in the current Import -> Export behavior of the
// YAML flow contract, one fixture per family under testdata/golden. This is a
// characterization test: it snapshots today's behavior, bugs included (most
// notably, HTTP assertions are silently dropped on import as of this
// snapshot). Future deliberate behavior changes update the golden via
// -update, making every change visible as a diff.
//
// Each case asserts two things:
//
//  1. Stability: re-exporting an already-exported document is a no-op.
//     Import(yaml) -> Export -> yamlA; Import(yamlA) -> Export -> yamlB;
//     yamlA == yamlB.
//  2. No unintentional drift: the stable output matches the committed
//     .golden snapshot.
func TestGoldenRoundTrip(t *testing.T) {
	for _, name := range goldenCases(t) {
		t.Run(name, func(t *testing.T) {
			in := readGoldenFile(t, filepath.Join(goldenDir, name+".yaml"))

			first := exportAfterImport(t, in)
			second := exportAfterImport(t, first)
			if !bytes.Equal(first, second) {
				t.Fatalf("unstable round-trip for %s:\n%s", name, diff(first, second))
			}

			goldenPath := filepath.Join(goldenDir, name+".golden")
			if *update {
				writeGoldenFile(t, goldenPath, first)
			}

			want := readGoldenFile(t, goldenPath)
			if !bytes.Equal(first, want) {
				t.Fatalf("golden mismatch for %s (run with -update after intentional changes):\n%s", name, diff(want, first))
			}
		})
	}
}
