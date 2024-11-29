package machine

type Machine interface {
	GetID() string
	GetInstanceID() string
	GetIP() string
	GetInternalPort() int
	GetName() string
	GetRegion() string
}
