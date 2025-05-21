package embededJS

import _ "embed"

//go:embed worker.cjs.embed
var WorkerJS string
