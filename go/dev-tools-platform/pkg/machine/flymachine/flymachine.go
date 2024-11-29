package flymachine

type Region string

const (
	RegionAmsterdam Region = "ams"
	RegionFrankfurt Region = "fra"
	RegionStockholm Region = "arn"
	RegionAtlanta   Region = "atl"
	RegionParis     Region = "cdg"
	RegionDallas    Region = "dfw"
	RegionLondon    Region = "lhr"
	RegionJapan     Region = "nrt"
	RegionSingapore Region = "sin"
	RegionAustralia Region = "syd"
	RegionToronto   Region = "yyz"
)

type CpuKind string

const (
	CpuKindShared      CpuKind = "shared"
	CpuKindPerformance CpuKind = "performance"
)

type State string

func (s State) String() string {
	return string(s)
}

const (
	StateStarted   State = "started"
	StateStopped   State = "stopped"
	StateDestroyed State = "destroyed"
)

type FlyMachine struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Config     FlyMachineCreateConfig `json:"config"`
	InstanceID string                 `json:"instance_id,omitempty"`
	Region     Region                 `json:"region"`
	Guest      Guest                  `json:"guest"`
	PrivateIP  string                 `json:"private_ip,omitempty"`
}

type Guest struct {
	Memory  int     `json:"memory"`
	Cpus    int     `json:"cpus"`
	CpuKind CpuKind `json:"cpu_kind"`
}

type FlyMachineCreateConfig struct {
	Image    string              `json:"image"`
	Env      map[string]string   `json:"env"`
	Services []FlyMachineService `json:"services"`
}

type FlyMachineService struct {
	Ports        []FlyMachinePortPair `json:"ports"`
	Protocol     string               `json:"protocol"`
	InternalPort int                  `json:"internal_port"`
}

type FlyMachinePortPair struct {
	Port     int      `json:"port"`
	Handlers []string `json:"handlers"`
}

func New(id, name string) *FlyMachine {
	return &FlyMachine{
		ID:   id,
		Name: name,
	}
}

func (m *FlyMachine) GetName() string {
	return m.Name
}

func (m *FlyMachine) GetID() string {
	return m.ID
}

func (m *FlyMachine) GetRegion() string {
	return string(m.Region)
}

func (m *FlyMachine) GetIP() string {
	return m.PrivateIP
}

func (m *FlyMachine) GetInstanceID() string {
	return m.InstanceID
}

func (m *FlyMachine) GetInternalPort() int {
	return m.Config.Services[0].InternalPort
}
