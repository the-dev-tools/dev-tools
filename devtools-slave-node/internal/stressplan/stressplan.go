package stressplan

import (
	"sync"
	"time"

	"github.com/DevToolsGit/devtools-slave-node/pkg/resolver"
	"github.com/DevToolsGit/devtools-tasks/pkg/model/mrequest"
	"github.com/DevToolsGit/devtools-tasks/pkg/model/mtask"
	"github.com/DevToolsGit/devtools-tasks/pkg/model/mtaskresult"
	"github.com/google/uuid"
)

type StressPlan struct {
	UUID        string
	Task        mtask.TaskRequest
	ReqResaults []mrequest.RequestResult
	TimeTaken   time.Duration
	TaskResault mtaskresult.TaskResult
}

func NewStressPlan(id int, tasks mtask.TaskRequest) StressPlan {
	return StressPlan{
		Task: tasks,
	}
}

func (sp *StressPlan) Setup() error {
	uuid, err := uuid.NewV7()
	if err != nil {
		return err
	}

	sp.UUID = uuid.String()
	return nil
}

type resaultPair struct {
	resault *mrequest.RequestResult
	err     error
}

func (sp *StressPlan) Execute() error {
	// execute the stress plan
	method := resolver.Resolve(sp.Task.RequestData.MethodTypeID)
	wg := sync.WaitGroup{}
	resualtChan := make(chan resaultPair, sp.Task.Amount)

	now := time.Now()
	for i := 0; i < sp.Task.Amount; i++ {
		wg.Add(1)
		go func() {
			res, err := method(sp.Task.RequestData)
			resualtChan <- resaultPair{res, err}
			wg.Done()
		}()
	}

	// TODO: add timeout here to check if the stress plan is taking too long
	wg.Wait()

	sp.TimeTaken = time.Since(now)

	for i := 0; i < sp.Task.Amount; i++ {
		res := <-resualtChan
		if res.err != nil {
			return res.err
		}
		sp.ReqResaults = append(sp.ReqResaults, *res.resault)
	}

	return nil
}

func (sp *StressPlan) Analize() error {
	taskResault := mtaskresult.TaskResult{
		ID: sp.UUID,
	}

	failAmount := 0
	for _, reqRes := range sp.ReqResaults {
		if reqRes.StatusCode != 200 {
			failAmount++
		}
		_, ok := taskResault.StatusCodeMap[reqRes.StatusCode]
		if !ok {
			taskResault.StatusCodeMap[reqRes.StatusCode] = 0
		}
		taskResault.StatusCodeMap[reqRes.StatusCode]++

		// TODO: maybe we can body size limit to avoid too much data in the response
		taskResault.RequestBodyMap[reqRes.RequestID] = reqRes.Body

		taskResault.RequestIDs = append(taskResault.RequestIDs, reqRes.RequestID)
	}

	if float32(failAmount)/float32(sp.Task.Amount) > sp.Task.PassSuccessRate {
		taskResault.Status = mtaskresult.TaskStatusSuccess
	} else {
		taskResault.Status = mtaskresult.TaskStatusFailed
	}

	taskResault.SuccessCount = sp.Task.Amount - failAmount
	taskResault.FailedCount = failAmount

	// TODO: will look into network error to find out
	taskResault.SentCount = sp.Task.Amount

	return nil
}
