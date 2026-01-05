package notxread_test

import (
	"testing"

	"the-dev-tools/notxread"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, notxread.Analyzer, "txread")
}
