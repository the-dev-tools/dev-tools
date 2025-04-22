package scollection

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcollection"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

var ErrNoCollectionFound = sql.ErrNoRows

type CollectionService struct {
	queries *gen.Queries
	logger  *slog.Logger
}

func ConvertToDBCollection(collection mcollection.Collection) gen.Collection {
	return gen.Collection{
		ID:          collection.ID,
		WorkspaceID: collection.WorkspaceID,
		Name:        collection.Name,
	}
}

func ConvertToModelCollection(collection gen.Collection) *mcollection.Collection {
	return &mcollection.Collection{
		ID:          collection.ID,
		WorkspaceID: collection.WorkspaceID,
		Name:        collection.Name,
	}
}

func New(queries *gen.Queries, logger *slog.Logger) CollectionService {
	return CollectionService{
		queries: queries,
		logger:  logger,
	}
}

func (cs CollectionService) TX(tx *sql.Tx) CollectionService {
	return CollectionService{queries: cs.queries.WithTx(tx)}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*CollectionService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	service := CollectionService{queries: queries}
	return &service, nil
}

func (cs CollectionService) ListCollections(ctx context.Context, workspaceID idwrap.IDWrap) ([]mcollection.Collection, error) {
	rows, err := cs.queries.GetCollectionByWorkspaceID(ctx, workspaceID)
	if err != nil {
		if err == sql.ErrNoRows {
			cs.logger.InfoContext(ctx, fmt.Sprintf("workspaceID: %s not found", workspaceID.String()))
			return nil, ErrNoCollectionFound
		}
		return nil, err
	}
	return tgeneric.MassConvertPtr(rows, ConvertToModelCollection), nil
}

func (cs CollectionService) CreateCollection(ctx context.Context, collection *mcollection.Collection) error {
	col := ConvertToDBCollection(*collection)
	return cs.queries.CreateCollection(ctx, gen.CreateCollectionParams{
		ID:          col.ID,
		WorkspaceID: col.WorkspaceID,
		Name:        col.Name,
	})
}

func (cs CollectionService) GetCollection(ctx context.Context, id idwrap.IDWrap) (*mcollection.Collection, error) {
	collection, err := cs.queries.GetCollection(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			cs.logger.DebugContext(ctx, fmt.Sprintf("CollectionID: %s not found", id.String()))
			return nil, ErrNoCollectionFound
		}
		return nil, err
	}
	return ConvertToModelCollection(collection), nil
}

func (cs CollectionService) UpdateCollection(ctx context.Context, collection *mcollection.Collection) error {
	err := cs.queries.UpdateCollection(ctx, gen.UpdateCollectionParams{
		ID:          collection.ID,
		WorkspaceID: collection.WorkspaceID,
		Name:        collection.Name,
	})
	return err
}

func (cs CollectionService) DeleteCollection(ctx context.Context, id idwrap.IDWrap) error {
	return cs.queries.DeleteCollection(ctx, id)
}

func (cs CollectionService) GetWorkspaceID(ctx context.Context, id idwrap.IDWrap) (idwrap.IDWrap, error) {
	ulidData, err := cs.queries.GetCollectionWorkspaceID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return idwrap.IDWrap{}, ErrNoCollectionFound
		}
		return idwrap.IDWrap{}, err
	}
	return ulidData, nil
}

func (cs CollectionService) CheckWorkspaceID(ctx context.Context, id, ownerID idwrap.IDWrap) (bool, error) {
	CollectionWorkspaceID, err := cs.GetWorkspaceID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, ErrNoCollectionFound
		}
		return false, err
	}
	return ownerID.Compare(CollectionWorkspaceID) == 0, nil
}
