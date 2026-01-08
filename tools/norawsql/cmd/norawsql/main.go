// Command norawsql runs the norawsql analyzer.
package main

import (
	"github.com/the-dev-tools/dev-tools/tools/norawsql"

	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(norawsql.Analyzer)
}
