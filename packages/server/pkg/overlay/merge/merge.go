package merge

import (
	"context"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/service/soverlayheader"
	"the-dev-tools/server/pkg/service/soverlayquery"
)

const (
	refKindOrigin int8 = 1
	refKindDelta  int8 = 2
)

type Manager struct {
	header *soverlayheader.Service
	query  *soverlayquery.Service
}

func New(header *soverlayheader.Service, query *soverlayquery.Service) *Manager {
	return &Manager{header: header, query: query}
}

func (m *Manager) MergeHeaders(ctx context.Context, originHeaders []mexampleheader.Header, deltaHeaders []mexampleheader.Header, deltaExampleID idwrap.IDWrap) ([]mexampleheader.Header, error) {
	if m == nil || m.header == nil {
		return deltaHeaders, nil
	}

	orderRows, err := m.header.SelectAsc(ctx, deltaExampleID)
	if err != nil {
		return nil, err
	}

	originByID := make(map[string]mexampleheader.Header, len(originHeaders))
	for _, header := range originHeaders {
		originByID[header.ID.String()] = header
	}

	deltaByID := make(map[string]mexampleheader.Header, len(deltaHeaders))
	parentToDelta := make(map[string]mexampleheader.Header, len(deltaHeaders))
	for _, header := range deltaHeaders {
		deltaByID[header.ID.String()] = header
		if header.DeltaParentID != nil {
			parentToDelta[header.DeltaParentID.String()] = header
		}
	}

	finalHeaders := make([]mexampleheader.Header, 0, len(deltaHeaders))
	processed := make(map[string]struct{}, len(deltaHeaders))

	for _, row := range orderRows {
		refID, convErr := idwrap.NewFromBytes(row.RefID)
		if convErr != nil {
			return nil, convErr
		}

		switch row.RefKind {
		case refKindOrigin:
			header, exists := parentToDelta[refID.String()]
			if !exists {
				if originHeader, ok := originByID[refID.String()]; ok {
					header = originHeader
					header.ID = refID
					parent := refID
					header.DeltaParentID = &parent
					header.ExampleID = deltaExampleID
				} else {
					continue
				}
			}

			stateRow, hasState, stateErr := m.header.GetState(ctx, deltaExampleID, refID)
			if stateErr != nil {
				return nil, stateErr
			}
			if hasState {
				if stateRow.Suppressed {
					processed[header.ID.String()] = struct{}{}
					continue
				}
				if stateRow.Key.Valid {
					header.HeaderKey = stateRow.Key.String
				}
				if stateRow.Val.Valid {
					header.Value = stateRow.Val.String
				}
				if stateRow.Desc.Valid {
					header.Description = stateRow.Desc.String
				}
				if stateRow.Enabled.Valid {
					header.Enable = stateRow.Enabled.Bool
				}
			}

			finalHeaders = append(finalHeaders, header)
			processed[header.ID.String()] = struct{}{}

		case refKindDelta:
			existing, exists := deltaByID[refID.String()]
			key, value, desc, enabled, found, deltaErr := m.header.GetDelta(ctx, deltaExampleID, refID)
			if deltaErr != nil {
				return nil, deltaErr
			}
			if !found {
				continue
			}

			if exists {
				existing.HeaderKey = key
				existing.Value = value
				existing.Description = desc
				existing.Enable = enabled
				existing.DeltaParentID = nil
				finalHeaders = append(finalHeaders, existing)
				processed[existing.ID.String()] = struct{}{}
				continue
			}

			deltaHeader := mexampleheader.Header{
				ID:          refID,
				ExampleID:   deltaExampleID,
				HeaderKey:   key,
				Value:       value,
				Description: desc,
				Enable:      enabled,
			}
			finalHeaders = append(finalHeaders, deltaHeader)
			processed[deltaHeader.ID.String()] = struct{}{}
		}
	}

	for _, header := range deltaHeaders {
		if _, seen := processed[header.ID.String()]; seen {
			continue
		}
		if header.DeltaParentID != nil {
			stateRow, hasState, stateErr := m.header.GetState(ctx, deltaExampleID, *header.DeltaParentID)
			if stateErr != nil {
				return nil, stateErr
			}
			if hasState {
				if stateRow.Suppressed {
					continue
				}
				if stateRow.Key.Valid {
					header.HeaderKey = stateRow.Key.String
				}
				if stateRow.Val.Valid {
					header.Value = stateRow.Val.String
				}
				if stateRow.Desc.Valid {
					header.Description = stateRow.Desc.String
				}
				if stateRow.Enabled.Valid {
					header.Enable = stateRow.Enabled.Bool
				}
			}
		}
		finalHeaders = append(finalHeaders, header)
	}

	return finalHeaders, nil
}

func (m *Manager) MergeQueries(ctx context.Context, originQueries []mexamplequery.Query, deltaQueries []mexamplequery.Query, deltaExampleID idwrap.IDWrap) ([]mexamplequery.Query, error) {
	if m == nil || m.query == nil {
		return deltaQueries, nil
	}

	orderRows, err := m.query.SelectAsc(ctx, deltaExampleID)
	if err != nil {
		return nil, err
	}

	originByID := make(map[string]mexamplequery.Query, len(originQueries))
	for _, query := range originQueries {
		originByID[query.ID.String()] = query
	}

	deltaByID := make(map[string]mexamplequery.Query, len(deltaQueries))
	parentToDelta := make(map[string]mexamplequery.Query, len(deltaQueries))
	for _, query := range deltaQueries {
		deltaByID[query.ID.String()] = query
		if query.DeltaParentID != nil {
			parentToDelta[query.DeltaParentID.String()] = query
		}
	}

	finalQueries := make([]mexamplequery.Query, 0, len(deltaQueries))
	processed := make(map[string]struct{}, len(deltaQueries))

	for _, row := range orderRows {
		refID, convErr := idwrap.NewFromBytes(row.RefID)
		if convErr != nil {
			return nil, convErr
		}

		switch row.RefKind {
		case refKindOrigin:
			query, exists := parentToDelta[refID.String()]
			if !exists {
				if originQuery, ok := originByID[refID.String()]; ok {
					query = originQuery
					query.ID = refID
					parent := refID
					query.DeltaParentID = &parent
					query.ExampleID = deltaExampleID
				} else {
					continue
				}
			}

			stateRow, hasState, stateErr := m.query.GetState(ctx, deltaExampleID, refID)
			if stateErr != nil {
				return nil, stateErr
			}
			if hasState {
				if stateRow.Suppressed {
					processed[query.ID.String()] = struct{}{}
					continue
				}
				if stateRow.Key.Valid {
					query.QueryKey = stateRow.Key.String
				}
				if stateRow.Val.Valid {
					query.Value = stateRow.Val.String
				}
				if stateRow.Desc.Valid {
					query.Description = stateRow.Desc.String
				}
				if stateRow.Enabled.Valid {
					query.Enable = stateRow.Enabled.Bool
				}
			}

			finalQueries = append(finalQueries, query)
			processed[query.ID.String()] = struct{}{}

		case refKindDelta:
			existing, exists := deltaByID[refID.String()]
			key, value, desc, enabled, found, deltaErr := m.query.GetDelta(ctx, deltaExampleID, refID)
			if deltaErr != nil {
				return nil, deltaErr
			}
			if !found {
				continue
			}

			if exists {
				existing.QueryKey = key
				existing.Value = value
				existing.Description = desc
				existing.Enable = enabled
				existing.DeltaParentID = nil
				finalQueries = append(finalQueries, existing)
				processed[existing.ID.String()] = struct{}{}
				continue
			}

			deltaQuery := mexamplequery.Query{
				ID:          refID,
				ExampleID:   deltaExampleID,
				QueryKey:    key,
				Value:       value,
				Description: desc,
				Enable:      enabled,
			}
			finalQueries = append(finalQueries, deltaQuery)
			processed[deltaQuery.ID.String()] = struct{}{}
		}
	}

	for _, query := range deltaQueries {
		if _, seen := processed[query.ID.String()]; seen {
			continue
		}
		if query.DeltaParentID != nil {
			stateRow, hasState, stateErr := m.query.GetState(ctx, deltaExampleID, *query.DeltaParentID)
			if stateErr != nil {
				return nil, stateErr
			}
			if hasState {
				if stateRow.Suppressed {
					continue
				}
				if stateRow.Key.Valid {
					query.QueryKey = stateRow.Key.String
				}
				if stateRow.Val.Valid {
					query.Value = stateRow.Val.String
				}
				if stateRow.Desc.Valid {
					query.Description = stateRow.Desc.String
				}
				if stateRow.Enabled.Valid {
					query.Enable = stateRow.Enabled.Bool
				}
			}
		}
		finalQueries = append(finalQueries, query)
	}

	return finalQueries, nil
}
