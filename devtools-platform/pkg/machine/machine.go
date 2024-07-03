package machine

type Machine interface {
	GetID() string
	GetName() string
}

type MachineCreateRequest struct {
	Name   string                 `json:"name"`
	Config map[string]interface{} `json:"config"`
}
