package scollection

import (
	"context"
	"database/sql"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mcollection"
	"the-dev-tools/backend/pkg/translate/tgeneric"
	"the-dev-tools/db/pkg/sqlc/gen"
)

var ErrNoCollectionFound = sql.ErrNoRows

type CollectionService struct {
	queries *gen.Queries
}

func ConvertToDBCollection(collection mcollection.Collection) gen.Collection {
	return gen.Collection{
		ID:      collection.ID,
		OwnerID: collection.OwnerID,
		Name:    collection.Name,
	}
}

func ConvertToModelCollection(collection gen.Collection) *mcollection.Collection {
	return &mcollection.Collection{
		ID:      collection.ID,
		OwnerID: collection.OwnerID,
		Name:    collection.Name,
	}
}

func New(queries *gen.Queries) CollectionService {
	return CollectionService{queries: queries}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*CollectionService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	service := CollectionService{queries: queries}
	return &service, nil
}

func (cs CollectionService) ListCollections(ctx context.Context, ownerID idwrap.IDWrap) ([]mcollection.Collection, error) {
	rows, err := cs.queries.GetCollectionByOwnerID(ctx, ownerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoCollectionFound
		}
		return nil, err
	}
	return tgeneric.MassConvertPtr(rows, ConvertToModelCollection), nil
}

func (cs CollectionService) CreateCollection(ctx context.Context, collection *mcollection.Collection) error {
	col := ConvertToDBCollection(*collection)
	return cs.queries.CreateCollection(ctx, gen.CreateCollectionParams{
		ID:      col.ID,
		OwnerID: col.OwnerID,
		Name:    col.Name,
	})
}

func (cs CollectionService) GetCollection(ctx context.Context, id idwrap.IDWrap) (*mcollection.Collection, error) {
	collection, err := cs.queries.GetCollection(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoCollectionFound
		}
		return nil, err
	}
	return ConvertToModelCollection(collection), nil
}

func (cs CollectionService) UpdateCollection(ctx context.Context, collection *mcollection.Collection) error {
	err := cs.queries.UpdateCollection(ctx, gen.UpdateCollectionParams{
		ID:      collection.ID,
		OwnerID: collection.OwnerID,
		Name:    collection.Name,
	})
	return err
}

func (cs CollectionService) DeleteCollection(ctx context.Context, id idwrap.IDWrap) error {
	return cs.queries.DeleteCollection(ctx, id)
}

func (cs CollectionService) GetOwner(ctx context.Context, id idwrap.IDWrap) (idwrap.IDWrap, error) {
	ulidData, err := cs.queries.GetCollectionOwnerID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return idwrap.IDWrap{}, ErrNoCollectionFound
		}
		return idwrap.IDWrap{}, err
	}
	return ulidData, nil
}

func (cs CollectionService) CheckOwner(ctx context.Context, id, ownerID idwrap.IDWrap) (bool, error) {
	CollectionOwnerID, err := cs.GetOwner(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, ErrNoCollectionFound
		}
		return false, err
	}
	return ownerID.Compare(CollectionOwnerID) == 0, nil
}
