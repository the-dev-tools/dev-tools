package notxread_test

import (
	"testing"

	"github.com/the-dev-tools/dev-tools/tools/notxread"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, notxread.Analyzer, "txread")
}
