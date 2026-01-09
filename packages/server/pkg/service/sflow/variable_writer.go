package sflow

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/tgeneric"
	"slices"
)

type FlowVariableWriter struct {
	queries *gen.Queries
	reader  *FlowVariableReader
}

func NewFlowVariableWriter(tx gen.DBTX) *FlowVariableWriter {
	// Create queries from TX
	queries := gen.New(tx)
	// Create internal reader using the same queries (and thus same TX)
	reader := NewFlowVariableReaderFromQueries(queries)
	return &FlowVariableWriter{
		queries: queries,
		reader:  reader,
	}
}

func NewFlowVariableWriterFromQueries(queries *gen.Queries) *FlowVariableWriter {
	reader := NewFlowVariableReaderFromQueries(queries)
	return &FlowVariableWriter{
		queries: queries,
		reader:  reader,
	}
}

func (w *FlowVariableWriter) CreateFlowVariable(ctx context.Context, item mflow.FlowVariable) error {
	arg := ConvertFlowVariableToDB(item)
	err := w.queries.CreateFlowVariable(ctx, gen.CreateFlowVariableParams(arg))
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowVariableFound, err)
}

const sizeOfChunks = 10

func (w *FlowVariableWriter) CreateFlowVariableBulk(ctx context.Context, variables []mflow.FlowVariable) error {
	for chunk := range slices.Chunk(variables, sizeOfChunks) {
		if len(chunk) < 10 {
			for _, variable := range chunk {
				err := w.CreateFlowVariable(ctx, variable)
				if err != nil {
					return err
				}
			}
			continue
		}

		// Convert all items to DB parameters
		dbItems := tgeneric.MassConvert(chunk, ConvertFlowVariableToDB)
		params := createBulkParams(dbItems)

		err := w.queries.CreateFlowVariableBulk(ctx, params)
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

func (w *FlowVariableWriter) UpdateFlowVariable(ctx context.Context, item mflow.FlowVariable) error {
	err := w.queries.UpdateFlowVariable(ctx, gen.UpdateFlowVariableParams{
		ID:          item.ID,
		Key:         item.Name,
		Value:       item.Value,
		Enabled:     item.Enabled,
		Description: item.Description,
	})
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowVariableFound, err)
}

func (w *FlowVariableWriter) DeleteFlowVariable(ctx context.Context, id idwrap.IDWrap) error {
	err := w.queries.DeleteFlowVariable(ctx, id)
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowVariableFound, err)
}

// UpdateFlowVariableOrder updates the display_order for a single flow variable
func (w *FlowVariableWriter) UpdateFlowVariableOrder(ctx context.Context, id idwrap.IDWrap, order float64) error {
	err := w.queries.UpdateFlowVariableOrder(ctx, gen.UpdateFlowVariableOrderParams{
		ID:           id,
		DisplayOrder: order,
	})
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowVariableFound, err)
}

// MoveFlowVariableAfter moves a flow variable to be positioned after the target variable
func (w *FlowVariableWriter) MoveFlowVariableAfter(ctx context.Context, variableID, targetVariableID idwrap.IDWrap) error {
	// Validate the move operation
	if variableID.Compare(targetVariableID) == 0 {
		return errors.New("cannot move flow variable relative to itself")
	}

	// Check flow boundaries using internal reader
	if err := w.checkFlowBoundaries(ctx, variableID, targetVariableID); err != nil {
		return err
	}

	// Get flow ID for both variables
	sourceVariable, err := w.reader.GetFlowVariable(ctx, variableID)
	if err != nil {
		return fmt.Errorf("failed to get source flow variable: %w", err)
	}

	// Get all flow variables in the flow in order
	variables, err := w.reader.GetFlowVariablesByFlowIDOrdered(ctx, sourceVariable.FlowID)
	if err != nil {
		return fmt.Errorf("failed to get flow variables in order: %w", err)
	}

	// Find positions of source and target variables
	var sourcePos, targetPos = -1, -1
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
	return w.ReorderFlowVariables(ctx, newOrder)
}

// MoveFlowVariableBefore moves a flow variable to be positioned before the target variable
func (w *FlowVariableWriter) MoveFlowVariableBefore(ctx context.Context, variableID, targetVariableID idwrap.IDWrap) error {
	// Validate the move operation
	if variableID.Compare(targetVariableID) == 0 {
		return errors.New("cannot move flow variable relative to itself")
	}

	// Check flow boundaries using internal reader
	if err := w.checkFlowBoundaries(ctx, variableID, targetVariableID); err != nil {
		return err
	}

	// Get flow ID for both variables
	sourceVariable, err := w.reader.GetFlowVariable(ctx, variableID)
	if err != nil {
		return fmt.Errorf("failed to get source flow variable: %w", err)
	}

	// Get all flow variables in the flow in order
	variables, err := w.reader.GetFlowVariablesByFlowIDOrdered(ctx, sourceVariable.FlowID)
	if err != nil {
		return fmt.Errorf("failed to get flow variables in order: %w", err)
	}

	// Find positions of source and target variables
	var sourcePos, targetPos = -1, -1
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
	return w.ReorderFlowVariables(ctx, newOrder)
}

// ReorderFlowVariables performs a bulk reorder of flow variables by updating their display_order
func (w *FlowVariableWriter) ReorderFlowVariables(ctx context.Context, orderedIDs []idwrap.IDWrap) error {
	// Update display_order for each flow variable based on its position in the slice
	for i, id := range orderedIDs {
		err := w.queries.UpdateFlowVariableOrder(ctx, gen.UpdateFlowVariableOrderParams{
			ID:           id,
			DisplayOrder: float64(i),
		})
		if err != nil {
			return fmt.Errorf("failed to update flow variable order: %w", err)
		}
	}

	return nil
}

// checkFlowBoundaries ensures both flow variables are in the same flow
func (w *FlowVariableWriter) checkFlowBoundaries(ctx context.Context, variableID, targetVariableID idwrap.IDWrap) error {
	sourceVariable, err := w.reader.GetFlowVariable(ctx, variableID)
	if err != nil {
		return fmt.Errorf("failed to get source flow variable: %w", err)
	}

	targetVariable, err := w.reader.GetFlowVariable(ctx, targetVariableID)
	if err != nil {
		return fmt.Errorf("failed to get target flow variable: %w", err)
	}

	if sourceVariable.FlowID.Compare(targetVariable.FlowID) != 0 {
		return errors.New("flow variables must be in the same flow")
	}

	return nil
}
