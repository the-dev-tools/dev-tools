// Command nopermskip runs the nopermskip analyzer.
package main

import (
	"github.com/the-dev-tools/dev-tools/tools/nopermskip"

	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(nopermskip.Analyzer)
}
