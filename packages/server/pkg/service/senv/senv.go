package senv

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/movable"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

type EnvironmentService struct {
	queries              *gen.Queries
	logger               *slog.Logger
	linkedListManager    movable.LinkedListManager
	movableRepository    movable.MovableRepository
}

// Type alias for backward compatibility
type EnvService = EnvironmentService

var (
	ErrNoEnvironmentFound         = sql.ErrNoRows
	ErrNoEnvFound                = sql.ErrNoRows  // Backward compatibility alias
	ErrInvalidMoveOperation       = fmt.Errorf("invalid move operation")
	ErrWorkspaceBoundaryViolation = fmt.Errorf("environments must be in same workspace")
	ErrSelfReferentialMove        = fmt.Errorf("cannot move environment relative to itself")
)

func New(queries *gen.Queries, logger *slog.Logger) EnvironmentService {
	// Create the movable repository for environments
	movableRepo := NewEnvironmentMovableRepository(queries)
	
	// Create the linked list manager with the movable repository
	linkedListManager := movable.NewDefaultLinkedListManager(movableRepo)
	
	return EnvironmentService{
		queries:              queries,
		logger:               logger,
		linkedListManager:    linkedListManager,
		movableRepository:    movableRepo,
	}
}

func (e EnvironmentService) TX(tx *sql.Tx) EnvironmentService {
	// Create new instances with transaction support
	txQueries := e.queries.WithTx(tx)
	movableRepo := NewEnvironmentMovableRepository(txQueries)
	linkedListManager := movable.NewDefaultLinkedListManager(movableRepo)
	
	return EnvironmentService{
		queries:              txQueries,
		logger:               e.logger,
		linkedListManager:    linkedListManager,
		movableRepository:    movableRepo,
	}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*EnvironmentService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	
	// Create movable repository and linked list manager
	movableRepo := NewEnvironmentMovableRepository(queries)
	linkedListManager := movable.NewDefaultLinkedListManager(movableRepo)
	
	// Use a default logger for transaction services
	logger := slog.Default()
	
	service := EnvironmentService{
		queries:              queries,
		logger:               logger,
		linkedListManager:    linkedListManager,
		movableRepository:    movableRepo,
	}
	return &service, nil
}

func ConvertToDBEnv(env menv.Env) gen.Environment {
	return gen.Environment{
		ID:          env.ID,
		WorkspaceID: env.WorkspaceID,
		Type:        int8(env.Type),
		Name:        env.Name,
		Description: env.Description,
	}
}

func ConvertToModelEnv(env gen.Environment) *menv.Env {
	return &menv.Env{
		ID:          env.ID,
		WorkspaceID: env.WorkspaceID,
		Type:        menv.EnvType(env.Type),
		Name:        env.Name,
		Description: env.Description,
	}
}

func (e EnvironmentService) GetEnvironment(ctx context.Context, id idwrap.IDWrap) (*menv.Env, error) {
	env, err := e.queries.GetEnvironment(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			e.logger.DebugContext(ctx, fmt.Sprintf("EnvironmentID: %s not found", id.String()))
			return nil, ErrNoEnvironmentFound
		}
		return nil, err
	}
	return ConvertToModelEnv(env), nil
}

func (e EnvironmentService) ListEnvironments(ctx context.Context, workspaceID idwrap.IDWrap) ([]menv.Env, error) {
	envs, err := e.queries.GetEnvironmentsByWorkspaceID(ctx, workspaceID)
	if err != nil {
		if err == sql.ErrNoRows {
			e.logger.InfoContext(ctx, fmt.Sprintf("workspaceID: %s not found", workspaceID.String()))
			return nil, ErrNoEnvironmentFound
		}
		return nil, err
	}
	return tgeneric.MassConvertPtr(envs, ConvertToModelEnv), nil
}

func (e EnvironmentService) CreateEnvironment(ctx context.Context, env *menv.Env) error {
	// Find the current tail of the linked list (last environment in workspace)
	existingEnvironments, err := e.GetEnvironmentsByWorkspaceIDOrdered(ctx, env.WorkspaceID)
	if err != nil && err != ErrNoEnvironmentFound {
		return fmt.Errorf("failed to get existing environments: %w", err)
	}
	
	var prev *idwrap.IDWrap
	
	// If there are existing environments, set prev to point to the last one
	// and update that environment's next pointer to point to the new one
	if len(existingEnvironments) > 0 {
		lastEnvironment := existingEnvironments[len(existingEnvironments)-1]
		prev = &lastEnvironment.ID
		
		// Get the database version to access prev/next fields
		dbLastEnvironment, err := e.queries.GetEnvironment(ctx, lastEnvironment.ID)
		if err != nil {
			return fmt.Errorf("failed to get last environment from database: %w", err)
		}
		
		// Update the current tail's next pointer to point to new environment
		err = e.queries.UpdateEnvironmentOrder(ctx, gen.UpdateEnvironmentOrderParams{
			Prev:        dbLastEnvironment.Prev, // Keep existing prev pointer
			Next:        &env.ID,                // Set next to new environment
			ID:          lastEnvironment.ID,     // Update the last environment
			WorkspaceID: env.WorkspaceID,
		})
		if err != nil {
			return fmt.Errorf("failed to update previous tail environment: %w", err)
		}
	}
	
	// Create the environment with proper linked list pointers
	return e.queries.CreateEnvironment(ctx, gen.CreateEnvironmentParams{
		ID:          env.ID,
		WorkspaceID: env.WorkspaceID,
		Type:        int8(env.Type),
		Name:        env.Name,
		Description: env.Description,
		Prev:        prev, // Points to current tail (or nil if first)
		Next:        nil,  // Always nil for new environments (they become the new tail)
	})
}

func (e EnvironmentService) UpdateEnvironment(ctx context.Context, env *menv.Env) error {
	dbEnv := ConvertToDBEnv(*env)
	return e.queries.UpdateEnvironment(ctx, gen.UpdateEnvironmentParams{
		ID:          dbEnv.ID,
		Name:        dbEnv.Name,
		Description: dbEnv.Description,
	})
}

func (e EnvironmentService) DeleteEnvironment(ctx context.Context, id idwrap.IDWrap) error {
	return e.DeleteEnvironmentTX(ctx, nil, id)
}

// Backward compatibility methods for the RPC layer

// Create provides backward compatibility for the RPC layer
func (e EnvironmentService) Create(ctx context.Context, env menv.Env) error {
	return e.CreateEnvironment(ctx, &env)
}

// Get provides backward compatibility for the RPC layer
func (e EnvironmentService) Get(ctx context.Context, id idwrap.IDWrap) (*menv.Env, error) {
	return e.GetEnvironment(ctx, id)
}

// GetByWorkspace provides backward compatibility for the RPC layer
func (e EnvironmentService) GetByWorkspace(ctx context.Context, workspaceID idwrap.IDWrap) ([]menv.Env, error) {
	return e.GetEnvironmentsByWorkspaceIDOrdered(ctx, workspaceID)
}

// Update provides backward compatibility for the RPC layer
func (e EnvironmentService) Update(ctx context.Context, env *menv.Env) error {
	return e.UpdateEnvironment(ctx, env)
}

// Delete provides backward compatibility for the RPC layer
func (e EnvironmentService) Delete(ctx context.Context, id idwrap.IDWrap) error {
	return e.DeleteEnvironment(ctx, id)
}

// DeleteEnvironmentTX deletes an environment while maintaining linked-list integrity within a transaction
func (e EnvironmentService) DeleteEnvironmentTX(ctx context.Context, tx *sql.Tx, id idwrap.IDWrap) error {
	service := e
	if tx != nil {
		service = e.TX(tx)
	}
	
	// 1. Get the environment being deleted to find its prev/next pointers
	env, err := service.queries.GetEnvironment(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			e.logger.DebugContext(ctx, fmt.Sprintf("Environment %s not found for deletion", id.String()))
			return ErrNoEnvironmentFound
		}
		return fmt.Errorf("failed to get environment for deletion: %w", err)
	}
	
	// 2. Fix linked-list pointers before deletion
	// Update prev environment's next pointer to skip the deleted environment
	if env.Prev != nil {
		err = service.queries.UpdateEnvironmentNext(ctx, gen.UpdateEnvironmentNextParams{
			Next:        env.Next,      // Point to the deleted environment's next
			ID:          *env.Prev,
			WorkspaceID: env.WorkspaceID,
		})
		if err != nil {
			return fmt.Errorf("failed to update previous environment's next pointer: %w", err)
		}
	}
	
	// Update next environment's prev pointer to skip the deleted environment
	if env.Next != nil {
		err = service.queries.UpdateEnvironmentPrev(ctx, gen.UpdateEnvironmentPrevParams{
			Prev:        env.Prev,      // Point to the deleted environment's prev
			ID:          *env.Next,
			WorkspaceID: env.WorkspaceID,
		})
		if err != nil {
			return fmt.Errorf("failed to update next environment's prev pointer: %w", err)
		}
	}
	
	// 3. Now safely delete the environment
	err = service.queries.DeleteEnvironment(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete environment after fixing linked-list: %w", err)
	}
	
	e.logger.DebugContext(ctx, "Environment deleted with linked-list integrity maintained", 
		"environmentID", id.String())
	
	return nil
}

func (e EnvironmentService) GetWorkspaceID(ctx context.Context, envID idwrap.IDWrap) (idwrap.IDWrap, error) {
	ulidData, err := e.queries.GetEnvironmentWorkspaceID(ctx, envID)
	if err != nil {
		if err == sql.ErrNoRows {
			return idwrap.IDWrap{}, ErrNoEnvironmentFound
		}
		return idwrap.IDWrap{}, err
	}
	return ulidData, nil
}

func (e EnvironmentService) CheckWorkspaceID(ctx context.Context, envID, ownerID idwrap.IDWrap) (bool, error) {
	environmentWorkspaceID, err := e.GetWorkspaceID(ctx, envID)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, ErrNoEnvironmentFound
		}
		return false, err
	}
	return ownerID.Compare(environmentWorkspaceID) == 0, nil
}

// Movable operations for environments

// MoveEnvironmentAfter moves an environment to be positioned after the target environment
func (e EnvironmentService) MoveEnvironmentAfter(ctx context.Context, envID, targetEnvID idwrap.IDWrap) error {
	return e.MoveEnvironmentAfterTX(ctx, nil, envID, targetEnvID)
}

// MoveEnvironmentAfterTX moves an environment to be positioned after the target environment within a transaction
func (e EnvironmentService) MoveEnvironmentAfterTX(ctx context.Context, tx *sql.Tx, envID, targetEnvID idwrap.IDWrap) error {
	service := e
	if tx != nil {
		service = e.TX(tx)
	}
	
	// Get workspace ID for both environments to ensure they're in the same workspace
	sourceWorkspaceID, err := service.GetWorkspaceID(ctx, envID)
	if err != nil {
		return fmt.Errorf("failed to get source environment workspace: %w", err)
	}
	
	targetWorkspaceID, err := service.GetWorkspaceID(ctx, targetEnvID)
	if err != nil {
		return fmt.Errorf("failed to get target environment workspace: %w", err)
	}
	
	if sourceWorkspaceID.Compare(targetWorkspaceID) != 0 {
		return fmt.Errorf("environments must be in the same workspace")
	}
	
	// Get all environments in the workspace in order
	environments, err := service.GetEnvironmentsByWorkspaceIDOrdered(ctx, sourceWorkspaceID)
	if err != nil {
		return fmt.Errorf("failed to get environments in order: %w", err)
	}
	
	// Find positions of source and target environments
	var sourcePos, targetPos int = -1, -1
	for i, env := range environments {
		if env.ID.Compare(envID) == 0 {
			sourcePos = i
		}
		if env.ID.Compare(targetEnvID) == 0 {
			targetPos = i
		}
	}
	
	if sourcePos == -1 {
		return fmt.Errorf("source environment not found in workspace")
	}
	if targetPos == -1 {
		return fmt.Errorf("target environment not found in workspace")
	}
	
	if sourcePos == targetPos {
		return fmt.Errorf("cannot move environment relative to itself")
	}
	
	// Calculate new order: move source to be after target
	newOrder := make([]idwrap.IDWrap, 0, len(environments))
	
	for i, env := range environments {
		if i == sourcePos {
			continue // Skip source environment
		}
		newOrder = append(newOrder, env.ID)
		if i == targetPos {
			newOrder = append(newOrder, envID) // Insert source after target
		}
	}
	
	// Reorder environments
	return service.ReorderEnvironmentsTX(ctx, tx, sourceWorkspaceID, newOrder)
}

// MoveEnvironmentBefore moves an environment to be positioned before the target environment
func (e EnvironmentService) MoveEnvironmentBefore(ctx context.Context, envID, targetEnvID idwrap.IDWrap) error {
	return e.MoveEnvironmentBeforeTX(ctx, nil, envID, targetEnvID)
}

// MoveEnvironmentBeforeTX moves an environment to be positioned before the target environment within a transaction
func (e EnvironmentService) MoveEnvironmentBeforeTX(ctx context.Context, tx *sql.Tx, envID, targetEnvID idwrap.IDWrap) error {
	service := e
	if tx != nil {
		service = e.TX(tx)
	}
	
	// Get workspace ID for both environments to ensure they're in the same workspace
	sourceWorkspaceID, err := service.GetWorkspaceID(ctx, envID)
	if err != nil {
		return fmt.Errorf("failed to get source environment workspace: %w", err)
	}
	
	targetWorkspaceID, err := service.GetWorkspaceID(ctx, targetEnvID)
	if err != nil {
		return fmt.Errorf("failed to get target environment workspace: %w", err)
	}
	
	if sourceWorkspaceID.Compare(targetWorkspaceID) != 0 {
		return fmt.Errorf("environments must be in the same workspace")
	}
	
	// Get all environments in the workspace in order
	environments, err := service.GetEnvironmentsByWorkspaceIDOrdered(ctx, sourceWorkspaceID)
	if err != nil {
		return fmt.Errorf("failed to get environments in order: %w", err)
	}
	
	// Find positions of source and target environments
	var sourcePos, targetPos int = -1, -1
	for i, env := range environments {
		if env.ID.Compare(envID) == 0 {
			sourcePos = i
		}
		if env.ID.Compare(targetEnvID) == 0 {
			targetPos = i
		}
	}
	
	if sourcePos == -1 {
		return fmt.Errorf("source environment not found in workspace")
	}
	if targetPos == -1 {
		return fmt.Errorf("target environment not found in workspace")
	}
	
	if sourcePos == targetPos {
		return fmt.Errorf("cannot move environment relative to itself")
	}
	
	// Calculate new order: move source to be before target
	newOrder := make([]idwrap.IDWrap, 0, len(environments))
	
	for i, env := range environments {
		if i == sourcePos {
			continue // Skip source environment
		}
		if i == targetPos {
			newOrder = append(newOrder, envID) // Insert source before target
		}
		newOrder = append(newOrder, env.ID)
	}
	
	// Reorder environments
	return service.ReorderEnvironmentsTX(ctx, tx, sourceWorkspaceID, newOrder)
}

// GetEnvironmentsByWorkspaceIDOrdered returns environments in the workspace in their proper order
func (e EnvironmentService) GetEnvironmentsByWorkspaceIDOrdered(ctx context.Context, workspaceID idwrap.IDWrap) ([]menv.Env, error) {
	// Use the underlying query that maintains the linked list order
	orderedEnvironments, err := e.queries.GetEnvironmentsByWorkspaceIDOrdered(ctx, gen.GetEnvironmentsByWorkspaceIDOrderedParams{
		WorkspaceID:   workspaceID,
		WorkspaceID_2: workspaceID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			e.logger.InfoContext(ctx, fmt.Sprintf("workspaceID: %s has no environments", workspaceID.String()))
			return []menv.Env{}, nil
		}
		return nil, err
	}
	
	// Convert to model environments
	environments := make([]menv.Env, len(orderedEnvironments))
	for i, env := range orderedEnvironments {
		environments[i] = menv.Env{
			ID:          idwrap.NewFromBytesMust(env.ID),
			WorkspaceID: idwrap.NewFromBytesMust(env.WorkspaceID),
			Type:        menv.EnvType(env.Type),
			Name:        env.Name,
			Description: env.Description,
			// Note: Updated field not available in the ordered query result
			// If needed, we could make a separate query to get this field
		}
	}
	
	return environments, nil
}

// ReorderEnvironments performs a bulk reorder of environments using the movable system
func (e EnvironmentService) ReorderEnvironments(ctx context.Context, workspaceID idwrap.IDWrap, orderedIDs []idwrap.IDWrap) error {
	return e.ReorderEnvironmentsTX(ctx, nil, workspaceID, orderedIDs)
}

// ReorderEnvironmentsTX performs a bulk reorder of environments using the movable system within a transaction
func (e EnvironmentService) ReorderEnvironmentsTX(ctx context.Context, tx *sql.Tx, workspaceID idwrap.IDWrap, orderedIDs []idwrap.IDWrap) error {
	service := e
	if tx != nil {
		service = e.TX(tx)
	}
	
	// Build position updates
	updates := make([]movable.PositionUpdate, len(orderedIDs))
	for i, id := range orderedIDs {
		updates[i] = movable.PositionUpdate{
			ItemID:   id,
			ListType: movable.WorkspaceListTypeEnvironments,
			Position: i,
		}
	}
	
	// Execute the batch update using the movable repository
	if err := service.movableRepository.UpdatePositions(ctx, tx, updates); err != nil {
		return fmt.Errorf("failed to reorder environments: %w", err)
	}
	
	e.logger.DebugContext(ctx, "Environments reordered", 
		"workspaceID", workspaceID.String(),
		"environmentCount", len(orderedIDs))
	
	return nil
}

// CompactEnvironmentPositions recalculates and compacts position values to eliminate gaps
func (e EnvironmentService) CompactEnvironmentPositions(ctx context.Context, workspaceID idwrap.IDWrap) error {
	return e.CompactEnvironmentPositionsTX(ctx, nil, workspaceID)
}

// CompactEnvironmentPositionsTX recalculates and compacts position values within a transaction
func (e EnvironmentService) CompactEnvironmentPositionsTX(ctx context.Context, tx *sql.Tx, workspaceID idwrap.IDWrap) error {
	service := e
	if tx != nil {
		service = e.TX(tx)
	}
	
	if err := service.linkedListManager.CompactPositions(ctx, tx, workspaceID, movable.WorkspaceListTypeEnvironments); err != nil {
		return fmt.Errorf("failed to compact environment positions: %w", err)
	}
	
	e.logger.DebugContext(ctx, "Environment positions compacted", "workspaceID", workspaceID.String())
	return nil
}

// validateMoveOperation validates that a move operation is safe and valid
func (e EnvironmentService) validateMoveOperation(ctx context.Context, envID, targetEnvID idwrap.IDWrap) error {
	if envID.Compare(targetEnvID) == 0 {
		return ErrSelfReferentialMove
	}
	
	return nil
}

// checkWorkspaceBoundaries ensures both environments are in the same workspace
func (e EnvironmentService) checkWorkspaceBoundaries(ctx context.Context, envID, targetEnvID idwrap.IDWrap) error {
	sourceWorkspaceID, err := e.GetWorkspaceID(ctx, envID)
	if err != nil {
		return fmt.Errorf("failed to get source environment workspace: %w", err)
	}
	
	targetWorkspaceID, err := e.GetWorkspaceID(ctx, targetEnvID)
	if err != nil {
		return fmt.Errorf("failed to get target environment workspace: %w", err)
	}
	
	if sourceWorkspaceID.Compare(targetWorkspaceID) != 0 {
		return ErrWorkspaceBoundaryViolation
	}
	
	return nil
}
