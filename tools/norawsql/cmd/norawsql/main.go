// Command norawsql runs the norawsql analyzer.
package main

import (
	"the-dev-tools/norawsql"

	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(norawsql.Analyzer)
}
