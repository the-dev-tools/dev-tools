package machine

type Machine interface {
	GetID() string
	GetIP() string
	GetInternalPort() int
	GetName() string
	GetRegion() string
}
