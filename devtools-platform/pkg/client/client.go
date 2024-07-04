package client

import (
	"github.com/DevToolsGit/devtools-platform/pkg/machine"
)

type Client interface {
	ListMachines() ([]string, error)
	GetMachine(name string) (machine.Machine, error)
	CreateMachine(machineCreate machine.Machine) (machine.Machine, error)
	DeleteMachine(id string, force bool) error
}
