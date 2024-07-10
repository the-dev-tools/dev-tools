package mresolver

import "github.com/DevToolsGit/devtools-nodes/pkg/model/mnodemaster"

type Resolver func(nodeType string) (func(*mnodemaster.NodeMaster) error, error)
