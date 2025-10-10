package rflow

import (
	"sync"

	"the-dev-tools/server/pkg/flow/node/nrequest"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnodeexecution"
	"the-dev-tools/server/pkg/service/snodeexecution"
)

type requestExecutionCoordinator struct {
	nes           snodeexecution.NodeExecutionService
	sendExecution func(mnodeexecution.NodeExecution) error
	sendExample   func(idwrap.IDWrap, idwrap.IDWrap) error

	mu     sync.Mutex
	states map[idwrap.IDWrap]*requestExecutionState
}

type requestExecutionState struct {
	exec            *mnodeexecution.NodeExecution
	completed       bool
	executionSent   bool
	response        *nrequest.NodeRequestSideResp
	responsePersist bool
	exampleSent     bool
}

func newRequestExecutionCoordinator(
	nes snodeexecution.NodeExecutionService,
	sendExecution func(mnodeexecution.NodeExecution) error,
	sendExample func(idwrap.IDWrap, idwrap.IDWrap) error,
) *requestExecutionCoordinator {
	return &requestExecutionCoordinator{
		nes:           nes,
		sendExecution: sendExecution,
		sendExample:   sendExample,
		states:        make(map[idwrap.IDWrap]*requestExecutionState),
	}
}

func (c *requestExecutionCoordinator) Register(exec mnodeexecution.NodeExecution) error {
	state := c.ensureState(exec.ID)

	execCopy := exec
	c.mu.Lock()
	state.exec = &execCopy
	state.completed = false
	c.mu.Unlock()

	if err := persistUpsert2s(c.nes, execCopy); err != nil {
		return err
	}

	return c.tryAttachResponse(exec.ID)
}

func (c *requestExecutionCoordinator) RecordResponse(resp nrequest.NodeRequestSideResp, persisted bool) error {
	respCopy := resp

	state := c.ensureState(resp.ExecutionID)

	c.mu.Lock()
	state.response = &respCopy
	state.responsePersist = persisted
	var needsExecutionUpdate bool
	var execForUpdate *mnodeexecution.NodeExecution
	if state.exec != nil {
		execCopy := *state.exec
		respID := respCopy.Resp.ExampleResp.ID
		execCopy.ResponseID = &respID
		state.exec = &execCopy
		needsExecutionUpdate = true
		execForUpdate = &execCopy
	}

	shouldStreamExecution := state.completed && !state.executionSent && c.canStreamExecution(state)
	var executionToSend *mnodeexecution.NodeExecution
	if shouldStreamExecution {
		execCopy := *state.exec
		state.executionSent = true
		executionToSend = &execCopy
	}

	shouldStreamExample := state.completed && !state.exampleSent && state.responsePersist && state.response != nil
	var exampleToSend *nrequest.NodeRequestSideResp
	if shouldStreamExample {
		state.exampleSent = true
		exampleCopy := *state.response
		exampleToSend = &exampleCopy
	}
	c.mu.Unlock()

	if needsExecutionUpdate {
		if err := persistUpsert2s(c.nes, *execForUpdate); err != nil {
			return err
		}
	}

	if executionToSend != nil {
		if err := c.sendExecution(*executionToSend); err != nil {
			return err
		}
	}
	if exampleToSend != nil {
		return c.sendExample(exampleToSend.Example.ID, exampleToSend.Resp.ExampleResp.ID)
	}
	return nil
}

func (c *requestExecutionCoordinator) Complete(
	executionID idwrap.IDWrap, updateFn func(*mnodeexecution.NodeExecution) error) error {
	state := c.ensureState(executionID)

	var execCopy mnodeexecution.NodeExecution

	c.mu.Lock()
	if state.exec == nil {
		state.exec = &mnodeexecution.NodeExecution{
			ID: executionID,
		}
	}
	if err := updateFn(state.exec); err != nil {
		c.mu.Unlock()
		return err
	}
	execCopy = *state.exec
	c.mu.Unlock()

	if err := persistUpsert2s(c.nes, execCopy); err != nil {
		return err
	}

	c.mu.Lock()
	state.exec = &execCopy
	state.completed = true

	var executionToSend *mnodeexecution.NodeExecution
	if !state.executionSent && c.canStreamExecution(state) {
		exec := *state.exec
		state.executionSent = true
		executionToSend = &exec
	}

	var exampleToSend *nrequest.NodeRequestSideResp
	if state.responsePersist && state.response != nil && !state.exampleSent {
		respCopy := *state.response
		state.exampleSent = true
		exampleToSend = &respCopy
	}
	c.mu.Unlock()

	if executionToSend != nil {
		if err := c.sendExecution(*executionToSend); err != nil {
			return err
		}
	}
	if exampleToSend != nil {
		return c.sendExample(exampleToSend.Example.ID, exampleToSend.Resp.ExampleResp.ID)
	}
	return nil
}

func (c *requestExecutionCoordinator) Flush() error {
	c.mu.Lock()
	executions := make([]mnodeexecution.NodeExecution, 0, len(c.states))
	examples := make([]nrequest.NodeRequestSideResp, 0, len(c.states))
	for _, state := range c.states {
		if state.exec != nil && !state.executionSent {
			exec := *state.exec
			state.executionSent = true
			executions = append(executions, exec)
		}
		if state.responsePersist && state.response != nil && !state.exampleSent {
			respCopy := *state.response
			state.exampleSent = true
			examples = append(examples, respCopy)
		}
	}
	c.mu.Unlock()

	for _, exec := range executions {
		if err := c.sendExecution(exec); err != nil {
			return err
		}
	}
	for _, resp := range examples {
		if err := c.sendExample(resp.Example.ID, resp.Resp.ExampleResp.ID); err != nil {
			return err
		}
	}
	return nil
}

func (c *requestExecutionCoordinator) tryAttachResponse(executionID idwrap.IDWrap) error {
	c.mu.Lock()
	state, ok := c.states[executionID]
	if !ok || state.exec == nil || state.response == nil {
		c.mu.Unlock()
		return nil
	}
	execCopy := *state.exec
	respID := state.response.Resp.ExampleResp.ID
	execCopy.ResponseID = &respID
	c.mu.Unlock()

	if err := persistUpsert2s(c.nes, execCopy); err != nil {
		return err
	}

	c.mu.Lock()
	state.exec = &execCopy
	var executionToSend *mnodeexecution.NodeExecution
	if state.completed && !state.executionSent && c.canStreamExecution(state) {
		exec := *state.exec
		state.executionSent = true
		executionToSend = &exec
	}
	var exampleToSend *nrequest.NodeRequestSideResp
	if state.completed && state.responsePersist && state.response != nil && !state.exampleSent {
		respCopy := *state.response
		state.exampleSent = true
		exampleToSend = &respCopy
	}
	c.mu.Unlock()

	if executionToSend != nil {
		if err := c.sendExecution(*executionToSend); err != nil {
			return err
		}
	}
	if exampleToSend != nil {
		return c.sendExample(exampleToSend.Example.ID, exampleToSend.Resp.ExampleResp.ID)
	}
	return nil
}

func (c *requestExecutionCoordinator) ensureState(executionID idwrap.IDWrap) *requestExecutionState {
	c.mu.Lock()
	defer c.mu.Unlock()
	if state, ok := c.states[executionID]; ok {
		return state
	}
	state := &requestExecutionState{}
	c.states[executionID] = state
	return state
}

func (c *requestExecutionCoordinator) canStreamExecution(state *requestExecutionState) bool {
	if state.response == nil {
		return true
	}
	return state.responsePersist
}

func (c *requestExecutionCoordinator) MarkResponsePersisted(resp nrequest.NodeRequestSideResp) error {
	respCopy := resp
	state := c.ensureState(resp.ExecutionID)

	c.mu.Lock()
	if state.response == nil {
		state.response = &respCopy
	}
	state.responsePersist = true

	var executionToSend *mnodeexecution.NodeExecution
	if state.completed && !state.executionSent && c.canStreamExecution(state) && state.exec != nil {
		exec := *state.exec
		state.executionSent = true
		executionToSend = &exec
	}

	var exampleToSend *nrequest.NodeRequestSideResp
	if state.completed && !state.exampleSent && state.response != nil {
		state.exampleSent = true
		copyResp := *state.response
		exampleToSend = &copyResp
	}
	c.mu.Unlock()

	if executionToSend != nil {
		if err := c.sendExecution(*executionToSend); err != nil {
			return err
		}
	}
	if exampleToSend != nil {
		return c.sendExample(exampleToSend.Example.ID, exampleToSend.Resp.ExampleResp.ID)
	}
	return nil
}
