package machine

type Machine interface {
	GetID() string
	GetName() string
	GetRegion() string
}
