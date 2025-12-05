package sflowvariable

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflowvariable"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

type FlowVariableService struct {
	queries *gen.Queries
}

var ErrNoFlowVariableFound = errors.New("no flow variable find")

func New(queries *gen.Queries) FlowVariableService {
	return FlowVariableService{
		queries: queries,
	}
}

func (s FlowVariableService) TX(tx *sql.Tx) FlowVariableService {
	return FlowVariableService{
		queries: s.queries.WithTx(tx),
	}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*FlowVariableService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}

	return &FlowVariableService{
		queries: queries,
	}, nil
}

func ConvertModelToDB(item mflowvariable.FlowVariable) gen.FlowVariable {
	return gen.FlowVariable{
		ID:           item.ID,
		FlowID:       item.FlowID,
		Key:          item.Name,
		Value:        item.Value,
		Enabled:      item.Enabled,
		Description:  item.Description,
		DisplayOrder: item.Order,
	}
}

func ConvertDBToModel(item gen.FlowVariable) mflowvariable.FlowVariable {
	return mflowvariable.FlowVariable{
		ID:          item.ID,
		FlowID:      item.FlowID,
		Name:        item.Key,
		Value:       item.Value,
		Enabled:     item.Enabled,
		Description: item.Description,
		Order:       item.DisplayOrder,
	}
}

func (s *FlowVariableService) GetFlowVariable(ctx context.Context, id idwrap.IDWrap) (mflowvariable.FlowVariable, error) {
	item, err := s.queries.GetFlowVariable(ctx, id)
	if err != nil {
		return mflowvariable.FlowVariable{}, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowVariableFound, err)
	}
	return ConvertDBToModel(item), nil
}

func (s *FlowVariableService) GetFlowVariablesByFlowID(ctx context.Context, flowID idwrap.IDWrap) ([]mflowvariable.FlowVariable, error) {
	items, err := s.queries.GetFlowVariablesByFlowID(ctx, flowID)
	if err != nil {
		return nil, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowVariableFound, err)
	}
	return tgeneric.MassConvert(items, ConvertDBToModel), nil
}

func (s *FlowVariableService) CreateFlowVariable(ctx context.Context, item mflowvariable.FlowVariable) error {
	arg := ConvertModelToDB(item)
	err := s.queries.CreateFlowVariable(ctx, gen.CreateFlowVariableParams{
		ID:           arg.ID,
		FlowID:       arg.FlowID,
		Key:          arg.Key,
		Value:        arg.Value,
		Enabled:      arg.Enabled,
		Description:  arg.Description,
		DisplayOrder: arg.DisplayOrder,
	})
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowVariableFound, err)
}

const sizeOfChunks = 10

func (s *FlowVariableService) CreateFlowVariableBulk(ctx context.Context, variables []mflowvariable.FlowVariable) error {
	for chunk := range slices.Chunk(variables, sizeOfChunks) {
		if len(chunk) < 10 {
			for _, variable := range chunk {
				err := s.CreateFlowVariable(ctx, variable)
				if err != nil {
					return err
				}
			}
			continue
		}

		// Convert all items to DB parameters
		dbItems := tgeneric.MassConvert(chunk, ConvertModelToDB)
		params := createBulkParams(dbItems)

		err := s.queries.CreateFlowVariableBulk(ctx, params)
		if err != nil {
			return err
		}
	}

	return nil
}

func createBulkParams(items []gen.FlowVariable) gen.CreateFlowVariableBulkParams {
	params := gen.CreateFlowVariableBulkParams{}

	// Position 1
	params.ID = items[0].ID
	params.FlowID = items[0].FlowID
	params.Key = items[0].Key
	params.Value = items[0].Value
	params.Enabled = items[0].Enabled
	params.Description = items[0].Description
	params.DisplayOrder = items[0].DisplayOrder

	// Position 2
	params.ID_2 = items[1].ID
	params.FlowID_2 = items[1].FlowID
	params.Key_2 = items[1].Key
	params.Value_2 = items[1].Value
	params.Enabled_2 = items[1].Enabled
	params.Description_2 = items[1].Description
	params.DisplayOrder_2 = items[1].DisplayOrder

	// Position 3
	params.ID_3 = items[2].ID
	params.FlowID_3 = items[2].FlowID
	params.Key_3 = items[2].Key
	params.Value_3 = items[2].Value
	params.Enabled_3 = items[2].Enabled
	params.Description_3 = items[2].Description
	params.DisplayOrder_3 = items[2].DisplayOrder

	// Position 4
	params.ID_4 = items[3].ID
	params.FlowID_4 = items[3].FlowID
	params.Key_4 = items[3].Key
	params.Value_4 = items[3].Value
	params.Enabled_4 = items[3].Enabled
	params.Description_4 = items[3].Description
	params.DisplayOrder_4 = items[3].DisplayOrder

	// Position 5
	params.ID_5 = items[4].ID
	params.FlowID_5 = items[4].FlowID
	params.Key_5 = items[4].Key
	params.Value_5 = items[4].Value
	params.Enabled_5 = items[4].Enabled
	params.Description_5 = items[4].Description
	params.DisplayOrder_5 = items[4].DisplayOrder

	// Position 6
	params.ID_6 = items[5].ID
	params.FlowID_6 = items[5].FlowID
	params.Key_6 = items[5].Key
	params.Value_6 = items[5].Value
	params.Enabled_6 = items[5].Enabled
	params.Description_6 = items[5].Description
	params.DisplayOrder_6 = items[5].DisplayOrder

	// Position 7
	params.ID_7 = items[6].ID
	params.FlowID_7 = items[6].FlowID
	params.Key_7 = items[6].Key
	params.Value_7 = items[6].Value
	params.Enabled_7 = items[6].Enabled
	params.Description_7 = items[6].Description
	params.DisplayOrder_7 = items[6].DisplayOrder

	// Position 8
	params.ID_8 = items[7].ID
	params.FlowID_8 = items[7].FlowID
	params.Key_8 = items[7].Key
	params.Value_8 = items[7].Value
	params.Enabled_8 = items[7].Enabled
	params.Description_8 = items[7].Description
	params.DisplayOrder_8 = items[7].DisplayOrder

	// Position 9
	params.ID_9 = items[8].ID
	params.FlowID_9 = items[8].FlowID
	params.Key_9 = items[8].Key
	params.Value_9 = items[8].Value
	params.Enabled_9 = items[8].Enabled
	params.Description_9 = items[8].Description
	params.DisplayOrder_9 = items[8].DisplayOrder

	// Position 10
	params.ID_10 = items[9].ID
	params.FlowID_10 = items[9].FlowID
	params.Key_10 = items[9].Key
	params.Value_10 = items[9].Value
	params.Enabled_10 = items[9].Enabled
	params.Description_10 = items[9].Description
	params.DisplayOrder_10 = items[9].DisplayOrder

	return params
}

func (s *FlowVariableService) UpdateFlowVariable(ctx context.Context, item mflowvariable.FlowVariable) error {
	err := s.queries.UpdateFlowVariable(ctx, gen.UpdateFlowVariableParams{
		ID:          item.ID,
		Key:         item.Name,
		Value:       item.Value,
		Enabled:     item.Enabled,
		Description: item.Description,
	})
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowVariableFound, err)
}

func (s *FlowVariableService) DeleteFlowVariable(ctx context.Context, id idwrap.IDWrap) error {
	err := s.queries.DeleteFlowVariable(ctx, id)
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowVariableFound, err)
}

// GetFlowVariablesByFlowIDOrdered returns flow variables in the flow ordered by display_order
func (s *FlowVariableService) GetFlowVariablesByFlowIDOrdered(ctx context.Context, flowID idwrap.IDWrap) ([]mflowvariable.FlowVariable, error) {
	items, err := s.queries.GetFlowVariablesByFlowIDOrdered(ctx, flowID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mflowvariable.FlowVariable{}, nil
		}
		return nil, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowVariableFound, err)
	}

	return tgeneric.MassConvert(items, ConvertDBToModel), nil
}

// UpdateFlowVariableOrder updates the display_order for a single flow variable
func (s *FlowVariableService) UpdateFlowVariableOrder(ctx context.Context, id idwrap.IDWrap, order float64) error {
	err := s.queries.UpdateFlowVariableOrder(ctx, gen.UpdateFlowVariableOrderParams{
		ID:           id,
		DisplayOrder: order,
	})
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowVariableFound, err)
}

// validateMoveOperation validates that a move operation is safe and valid
func (s *FlowVariableService) validateMoveOperation(ctx context.Context, variableID, targetVariableID idwrap.IDWrap) error {
	if variableID.Compare(targetVariableID) == 0 {
		return errors.New("cannot move flow variable relative to itself")
	}

	return nil
}

// checkFlowBoundaries ensures both flow variables are in the same flow
func (s *FlowVariableService) checkFlowBoundaries(ctx context.Context, variableID, targetVariableID idwrap.IDWrap) error {
	sourceVariable, err := s.GetFlowVariable(ctx, variableID)
	if err != nil {
		return fmt.Errorf("failed to get source flow variable: %w", err)
	}

	targetVariable, err := s.GetFlowVariable(ctx, targetVariableID)
	if err != nil {
		return fmt.Errorf("failed to get target flow variable: %w", err)
	}

	if sourceVariable.FlowID.Compare(targetVariable.FlowID) != 0 {
		return errors.New("flow variables must be in the same flow")
	}

	return nil
}

// MoveFlowVariableAfter moves a flow variable to be positioned after the target variable
func (s *FlowVariableService) MoveFlowVariableAfter(ctx context.Context, variableID, targetVariableID idwrap.IDWrap) error {
	return s.MoveFlowVariableAfterTX(ctx, nil, variableID, targetVariableID)
}

// MoveFlowVariableAfterTX moves a flow variable to be positioned after the target variable within a transaction
func (s *FlowVariableService) MoveFlowVariableAfterTX(ctx context.Context, tx *sql.Tx, variableID, targetVariableID idwrap.IDWrap) error {
	service := *s
	if tx != nil {
		service = s.TX(tx)
	}

	// Validate the move operation
	if err := service.validateMoveOperation(ctx, variableID, targetVariableID); err != nil {
		return err
	}

	// Check flow boundaries
	if err := service.checkFlowBoundaries(ctx, variableID, targetVariableID); err != nil {
		return err
	}

	// Get flow ID for both variables
	sourceVariable, err := service.GetFlowVariable(ctx, variableID)
	if err != nil {
		return fmt.Errorf("failed to get source flow variable: %w", err)
	}

	// Get all flow variables in the flow in order
	variables, err := service.GetFlowVariablesByFlowIDOrdered(ctx, sourceVariable.FlowID)
	if err != nil {
		return fmt.Errorf("failed to get flow variables in order: %w", err)
	}

	// Find positions of source and target variables
	var sourcePos, targetPos int = -1, -1
	for i, v := range variables {
		if v.ID.Compare(variableID) == 0 {
			sourcePos = i
		}
		if v.ID.Compare(targetVariableID) == 0 {
			targetPos = i
		}
	}

	if sourcePos == -1 {
		return fmt.Errorf("source flow variable not found in flow")
	}
	if targetPos == -1 {
		return fmt.Errorf("target flow variable not found in flow")
	}

	if sourcePos == targetPos {
		return fmt.Errorf("cannot move flow variable relative to itself")
	}

	// Calculate new order: move source to be after target
	newOrder := make([]idwrap.IDWrap, 0, len(variables))

	for i, v := range variables {
		if i == sourcePos {
			continue // Skip source variable
		}
		newOrder = append(newOrder, v.ID)
		if i == targetPos {
			newOrder = append(newOrder, variableID) // Insert source after target
		}
	}

	// Reorder flow variables
	return service.ReorderFlowVariablesTX(ctx, tx, newOrder)
}

// MoveFlowVariableBefore moves a flow variable to be positioned before the target variable
func (s *FlowVariableService) MoveFlowVariableBefore(ctx context.Context, variableID, targetVariableID idwrap.IDWrap) error {
	return s.MoveFlowVariableBeforeTX(ctx, nil, variableID, targetVariableID)
}

// MoveFlowVariableBeforeTX moves a flow variable to be positioned before the target variable within a transaction
func (s *FlowVariableService) MoveFlowVariableBeforeTX(ctx context.Context, tx *sql.Tx, variableID, targetVariableID idwrap.IDWrap) error {
	service := *s
	if tx != nil {
		service = s.TX(tx)
	}

	// Validate the move operation
	if err := service.validateMoveOperation(ctx, variableID, targetVariableID); err != nil {
		return err
	}

	// Check flow boundaries
	if err := service.checkFlowBoundaries(ctx, variableID, targetVariableID); err != nil {
		return err
	}

	// Get flow ID for both variables
	sourceVariable, err := service.GetFlowVariable(ctx, variableID)
	if err != nil {
		return fmt.Errorf("failed to get source flow variable: %w", err)
	}

	// Get all flow variables in the flow in order
	variables, err := service.GetFlowVariablesByFlowIDOrdered(ctx, sourceVariable.FlowID)
	if err != nil {
		return fmt.Errorf("failed to get flow variables in order: %w", err)
	}

	// Find positions of source and target variables
	var sourcePos, targetPos int = -1, -1
	for i, v := range variables {
		if v.ID.Compare(variableID) == 0 {
			sourcePos = i
		}
		if v.ID.Compare(targetVariableID) == 0 {
			targetPos = i
		}
	}

	if sourcePos == -1 {
		return fmt.Errorf("source flow variable not found in flow")
	}
	if targetPos == -1 {
		return fmt.Errorf("target flow variable not found in flow")
	}

	if sourcePos == targetPos {
		return fmt.Errorf("cannot move flow variable relative to itself")
	}

	// Calculate new order: move source to be before target
	newOrder := make([]idwrap.IDWrap, 0, len(variables))

	for i, v := range variables {
		if i == sourcePos {
			continue // Skip source variable
		}
		if i == targetPos {
			newOrder = append(newOrder, variableID) // Insert source before target
		}
		newOrder = append(newOrder, v.ID)
	}

	// Reorder flow variables
	return service.ReorderFlowVariablesTX(ctx, tx, newOrder)
}

// ReorderFlowVariables performs a bulk reorder of flow variables by updating their display_order
func (s *FlowVariableService) ReorderFlowVariables(ctx context.Context, orderedIDs []idwrap.IDWrap) error {
	return s.ReorderFlowVariablesTX(ctx, nil, orderedIDs)
}

// ReorderFlowVariablesTX performs a bulk reorder of flow variables within a transaction
func (s *FlowVariableService) ReorderFlowVariablesTX(ctx context.Context, tx *sql.Tx, orderedIDs []idwrap.IDWrap) error {
	service := *s
	if tx != nil {
		service = s.TX(tx)
	}

	// Update display_order for each flow variable based on its position in the slice
	for i, id := range orderedIDs {
		err := service.queries.UpdateFlowVariableOrder(ctx, gen.UpdateFlowVariableOrderParams{
			ID:           id,
			DisplayOrder: float64(i),
		})
		if err != nil {
			return fmt.Errorf("failed to update flow variable order: %w", err)
		}
	}

	return nil
}
