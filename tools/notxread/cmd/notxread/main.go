// Command notxread runs the notxread analyzer.
package main

import (
	"the-dev-tools/notxread"

	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(notxread.Analyzer)
}
