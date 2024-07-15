package mtaskmasterresult

import "devtools-tasks/pkg/model/mtaskresult"

type TaskMasterResult struct {
	ID           int
	TaskMasterID int
	TaskResaults []mtaskresult.TaskResult
}
