package nopermskip_test

import (
	"testing"

	"github.com/the-dev-tools/dev-tools/tools/nopermskip"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, nopermskip.Analyzer, "handler")
}
