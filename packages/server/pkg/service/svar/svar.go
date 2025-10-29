package svar

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mvar"
)

type VarService struct {
	queries *gen.Queries
	logger  *slog.Logger
}

var (
	ErrNoVarFound                   = sql.ErrNoRows
	ErrEnvironmentBoundaryViolation = fmt.Errorf("variables must be in same environment")
	ErrSelfReferentialMove          = fmt.Errorf("cannot move variable relative to itself")
)

func New(queries *gen.Queries, logger *slog.Logger) VarService {
	if logger == nil {
		logger = slog.Default()
	}
	return VarService{
		queries: queries,
		logger:  logger,
	}
}

func (s VarService) TX(tx *sql.Tx) VarService {
	if tx == nil {
		return s
	}
	return VarService{
		queries: s.queries.WithTx(tx),
		logger:  s.logger,
	}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*VarService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	service := VarService{
		queries: queries,
		logger:  slog.Default(),
	}
	return &service, nil
}

func ConvertToDBVar(v mvar.Var) gen.Variable {
	return gen.Variable{
		ID:           v.ID,
		EnvID:        v.EnvID,
		VarKey:       v.VarKey,
		Value:        v.Value,
		Enabled:      v.Enabled,
		Description:  v.Description,
		DisplayOrder: v.Order,
	}
}

func ConvertToModelVar(v gen.Variable) *mvar.Var {
	return &mvar.Var{
		ID:          v.ID,
		EnvID:       v.EnvID,
		VarKey:      v.VarKey,
		Value:       v.Value,
		Enabled:     v.Enabled,
		Description: v.Description,
		Order:       v.DisplayOrder,
	}
}

func (s VarService) Get(ctx context.Context, id idwrap.IDWrap) (*mvar.Var, error) {
	variable, err := s.queries.GetVariable(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoVarFound
		}
		return nil, err
	}
	return ConvertToModelVar(variable), nil
}

func (s VarService) GetVariableByEnvID(ctx context.Context, envID idwrap.IDWrap) ([]mvar.Var, error) {
	rows, err := s.queries.GetVariablesByEnvironmentIDOrdered(ctx, envID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mvar.Var{}, nil
		}
		return nil, err
	}

	vars := make([]mvar.Var, len(rows))
	for i, row := range rows {
		vars[i] = *ConvertToModelVar(row)
	}
	return vars, nil
}

func (s VarService) Create(ctx context.Context, variable mvar.Var) error {
	if variable.Order == 0 {
		nextOrder, err := s.nextDisplayOrder(ctx, variable.EnvID)
		if err != nil {
			return err
		}
		variable.Order = nextOrder
	}

	dbVar := ConvertToDBVar(variable)
	return s.queries.CreateVariable(ctx, gen.CreateVariableParams{
		ID:           dbVar.ID,
		EnvID:        dbVar.EnvID,
		VarKey:       dbVar.VarKey,
		Value:        dbVar.Value,
		Enabled:      dbVar.Enabled,
		Description:  dbVar.Description,
		DisplayOrder: dbVar.DisplayOrder,
	})
}

func (s VarService) Update(ctx context.Context, variable *mvar.Var) error {
	if variable.Order == 0 {
		current, err := s.queries.GetVariable(ctx, variable.ID)
		if err != nil {
			if err == sql.ErrNoRows {
				return ErrNoVarFound
			}
			return err
		}
		variable.Order = current.DisplayOrder
	}

	dbVar := ConvertToDBVar(*variable)
	return s.queries.UpdateVariable(ctx, gen.UpdateVariableParams{
		ID:           dbVar.ID,
		VarKey:       dbVar.VarKey,
		Value:        dbVar.Value,
		Enabled:      dbVar.Enabled,
		Description:  dbVar.Description,
		DisplayOrder: dbVar.DisplayOrder,
	})
}

func (s VarService) Delete(ctx context.Context, id idwrap.IDWrap) error {
	if err := s.queries.DeleteVariable(ctx, id); err != nil {
		if err == sql.ErrNoRows {
			return ErrNoVarFound
		}
		return err
	}
	return nil
}

func (s VarService) GetEnvID(ctx context.Context, varID idwrap.IDWrap) (idwrap.IDWrap, error) {
	variable, err := s.queries.GetVariable(ctx, varID)
	if err != nil {
		if err == sql.ErrNoRows {
			return idwrap.IDWrap{}, ErrNoVarFound
		}
		return idwrap.IDWrap{}, err
	}
	return variable.EnvID, nil
}

func (s VarService) CheckEnvironmentBoundaries(ctx context.Context, varID, envID idwrap.IDWrap) (bool, error) {
	actualEnvID, err := s.GetEnvID(ctx, varID)
	if err != nil {
		return false, err
	}
	return actualEnvID.Compare(envID) == 0, nil
}

func (s VarService) GetVariablesByEnvIDOrdered(ctx context.Context, envID idwrap.IDWrap) ([]mvar.Var, error) {
	return s.GetVariableByEnvID(ctx, envID)
}

func (s VarService) nextDisplayOrder(ctx context.Context, envID idwrap.IDWrap) (float64, error) {
	vars, err := s.queries.GetVariablesByEnvironmentID(ctx, envID)
	if err != nil {
		if err == sql.ErrNoRows {
			return 1, nil
		}
		return 0, err
	}

	max := 0.0
	for _, v := range vars {
		if v.DisplayOrder > max {
			max = v.DisplayOrder
		}
	}
	return max + 1, nil
}

// Legacy move helpers ---------------------------------------------------------

func (s VarService) MoveVariableAfter(ctx context.Context, varID, targetVarID idwrap.IDWrap) error {
	return s.moveVariable(ctx, varID, targetVarID, true)
}

func (s VarService) MoveVariableBefore(ctx context.Context, varID, targetVarID idwrap.IDWrap) error {
	return s.moveVariable(ctx, varID, targetVarID, false)
}

func (s VarService) moveVariable(ctx context.Context, varID, targetVarID idwrap.IDWrap, after bool) error {
	if varID.Compare(targetVarID) == 0 {
		return ErrSelfReferentialMove
	}

	sourceEnvID, err := s.GetEnvID(ctx, varID)
	if err != nil {
		return err
	}

	targetEnvID, err := s.GetEnvID(ctx, targetVarID)
	if err != nil {
		return err
	}

	if sourceEnvID.Compare(targetEnvID) != 0 {
		return ErrEnvironmentBoundaryViolation
	}

	vars, err := s.queries.GetVariablesByEnvironmentIDOrdered(ctx, sourceEnvID)
	if err != nil {
		return err
	}

	var (
		targetOrder float64
		hasTarget   bool
		hasSource   bool
		nextOrder   *float64
		prevOrder   *float64
	)

	for idx, row := range vars {
		if row.ID.Compare(targetVarID) == 0 {
			targetOrder = row.DisplayOrder
			hasTarget = true
			// Look ahead for the first neighbour that is not the moving variable.
			for j := idx + 1; j < len(vars); j++ {
				if vars[j].ID.Compare(varID) == 0 {
					continue
				}
				val := vars[j].DisplayOrder
				nextOrder = &val
				break
			}
			// Look behind for before operations.
			for j := idx - 1; j >= 0; j-- {
				if vars[j].ID.Compare(varID) == 0 {
					continue
				}
				val := vars[j].DisplayOrder
				prevOrder = &val
				break
			}
		}

		if row.ID.Compare(varID) == 0 {
			hasSource = true
		}
	}

	if !hasTarget || !hasSource {
		return ErrNoVarFound
	}

	var newOrder float64
	if after {
		if nextOrder != nil {
			newOrder = (targetOrder + *nextOrder) / 2
		} else {
			newOrder = targetOrder + 1
		}
	} else {
		if prevOrder != nil {
			newOrder = (*prevOrder + targetOrder) / 2
		} else {
			newOrder = targetOrder - 1
		}
	}

	current, err := s.queries.GetVariable(ctx, varID)
	if err != nil {
		if err == sql.ErrNoRows {
			return ErrNoVarFound
		}
		return err
	}

	current.DisplayOrder = newOrder
	return s.queries.UpdateVariable(ctx, gen.UpdateVariableParams{
		ID:           current.ID,
		VarKey:       current.VarKey,
		Value:        current.Value,
		Enabled:      current.Enabled,
		Description:  current.Description,
		DisplayOrder: current.DisplayOrder,
	})
}
