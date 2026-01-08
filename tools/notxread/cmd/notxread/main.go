// Command notxread runs the notxread analyzer.
package main

import (
	"github.com/the-dev-tools/dev-tools/tools/notxread"

	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(notxread.Analyzer)
}
