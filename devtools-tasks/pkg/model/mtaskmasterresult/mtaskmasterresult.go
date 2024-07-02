package mtaskmasterresult

import "github.com/DevToolsGit/devtools-tasks/pkg/model/mtaskresult"

type TaskMasterResult struct {
	ID           int
	TaskMasterID int
	TaskResaults []mtaskresult.TaskResult
}
