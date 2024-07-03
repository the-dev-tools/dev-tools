package client

import (
	"encoding/json"

	"github.com/DevToolsGit/devtools-platform/pkg/machine"
)

type Client interface {
	ListMachines() ([]string, error)
	GetMachine(name string) (machine.Machine, error)
	CreateMachine(marshal json.Marshaler) (machine.Machine, error)
	DeleteMachine(id string, force bool) error
}
