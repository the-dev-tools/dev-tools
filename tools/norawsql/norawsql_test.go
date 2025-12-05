package norawsql_test

import (
	"testing"

	"the-dev-tools/norawsql"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, norawsql.Analyzer, "rawsql")
}
