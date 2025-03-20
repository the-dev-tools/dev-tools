package tworkspace

import (
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/menv"
	"the-dev-tools/backend/pkg/model/mworkspace"
	workspacev1 "the-dev-tools/spec/dist/buf/go/workspace/v1"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func SeralizeWorkspace(ws mworkspace.Workspace, env *menv.Env) *workspacev1.Workspace {
	var selectedEnvID []byte = nil
	if env != nil {
		selectedEnvID = env.ID.Bytes()
	}

	return &workspacev1.Workspace{
		WorkspaceId:           ws.ID.Bytes(),
		Name:                  ws.Name,
		Updated:               timestamppb.New(ws.Updated),
		SelectedEnvironmentId: selectedEnvID,
		CollectionCount:       ws.CollectionCount,
		FlowCount:             ws.FlowCount,
	}
}

func SeralizeWorkspaceItem(ws mworkspace.Workspace, env *menv.Env) *workspacev1.WorkspaceListItem {
	var selectedEnvID []byte = nil
	if env != nil {
		selectedEnvID = env.ID.Bytes()
	}

	return &workspacev1.WorkspaceListItem{
		WorkspaceId:           ws.ID.Bytes(),
		SelectedEnvironmentId: selectedEnvID,
		Name:                  ws.Name,
		Updated:               timestamppb.New(ws.Updated),
		CollectionCount:       ws.CollectionCount,
		FlowCount:             ws.FlowCount,
	}
}

func DeserializeWorkspace(ws *workspacev1.Workspace) (mworkspace.Workspace, error) {
	id, err := idwrap.NewFromBytes(ws.WorkspaceId)
	if err != nil {
		return mworkspace.Workspace{}, err
	}

	return mworkspace.Workspace{
		ID:      id,
		Name:    ws.Name,
		Updated: ws.Updated.AsTime(),
	}, nil
}
