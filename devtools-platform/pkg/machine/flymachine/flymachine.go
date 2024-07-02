package flymachine

type FlyMachine struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (m *FlyMachine) GetName() string {
	return m.Name
}
