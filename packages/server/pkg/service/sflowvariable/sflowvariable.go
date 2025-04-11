package sflowvariable

import (
	"context"
	"database/sql"
	"errors"
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
	return FlowVariableService{queries: queries}
}

func (s FlowVariableService) TX(tx *sql.Tx) FlowVariableService {
	return FlowVariableService{queries: s.queries.WithTx(tx)}
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
		ID:          item.ID,
		FlowID:      item.FlowID,
		Key:         item.Name,
		Value:       item.Value,
		Enabled:     item.Enabled,
		Description: item.Description,
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
		ID:          arg.ID,
		FlowID:      arg.FlowID,
		Key:         arg.Key,
		Value:       arg.Value,
		Enabled:     arg.Enabled,
		Description: arg.Description,
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
		params := s.createBulkParams(dbItems)

		err := s.queries.CreateFlowVariableBulk(ctx, params)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *FlowVariableService) createBulkParams(items []gen.FlowVariable) gen.CreateFlowVariableBulkParams {
	params := gen.CreateFlowVariableBulkParams{}

	// Directly assign each position instead of using a loop
	// Position 1
	params.ID = items[0].ID
	params.FlowID = items[0].FlowID
	params.Key = items[0].Key
	params.Value = items[0].Value
	params.Enabled = items[0].Enabled
	params.Description = items[0].Description

	// Position 2
	params.ID_2 = items[1].ID
	params.FlowID_2 = items[1].FlowID
	params.Key_2 = items[1].Key
	params.Value_2 = items[1].Value
	params.Enabled_2 = items[1].Enabled
	params.Description_2 = items[1].Description

	// Position 3
	params.ID_3 = items[2].ID
	params.FlowID_3 = items[2].FlowID
	params.Key_3 = items[2].Key
	params.Value_3 = items[2].Value
	params.Enabled_3 = items[2].Enabled
	params.Description_3 = items[2].Description

	// Position 4
	params.ID_4 = items[3].ID
	params.FlowID_4 = items[3].FlowID
	params.Key_4 = items[3].Key
	params.Value_4 = items[3].Value
	params.Enabled_4 = items[3].Enabled
	params.Description_4 = items[3].Description

	// Position 5
	params.ID_5 = items[4].ID
	params.FlowID_5 = items[4].FlowID
	params.Key_5 = items[4].Key
	params.Value_5 = items[4].Value
	params.Enabled_5 = items[4].Enabled
	params.Description_5 = items[4].Description

	// Position 6
	params.ID_6 = items[5].ID
	params.FlowID_6 = items[5].FlowID
	params.Key_6 = items[5].Key
	params.Value_6 = items[5].Value
	params.Enabled_6 = items[5].Enabled
	params.Description_6 = items[5].Description

	// Position 7
	params.ID_7 = items[6].ID
	params.FlowID_7 = items[6].FlowID
	params.Key_7 = items[6].Key
	params.Value_7 = items[6].Value
	params.Enabled_7 = items[6].Enabled
	params.Description_7 = items[6].Description

	// Position 8
	params.ID_8 = items[7].ID
	params.FlowID_8 = items[7].FlowID
	params.Key_8 = items[7].Key
	params.Value_8 = items[7].Value
	params.Enabled_8 = items[7].Enabled
	params.Description_8 = items[7].Description

	// Position 9
	params.ID_9 = items[8].ID
	params.FlowID_9 = items[8].FlowID
	params.Key_9 = items[8].Key
	params.Value_9 = items[8].Value
	params.Enabled_9 = items[8].Enabled
	params.Description_9 = items[8].Description

	// Position 10
	params.ID_10 = items[9].ID
	params.FlowID_10 = items[9].FlowID
	params.Key_10 = items[9].Key
	params.Value_10 = items[9].Value
	params.Enabled_10 = items[9].Enabled
	params.Description_10 = items[9].Description

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
