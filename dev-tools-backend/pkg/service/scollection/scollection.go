package scollection

import (
	"context"
	"database/sql"
	"dev-tools-backend/pkg/model/mcollection"
	"dev-tools-db/pkg/sqlc/gen"
	"fmt"

	"github.com/oklog/ulid/v2"
)

type CollectionService struct {
	DB      *sql.DB
	queries *gen.Queries
}

func New(ctx context.Context, db *sql.DB) (*CollectionService, error) {
	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		return nil, err
	}
	service := CollectionService{DB: db, queries: queries}
	return &service, nil
}

func (cs CollectionService) ListCollections(ctx context.Context, ownerID ulid.ULID) ([]mcollection.Collection, error) {
	fmt.Println(ownerID, ctx)
	rows, err := cs.queries.GetCollectionByOwnerID(ctx, ownerID.Bytes())
	if err != nil {
		return nil, err
	}
	var collections []mcollection.Collection
	for _, row := range rows {
		collections = append(collections, mcollection.Collection{
			ID:      ulid.ULID(row.ID),
			OwnerID: ulid.ULID(row.OwnerID),
			Name:    row.Name,
		})
	}
	return collections, nil
}

func (cs CollectionService) CreateCollection(ctx context.Context, collection *mcollection.Collection) error {
	_, err := cs.queries.CreateCollection(ctx, gen.CreateCollectionParams{
		ID:      collection.ID.Bytes(),
		OwnerID: collection.OwnerID.Bytes(),
		Name:    collection.Name,
	})
	return err
}

func (cs CollectionService) GetCollection(ctx context.Context, id ulid.ULID) (*mcollection.Collection, error) {
	collection, err := cs.queries.GetCollection(ctx, id.Bytes())
	if err != nil {
		return nil, err
	}
	c := mcollection.Collection{
		ID:      ulid.ULID(collection.ID),
		OwnerID: ulid.ULID(collection.OwnerID),
		Name:    collection.Name,
	}
	return &c, nil
}

func (cs CollectionService) UpdateCollection(ctx context.Context, collection *mcollection.Collection) error {
	err := cs.queries.UpdateCollection(ctx, gen.UpdateCollectionParams{
		ID:      collection.ID.Bytes(),
		OwnerID: collection.OwnerID.Bytes(),
		Name:    collection.Name,
	})
	return err
}

func (cs CollectionService) DeleteCollection(ctx context.Context, id ulid.ULID) error {
	return cs.queries.DeleteCollection(ctx, id.Bytes())
}

func (cs CollectionService) GetOwner(ctx context.Context, id ulid.ULID) (ulid.ULID, error) {
	ulidBytes, err := cs.queries.GetCollectionOwnerID(ctx, id.Bytes())
	if err != nil {
		return ulid.ULID{}, err
	}
	return ulid.ULID(ulidBytes), nil
}

func (cs CollectionService) CheckOwner(ctx context.Context, id ulid.ULID, ownerID ulid.ULID) (bool, error) {
	CollectionOwnerID, err := cs.GetOwner(ctx, id)
	if err != nil {
		return false, err
	}
	return ownerID.Compare(CollectionOwnerID) == 0, nil
}
