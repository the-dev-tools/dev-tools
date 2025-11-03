package shttp

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
)

var ErrNoHttpAssertFound = errors.New("no HTTP assert found")

type HttpAssertService struct {
	queries *gen.Queries
}

func NewHttpAssertService(queries *gen.Queries) HttpAssertService {
	return HttpAssertService{queries: queries}
}

func (has HttpAssertService) TX(tx *sql.Tx) HttpAssertService {
	return HttpAssertService{queries: has.queries.WithTx(tx)}
}

func NewHttpAssertServiceTX(ctx context.Context, tx *sql.Tx) (*HttpAssertService, error) {
	queries := gen.New(tx)
	assertService := HttpAssertService{
		queries: queries,
	}

	return &assertService, nil
}

// ConvertToModelHttpAssert converts DB HttpAssert to model HttpAssert
func ConvertToModelHttpAssert(assert gen.HttpAssert) mhttp.HTTPAssert {
	return mhttp.HTTPAssert{
		ID:               assert.ID,
		HttpID:           assert.HttpID,
		AssertKey:        assert.Key,
		AssertValue:      assert.Value,
		Description:      assert.Description,
		Enabled:          assert.Enabled,
		ParentAssertID:   bytesToIDWrap(assert.ParentHttpAssertID),
		IsDelta:          assert.IsDelta,
		DeltaAssertKey:   nullToString(assert.DeltaKey),
		DeltaAssertValue: nullToString(assert.DeltaValue),
		DeltaDescription: nullToString(assert.DeltaDescription),
		DeltaEnabled:     assert.DeltaEnabled,
		Prev:             nil, // Not supported in current schema
		Next:             nil, // Not supported in current schema
		CreatedAt:        assert.CreatedAt,
		UpdatedAt:        assert.UpdatedAt,
	}
}

// ConvertToDBHttpAssert converts model HttpAssert to DB HttpAssert
func ConvertToDBHttpAssert(assert mhttp.HTTPAssert) gen.HttpAssert {
	return gen.HttpAssert{
		ID:                 assert.ID,
		HttpID:             assert.HttpID,
		Key:                assert.AssertKey,
		Value:              assert.AssertValue,
		Description:        assert.Description,
		Enabled:            assert.Enabled,
		Order:              0, // Default order, can be enhanced later
		ParentHttpAssertID: idWrapToBytes(assert.ParentAssertID),
		IsDelta:            assert.IsDelta,
		DeltaKey:           stringToNull(assert.DeltaAssertKey),
		DeltaValue:         stringToNull(assert.DeltaAssertValue),
		DeltaEnabled:       assert.DeltaEnabled,
		DeltaDescription:   stringToNull(assert.DeltaDescription),
		DeltaOrder:         sql.NullFloat64{}, // No order delta in model
		CreatedAt:          assert.CreatedAt,
		UpdatedAt:          assert.UpdatedAt,
	}
}

func (has HttpAssertService) Create(ctx context.Context, assert *mhttp.HTTPAssert) error {
	if assert == nil {
		return errors.New("assert cannot be nil")
	}

	now := time.Now().Unix()
	assert.CreatedAt = now
	assert.UpdatedAt = now

	dbAssert := ConvertToDBHttpAssert(*assert)
	return has.queries.CreateHTTPAssert(ctx, gen.CreateHTTPAssertParams{
		ID:                 dbAssert.ID,
		HttpID:             dbAssert.HttpID,
		Key:                dbAssert.Key,
		Value:              dbAssert.Value,
		Description:        dbAssert.Description,
		Enabled:            dbAssert.Enabled,
		Order:              dbAssert.Order,
		ParentHttpAssertID: dbAssert.ParentHttpAssertID,
		IsDelta:            dbAssert.IsDelta,
		DeltaKey:           dbAssert.DeltaKey,
		DeltaValue:         dbAssert.DeltaValue,
		DeltaEnabled:       dbAssert.DeltaEnabled,
		DeltaDescription:   dbAssert.DeltaDescription,
		DeltaOrder:         dbAssert.DeltaOrder,
		CreatedAt:          dbAssert.CreatedAt,
		UpdatedAt:          dbAssert.UpdatedAt,
	})
}

func (has HttpAssertService) CreateBulk(ctx context.Context, asserts []mhttp.HTTPAssert) error {
	if len(asserts) == 0 {
		return nil
	}

	// Create asserts individually since the bulk operation has complex parameter mapping
	for _, assert := range asserts {
		if err := has.Create(ctx, &assert); err != nil {
			return err
		}
	}

	return nil
}

func (has HttpAssertService) GetByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPAssert, error) {
	dbAsserts, err := has.queries.GetHTTPAssertsByHttpID(ctx, httpID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mhttp.HTTPAssert{}, nil
		}
		return nil, err
	}

	var asserts []mhttp.HTTPAssert
	for _, dbAssert := range dbAsserts {
		assert := ConvertToModelHttpAssert(dbAssert)
		asserts = append(asserts, assert)
	}

	return asserts, nil
}

func (has HttpAssertService) GetByIDs(ctx context.Context, ids []idwrap.IDWrap) ([]mhttp.HTTPAssert, error) {
	if len(ids) == 0 {
		return []mhttp.HTTPAssert{}, nil
	}

	dbAsserts, err := has.queries.GetHTTPAssertsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	var asserts []mhttp.HTTPAssert
	for _, dbAssert := range dbAsserts {
		assert := ConvertToModelHttpAssert(dbAssert)
		asserts = append(asserts, assert)
	}

	return asserts, nil
}

func (has HttpAssertService) GetByID(ctx context.Context, assertID idwrap.IDWrap) (mhttp.HTTPAssert, error) {
	dbAssert, err := has.queries.GetHTTPAssert(ctx, assertID)
	if err != nil {
		if err == sql.ErrNoRows {
			return mhttp.HTTPAssert{}, ErrNoHttpAssertFound
		}
		return mhttp.HTTPAssert{}, err
	}

	return ConvertToModelHttpAssert(dbAssert), nil
}

func (has HttpAssertService) Update(ctx context.Context, assert *mhttp.HTTPAssert) error {
	if assert == nil {
		return errors.New("assert cannot be nil")
	}

	assert.UpdatedAt = time.Now().Unix()

	dbAssert := ConvertToDBHttpAssert(*assert)
	return has.queries.UpdateHTTPAssert(ctx, gen.UpdateHTTPAssertParams{
		Key:              dbAssert.Key,
		Value:            dbAssert.Value,
		Description:      dbAssert.Description,
		Enabled:          dbAssert.Enabled,
		Order:            dbAssert.Order,
		DeltaKey:         dbAssert.DeltaKey,
		DeltaValue:       dbAssert.DeltaValue,
		DeltaEnabled:     dbAssert.DeltaEnabled,
		DeltaDescription: dbAssert.DeltaDescription,
		DeltaOrder:       dbAssert.DeltaOrder,
		UpdatedAt:        dbAssert.UpdatedAt,
		ID:               dbAssert.ID,
	})
}

func (has HttpAssertService) UpdateDelta(ctx context.Context, assertID idwrap.IDWrap, deltaKey, deltaValue, deltaDescription *string, deltaEnabled *bool) error {
	return has.queries.UpdateHTTPAssertDelta(ctx, gen.UpdateHTTPAssertDeltaParams{
		DeltaKey:         stringToNull(deltaKey),
		DeltaValue:       stringToNull(deltaValue),
		DeltaEnabled:     deltaEnabled,
		DeltaDescription: stringToNull(deltaDescription),
		ID:               assertID,
	})
}

func (has HttpAssertService) Delete(ctx context.Context, assertID idwrap.IDWrap) error {
	return has.queries.DeleteHTTPAssert(ctx, assertID)
}
