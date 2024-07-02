package client

import "github.com/DevToolsGit/devtools-platform/pkg/machine"

type Client interface {
	ListMachines() ([]string, error)
	GetMachine(name string) (machine.Machine, error)
}
