package svar

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mvar"
	"the-dev-tools/server/pkg/movable"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

type VarService struct {
	queries              *gen.Queries
	logger               *slog.Logger
	linkedListManager    movable.LinkedListManager
	movableRepository    movable.MovableRepository
}

var (
	ErrNoVarFound                 = sql.ErrNoRows
	ErrInvalidMoveOperation       = fmt.Errorf("invalid move operation")
	ErrEnvironmentBoundaryViolation = fmt.Errorf("variables must be in same environment")
	ErrSelfReferentialMove        = fmt.Errorf("cannot move variable relative to itself")
)

func New(queries *gen.Queries, logger *slog.Logger) VarService {
	// Create the movable repository for variables
	movableRepo := NewVariableMovableRepository(queries)
	
	// Create the linked list manager with the movable repository
	linkedListManager := movable.NewDefaultLinkedListManager(movableRepo)
	
	return VarService{
		queries:              queries,
		logger:               logger,
		linkedListManager:    linkedListManager,
		movableRepository:    movableRepo,
	}
}

func (e VarService) TX(tx *sql.Tx) VarService {
	// Create new instances with transaction support
	txQueries := e.queries.WithTx(tx)
	movableRepo := NewVariableMovableRepository(txQueries)
	linkedListManager := movable.NewDefaultLinkedListManager(movableRepo)
	
	return VarService{
		queries:              txQueries,
		logger:               e.logger,
		linkedListManager:    linkedListManager,
		movableRepository:    movableRepo,
	}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*VarService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	
	// Create movable repository and linked list manager
	movableRepo := NewVariableMovableRepository(queries)
	linkedListManager := movable.NewDefaultLinkedListManager(movableRepo)
	
	// Use a default logger for transaction services
	logger := slog.Default()
	
	service := VarService{
		queries:              queries,
		logger:               logger,
		linkedListManager:    linkedListManager,
		movableRepository:    movableRepo,
	}
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
	// Find the current tail of the linked list (last variable in environment)
	existingVariables, err := e.GetVariablesByEnvIDOrdered(ctx, varParm.EnvID)
	if err != nil && err != ErrNoVarFound {
		return fmt.Errorf("failed to get existing variables: %w", err)
	}

	var prev *idwrap.IDWrap

	// If there are existing variables, set prev to point to the last one
	// and update that variable's next pointer to point to the new one
	if len(existingVariables) > 0 {
		lastVariable := existingVariables[len(existingVariables)-1]
		prev = &lastVariable.ID

		// Get the database version to access prev/next fields
		dbLastVariable, err := e.queries.GetVariable(ctx, lastVariable.ID)
		if err != nil {
			return fmt.Errorf("failed to get last variable from database: %w", err)
		}

		// Update the current tail's next pointer to point to new variable
		err = e.queries.UpdateVariableOrder(ctx, gen.UpdateVariableOrderParams{
			Prev:  dbLastVariable.Prev, // Keep existing prev pointer
			Next:  &varParm.ID,         // Set next to new variable
			ID:    lastVariable.ID,     // Update the last variable
			EnvID: varParm.EnvID,
		})
		if err != nil {
			return fmt.Errorf("failed to update previous tail variable: %w", err)
		}
	}

	// Create the variable with proper linked list pointers
	return e.queries.CreateVariable(ctx, gen.CreateVariableParams{
		ID:          varParm.ID,
		EnvID:       varParm.EnvID,
		VarKey:      varParm.VarKey,
		Value:       varParm.Value,
		Enabled:     varParm.Enabled,
		Description: varParm.Description,
		Prev:        prev, // Points to current tail (or nil if first)
		Next:        nil,  // Always nil for new variables (they become the new tail)
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
	return e.DeleteVariableTX(ctx, nil, id)
}

// GetEnvID returns the environment ID for a variable
func (e VarService) GetEnvID(ctx context.Context, varID idwrap.IDWrap) (idwrap.IDWrap, error) {
	variable, err := e.queries.GetVariable(ctx, varID)
	if err != nil {
		if err == sql.ErrNoRows {
			return idwrap.IDWrap{}, ErrNoVarFound
		}
		return idwrap.IDWrap{}, err
	}
	return variable.EnvID, nil
}

// CheckEnvironmentBoundaries ensures both variables are in the same environment
func (e VarService) CheckEnvironmentBoundaries(ctx context.Context, varID, ownerID idwrap.IDWrap) (bool, error) {
	variableEnvID, err := e.GetEnvID(ctx, varID)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, ErrNoVarFound
		}
		return false, err
	}
	return ownerID.Compare(variableEnvID) == 0, nil
}

// GetVariablesByEnvIDOrdered returns variables in the environment in their proper order
func (e VarService) GetVariablesByEnvIDOrdered(ctx context.Context, envID idwrap.IDWrap) ([]mvar.Var, error) {
	// Use the underlying query that maintains the linked list order
	orderedVariables, err := e.queries.GetVariablesByEnvironmentIDOrdered(ctx, gen.GetVariablesByEnvironmentIDOrderedParams{
		EnvID:   envID,
		EnvID_2: envID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			e.logger.InfoContext(ctx, fmt.Sprintf("envID: %s has no variables", envID.String()))
			return []mvar.Var{}, nil
		}
		return nil, err
	}

	// Convert to model variables
	variables := make([]mvar.Var, len(orderedVariables))
	for i, v := range orderedVariables {
		variables[i] = mvar.Var{
			ID:          idwrap.NewFromBytesMust(v.ID),
			EnvID:       idwrap.NewFromBytesMust(v.EnvID),
			VarKey:      v.VarKey,
			Value:       v.Value,
			Enabled:     v.Enabled,
			Description: v.Description,
		}
	}

	return variables, nil
}

// DeleteVariableTX deletes a variable while maintaining linked-list integrity within a transaction
func (e VarService) DeleteVariableTX(ctx context.Context, tx *sql.Tx, id idwrap.IDWrap) error {
	service := e
	if tx != nil {
		service = e.TX(tx)
	}

	// 1. Get the variable being deleted to find its prev/next pointers
	variable, err := service.queries.GetVariable(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			e.logger.DebugContext(ctx, fmt.Sprintf("Variable %s not found for deletion", id.String()))
			return ErrNoVarFound
		}
		return fmt.Errorf("failed to get variable for deletion: %w", err)
	}

	// 2. Fix linked-list pointers before deletion
	// Update prev variable's next pointer to skip the deleted variable
	if variable.Prev != nil {
		err = service.queries.UpdateVariableNext(ctx, gen.UpdateVariableNextParams{
			Next:  variable.Next,      // Point to the deleted variable's next
			ID:    *variable.Prev,
			EnvID: variable.EnvID,
		})
		if err != nil {
			return fmt.Errorf("failed to update previous variable's next pointer: %w", err)
		}
	}

	// Update next variable's prev pointer to skip the deleted variable
	if variable.Next != nil {
		err = service.queries.UpdateVariablePrev(ctx, gen.UpdateVariablePrevParams{
			Prev:  variable.Prev,      // Point to the deleted variable's prev
			ID:    *variable.Next,
			EnvID: variable.EnvID,
		})
		if err != nil {
			return fmt.Errorf("failed to update next variable's prev pointer: %w", err)
		}
	}

	// 3. Now safely delete the variable
	err = service.queries.DeleteVariable(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete variable after fixing linked-list: %w", err)
	}

	e.logger.DebugContext(ctx, "Variable deleted with linked-list integrity maintained", 
		"variableID", id.String())

	return nil
}

// validateMoveOperation validates that a move operation is safe and valid
func (e VarService) validateMoveOperation(ctx context.Context, varID, targetVarID idwrap.IDWrap) error {
	if varID.Compare(targetVarID) == 0 {
		return ErrSelfReferentialMove
	}
	
	return nil
}

// checkEnvironmentBoundaries ensures both variables are in the same environment
func (e VarService) checkEnvironmentBoundaries(ctx context.Context, varID, targetVarID idwrap.IDWrap) error {
	sourceEnvID, err := e.GetEnvID(ctx, varID)
	if err != nil {
		return fmt.Errorf("failed to get source variable environment: %w", err)
	}

	targetEnvID, err := e.GetEnvID(ctx, targetVarID)
	if err != nil {
		return fmt.Errorf("failed to get target variable environment: %w", err)
	}

	if sourceEnvID.Compare(targetEnvID) != 0 {
		return ErrEnvironmentBoundaryViolation
	}

	return nil
}

// Movable operations for variables

// MoveVariableAfter moves a variable to be positioned after the target variable
func (e VarService) MoveVariableAfter(ctx context.Context, varID, targetVarID idwrap.IDWrap) error {
	return e.MoveVariableAfterTX(ctx, nil, varID, targetVarID)
}

// MoveVariableAfterTX moves a variable to be positioned after the target variable within a transaction
func (e VarService) MoveVariableAfterTX(ctx context.Context, tx *sql.Tx, varID, targetVarID idwrap.IDWrap) error {
	service := e
	if tx != nil {
		service = e.TX(tx)
	}

	// Validate the move operation
	if err := service.validateMoveOperation(ctx, varID, targetVarID); err != nil {
		return err
	}

	// Check environment boundaries
	if err := service.checkEnvironmentBoundaries(ctx, varID, targetVarID); err != nil {
		return err
	}

	// Get environment ID for both variables
	sourceEnvID, err := service.GetEnvID(ctx, varID)
	if err != nil {
		return fmt.Errorf("failed to get source variable environment: %w", err)
	}

	// Get all variables in the environment in order
	variables, err := service.GetVariablesByEnvIDOrdered(ctx, sourceEnvID)
	if err != nil {
		return fmt.Errorf("failed to get variables in order: %w", err)
	}

	// Find positions of source and target variables
	var sourcePos, targetPos int = -1, -1
	for i, v := range variables {
		if v.ID.Compare(varID) == 0 {
			sourcePos = i
		}
		if v.ID.Compare(targetVarID) == 0 {
			targetPos = i
		}
	}

	if sourcePos == -1 {
		return fmt.Errorf("source variable not found in environment")
	}
	if targetPos == -1 {
		return fmt.Errorf("target variable not found in environment")
	}

	if sourcePos == targetPos {
		return fmt.Errorf("cannot move variable relative to itself")
	}

	// Calculate new order: move source to be after target
	newOrder := make([]idwrap.IDWrap, 0, len(variables))

	for i, v := range variables {
		if i == sourcePos {
			continue // Skip source variable
		}
		newOrder = append(newOrder, v.ID)
		if i == targetPos {
			newOrder = append(newOrder, varID) // Insert source after target
		}
	}

	// Reorder variables
	return service.ReorderVariablesTX(ctx, tx, sourceEnvID, newOrder)
}

// MoveVariableBefore moves a variable to be positioned before the target variable
func (e VarService) MoveVariableBefore(ctx context.Context, varID, targetVarID idwrap.IDWrap) error {
	return e.MoveVariableBeforeTX(ctx, nil, varID, targetVarID)
}

// MoveVariableBeforeTX moves a variable to be positioned before the target variable within a transaction
func (e VarService) MoveVariableBeforeTX(ctx context.Context, tx *sql.Tx, varID, targetVarID idwrap.IDWrap) error {
	service := e
	if tx != nil {
		service = e.TX(tx)
	}

	// Validate the move operation
	if err := service.validateMoveOperation(ctx, varID, targetVarID); err != nil {
		return err
	}

	// Check environment boundaries
	if err := service.checkEnvironmentBoundaries(ctx, varID, targetVarID); err != nil {
		return err
	}

	// Get environment ID for both variables
	sourceEnvID, err := service.GetEnvID(ctx, varID)
	if err != nil {
		return fmt.Errorf("failed to get source variable environment: %w", err)
	}

	// Get all variables in the environment in order
	variables, err := service.GetVariablesByEnvIDOrdered(ctx, sourceEnvID)
	if err != nil {
		return fmt.Errorf("failed to get variables in order: %w", err)
	}

	// Find positions of source and target variables
	var sourcePos, targetPos int = -1, -1
	for i, v := range variables {
		if v.ID.Compare(varID) == 0 {
			sourcePos = i
		}
		if v.ID.Compare(targetVarID) == 0 {
			targetPos = i
		}
	}

	if sourcePos == -1 {
		return fmt.Errorf("source variable not found in environment")
	}
	if targetPos == -1 {
		return fmt.Errorf("target variable not found in environment")
	}

	if sourcePos == targetPos {
		return fmt.Errorf("cannot move variable relative to itself")
	}

	// Calculate new order: move source to be before target
	newOrder := make([]idwrap.IDWrap, 0, len(variables))

	for i, v := range variables {
		if i == sourcePos {
			continue // Skip source variable
		}
		if i == targetPos {
			newOrder = append(newOrder, varID) // Insert source before target
		}
		newOrder = append(newOrder, v.ID)
	}

	// Reorder variables
	return service.ReorderVariablesTX(ctx, tx, sourceEnvID, newOrder)
}

// ReorderVariables performs a bulk reorder of variables using the movable system
func (e VarService) ReorderVariables(ctx context.Context, envID idwrap.IDWrap, orderedIDs []idwrap.IDWrap) error {
	return e.ReorderVariablesTX(ctx, nil, envID, orderedIDs)
}

// ReorderVariablesTX performs a bulk reorder of variables using the movable system within a transaction
func (e VarService) ReorderVariablesTX(ctx context.Context, tx *sql.Tx, envID idwrap.IDWrap, orderedIDs []idwrap.IDWrap) error {
	service := e
	if tx != nil {
		service = e.TX(tx)
	}

	// Build position updates using the environment list type
	updates := make([]movable.PositionUpdate, len(orderedIDs))
	for i, id := range orderedIDs {
		updates[i] = movable.PositionUpdate{
			ItemID:   id,
			ListType: movable.WorkspaceListTypeVariables, // Variables within an environment
			Position: i,
		}
	}

	// Execute the batch update using the movable repository
	if err := service.movableRepository.UpdatePositions(ctx, tx, updates); err != nil {
		return fmt.Errorf("failed to reorder variables: %w", err)
	}

	e.logger.DebugContext(ctx, "Variables reordered", 
		"envID", envID.String(),
		"variableCount", len(orderedIDs))

	return nil
}

// CompactVariablePositions recalculates and compacts position values to eliminate gaps
func (e VarService) CompactVariablePositions(ctx context.Context, envID idwrap.IDWrap) error {
	return e.CompactVariablePositionsTX(ctx, nil, envID)
}

// CompactVariablePositionsTX recalculates and compacts position values within a transaction
func (e VarService) CompactVariablePositionsTX(ctx context.Context, tx *sql.Tx, envID idwrap.IDWrap) error {
	service := e
	if tx != nil {
		service = e.TX(tx)
	}

	if err := service.linkedListManager.CompactPositions(ctx, tx, envID, movable.WorkspaceListTypeVariables); err != nil {
		return fmt.Errorf("failed to compact variable positions: %w", err)
	}

	e.logger.DebugContext(ctx, "Variable positions compacted", "envID", envID.String())
	return nil
}
