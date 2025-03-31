package svar

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mvar"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

type VarService struct {
	queries *gen.Queries
}

var ErrNoVarFound error = sql.ErrNoRows

func New(queries *gen.Queries) VarService {
	return VarService{queries: queries}
}

func (e VarService) TX(tx *sql.Tx) VarService {
	return VarService{queries: e.queries.WithTx(tx)}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*VarService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	service := VarService{queries: queries}
	return &service, nil
}

func ConvertToDBVar(varParm mvar.Var) gen.Variable {
	return gen.Variable{
		ID:          varParm.ID,
		EnvID:       varParm.EnvID,
		VarKey:      varParm.VarKey,
		Value:       varParm.Value,
		Enabled:     varParm.Enabled,
		Description: varParm.Description,
	}
}

func ConvertToModelVar(varParm gen.Variable) *mvar.Var {
	return &mvar.Var{
		ID:          varParm.ID,
		EnvID:       varParm.EnvID,
		VarKey:      varParm.VarKey,
		Value:       varParm.Value,
		Enabled:     varParm.Enabled,
		Description: varParm.Description,
	}
}

func (e VarService) Get(ctx context.Context, id idwrap.IDWrap) (*mvar.Var, error) {
	variable, err := e.queries.GetVariable(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoVarFound
		}
		return nil, err
	}
	return ConvertToModelVar(variable), nil
}

func (e VarService) GetVariableByEnvID(ctx context.Context, envID idwrap.IDWrap) ([]mvar.Var, error) {
	rows, err := e.queries.GetVariablesByEnvironmentID(ctx, envID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mvar.Var{}, ErrNoVarFound
		}
		return nil, err
	}
	return tgeneric.MassConvertPtr(rows, ConvertToModelVar), nil
}

func (e VarService) Create(ctx context.Context, varParm mvar.Var) error {
	variable := ConvertToDBVar(varParm)
	return e.queries.CreateVariable(ctx, gen.CreateVariableParams{
		ID:          variable.ID,
		EnvID:       variable.EnvID,
		VarKey:      variable.VarKey,
		Value:       variable.Value,
		Enabled:     variable.Enabled,
		Description: variable.Description,
	})
}

func (e VarService) Update(ctx context.Context, varParm *mvar.Var) error {
	variable := ConvertToDBVar(*varParm)
	return e.queries.UpdateVariable(ctx, gen.UpdateVariableParams{
		ID:          variable.ID,
		VarKey:      variable.VarKey,
		Value:       variable.Value,
		Enabled:     variable.Enabled,
		Description: variable.Description,
	})
}

func (e VarService) Delete(ctx context.Context, id idwrap.IDWrap) error {
	return e.queries.DeleteVariable(ctx, id)
}
