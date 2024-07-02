package mtaskmaster

import "github.com/DevToolsGit/devtools-tasks/pkg/model/mtask"

type TaskMaster struct {
	ID      int
	OwnerID int
	Name    string
	Done    chan bool
	Tasks   map[string]mtask.Task
}

type TaskMasterSender struct {
	ID    int
	Tasks map[string]mtask.TaskSender
}
