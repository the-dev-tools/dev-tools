package tworkspace

import (
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/menv"
	"dev-tools-backend/pkg/model/mworkspace"
	"dev-tools-backend/pkg/translate/tenv"
	environmentv1 "dev-tools-services/gen/environment/v1"
	workspacev1 "dev-tools-services/gen/workspace/v1"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func SeralizeWorkspace(ws mworkspace.Workspace, env *menv.Env) *workspacev1.Workspace {
	var rpcEnv *environmentv1.Environment
	if env != nil {
		rpcEnv = tenv.SeralizeModelToRPC(*env)
	}

	return &workspacev1.Workspace{
		Id:          ws.ID.String(),
		Name:        ws.Name,
		Updated:     timestamppb.New(ws.Updated),
		Environment: rpcEnv,
	}
}

func DeserializeWorkspace(ws *workspacev1.Workspace) (mworkspace.Workspace, error) {
	id, err := idwrap.NewWithParse(ws.Id)
	if err != nil {
		return mworkspace.Workspace{}, err
	}

	return mworkspace.Workspace{
		ID:      id,
		Name:    ws.Name,
		Updated: ws.Updated.AsTime(),
	}, nil
}
