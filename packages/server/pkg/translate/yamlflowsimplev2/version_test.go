package yamlflowsimplev2

import (
	"strings"
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

const minimalVersionTestYAML = `
workspace_name: Version Test
flows:
  - name: VersionFlow
    steps:
      - manual_start:
          name: Start
`

// TestVersionField locks in the yamlflow schema versioning contract: an
// explicit supported version imports cleanly, an absent version defaults to
// the current version, an unsupported (too new) version is rejected with a
// specific error, and export always stamps the current version as the first
// key of the document.
func TestVersionField(t *testing.T) {
	t.Run("version 2 succeeds", func(t *testing.T) {
		yamlData := "version: 2\n" + minimalVersionTestYAML
		opts := GetDefaultOptions(idwrap.NewNow())
		if _, err := ConvertSimplifiedYAML([]byte(yamlData), opts); err != nil {
			t.Fatalf("expected version 2 to succeed, got error: %v", err)
		}
	})

	t.Run("absent version succeeds", func(t *testing.T) {
		opts := GetDefaultOptions(idwrap.NewNow())
		if _, err := ConvertSimplifiedYAML([]byte(minimalVersionTestYAML), opts); err != nil {
			t.Fatalf("expected absent version to succeed, got error: %v", err)
		}
	})

	t.Run("version 3 errors", func(t *testing.T) {
		yamlData := "version: 3\n" + minimalVersionTestYAML
		opts := GetDefaultOptions(idwrap.NewNow())
		_, err := ConvertSimplifiedYAML([]byte(yamlData), opts)
		if err == nil {
			t.Fatal("expected version 3 to fail, got nil error")
		}
		const wantSubstr = "unsupported yamlflow version 3 (this build supports up to 2)"
		if !strings.Contains(err.Error(), wantSubstr) {
			t.Fatalf("expected error to contain %q, got: %v", wantSubstr, err)
		}
	})

	t.Run("export emits version as first key", func(t *testing.T) {
		opts := GetDefaultOptions(idwrap.NewNow())
		bundle, err := ConvertSimplifiedYAML([]byte(minimalVersionTestYAML), opts)
		if err != nil {
			t.Fatalf("failed to convert: %v", err)
		}
		out, err := MarshalSimplifiedYAML(bundle)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}
		const wantFirstLine = "version: 2"
		firstLine := strings.SplitN(string(out), "\n", 2)[0]
		if firstLine != wantFirstLine {
			t.Fatalf("expected first line to be %q, got %q", wantFirstLine, firstLine)
		}
	})
}
