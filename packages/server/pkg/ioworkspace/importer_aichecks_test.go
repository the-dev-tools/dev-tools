package ioworkspace

import (
	"strings"
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/httpclient"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mexpect"
)

// AI checks and stream settings have no storage yet: an import says it drops them.
func TestImportWarnsThatAIChecksAreNotStored(t *testing.T) {
	limit := 400.0
	b := loadScenarioBundle()
	b.AIChecks = &mexpect.Checks{Steps: map[idwrap.IDWrap]mexpect.Expect{idwrap.NewNow(): {MaxLatencyMS: &limit}}}
	b.RequestStreams = map[idwrap.IDWrap]httpclient.StreamOptions{idwrap.NewNow(): {Preset: "openai"}}
	logs := importWithLogger(t, b)
	if !strings.Contains(logs, AIChecksNotStoredMessage) || !strings.Contains(logs, "expect_blocks=1") || !strings.Contains(logs, "streams=1") {
		t.Errorf("import did not warn about unstored AI checks; logs:\n%s", logs)
	}
	if logs := importWithLogger(t, loadScenarioBundle()); strings.Contains(logs, AIChecksNotStoredMessage) {
		t.Errorf("warned on a bundle without AI checks; logs:\n%s", logs)
	}
}
