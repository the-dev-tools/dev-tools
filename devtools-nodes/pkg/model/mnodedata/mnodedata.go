package mnodedata

type LoopData struct {
	Count         int
	LoopStartNode string
}

type LoopRemoteData struct {
	Count             uint64
	LoopStartNode     string
	MachinesAmount    uint64
	SlaveHttpEndpoint string
}
