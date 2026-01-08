package senv

import (
	"context"
	"database/sql"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/menv"
)

type VariableWriter struct {
	queries *gen.Queries
}

func NewVariableWriter(tx gen.DBTX) *VariableWriter {
	return &VariableWriter{
		queries: gen.New(tx),
	}
}

func NewVariableWriterFromQueries(queries *gen.Queries) *VariableWriter {
	return &VariableWriter{
		queries: queries,
	}
}

func (w *VariableWriter) Create(ctx context.Context, variable menv.Variable) error {
	if variable.Order == 0 {
		nextOrder, err := w.nextDisplayOrder(ctx, variable.EnvID)
		if err != nil {
			return err
		}
		variable.Order = nextOrder
	}

	dbVar := ConvertToDBVar(variable)
	return w.queries.CreateVariable(ctx, gen.CreateVariableParams(dbVar))
}

func (w *VariableWriter) Update(ctx context.Context, variable *menv.Variable) error {
	if variable.Order == 0 {
		current, err := w.queries.GetVariable(ctx, variable.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNoVarFound
			}
			return err
		}
		variable.Order = current.DisplayOrder
	}

	dbVar := ConvertToDBVar(*variable)
	return w.queries.UpdateVariable(ctx, gen.UpdateVariableParams{
		ID:           dbVar.ID,
		VarKey:       dbVar.VarKey,
		Value:        dbVar.Value,
		Enabled:      dbVar.Enabled,
		Description:  dbVar.Description,
		DisplayOrder: dbVar.DisplayOrder,
	})
}

func (w *VariableWriter) Upsert(ctx context.Context, variable menv.Variable) error {
	if variable.Order == 0 {
		nextOrder, err := w.nextDisplayOrder(ctx, variable.EnvID)
		if err != nil {
			return err
		}
		variable.Order = nextOrder
	}

	dbVar := ConvertToDBVar(variable)
	return w.queries.UpsertVariable(ctx, gen.UpsertVariableParams(dbVar))
}

func (w *VariableWriter) Delete(ctx context.Context, id idwrap.IDWrap) error {
	if err := w.queries.DeleteVariable(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoVarFound
		}
		return err
	}
	return nil
}

func (w *VariableWriter) nextDisplayOrder(ctx context.Context, envID idwrap.IDWrap) (float64, error) {
	vars, err := w.queries.GetVariablesByEnvironmentID(ctx, envID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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

// MoveVariableAfter moves a variable after the target variable
func (w *VariableWriter) MoveVariableAfter(ctx context.Context, varID, targetVarID idwrap.IDWrap) error {
	return w.moveVariable(ctx, varID, targetVarID, true)
}

// MoveVariableBefore moves a variable before the target variable
func (w *VariableWriter) MoveVariableBefore(ctx context.Context, varID, targetVarID idwrap.IDWrap) error {
	return w.moveVariable(ctx, varID, targetVarID, false)
}

func (w *VariableWriter) moveVariable(ctx context.Context, varID, targetVarID idwrap.IDWrap, after bool) error {
	if varID.Compare(targetVarID) == 0 {
		return ErrSelfReferentialMove
	}

	// Helper to get EnvID inside the writer context
	getEnvID := func(vID idwrap.IDWrap) (idwrap.IDWrap, error) {
		variable, err := w.queries.GetVariable(ctx, vID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return idwrap.IDWrap{}, ErrNoVarFound
			}
			return idwrap.IDWrap{}, err
		}
		return variable.EnvID, nil
	}

	sourceEnvID, err := getEnvID(varID)
	if err != nil {
		return err
	}

	targetEnvID, err := getEnvID(targetVarID)
	if err != nil {
		return err
	}

	if sourceEnvID.Compare(targetEnvID) != 0 {
		return ErrEnvironmentBoundaryViolation
	}

	vars, err := w.queries.GetVariablesByEnvironmentIDOrdered(ctx, sourceEnvID)
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

	current, err := w.queries.GetVariable(ctx, varID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoVarFound
		}
		return err
	}

	current.DisplayOrder = newOrder
	return w.queries.UpdateVariable(ctx, gen.UpdateVariableParams{
		ID:           current.ID,
		VarKey:       current.VarKey,
		Value:        current.Value,
		Enabled:      current.Enabled,
		Description:  current.Description,
		DisplayOrder: current.DisplayOrder,
	})
}
