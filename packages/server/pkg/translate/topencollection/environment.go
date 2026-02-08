package topencollection

import (
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/menv"
)

// convertEnvironment converts an OpenCollection environment to DevTools models.
func convertEnvironment(ocEnv OCEnvironment, workspaceID idwrap.IDWrap) (menv.Env, []menv.Variable) {
	envID := idwrap.NewNow()
	env := menv.Env{
		ID:          envID,
		WorkspaceID: workspaceID,
		Type:        menv.EnvNormal,
		Name:        ocEnv.Name,
	}

	var vars []menv.Variable
	for i, v := range ocEnv.Variables {
		enabled := true
		if v.Enabled != nil {
			enabled = *v.Enabled
		}

		vars = append(vars, menv.Variable{
			ID:      idwrap.NewNow(),
			EnvID:   envID,
			VarKey:  v.Name,
			Value:   v.Value,
			Enabled: enabled,
			Order:   float64(i + 1),
		})
	}

	return env, vars
}
