package assertiondelta

import (
	"context"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/service/sassert"
)

// NewStore adapts sassert.AssertService to the resolver Store contract.
func NewStore(service sassert.AssertService) Store {
	return &serviceStore{service: service}
}

type serviceStore struct {
	service sassert.AssertService
}

func (s *serviceStore) GetAssert(ctx context.Context, id idwrap.IDWrap) (massert.Assert, error) {
	assert, err := s.service.GetAssert(ctx, id)
	if err != nil {
		return massert.Assert{}, err
	}
	return *assert, nil
}

func (s *serviceStore) ListByExample(ctx context.Context, example idwrap.IDWrap) ([]massert.Assert, error) {
	return s.service.GetAssertByExampleID(ctx, example)
}

func (s *serviceStore) ListByDeltaParent(ctx context.Context, parent idwrap.IDWrap) ([]massert.Assert, error) {
	return s.service.GetAssertsByDeltaParent(ctx, parent)
}

func (s *serviceStore) UpdateAssert(ctx context.Context, assert massert.Assert) error {
	return s.service.UpdateAssert(ctx, assert)
}

func (s *serviceStore) CreateAssert(ctx context.Context, assert massert.Assert) error {
	return s.service.CreateAssert(ctx, assert)
}

func (s *serviceStore) DeleteAssert(ctx context.Context, id idwrap.IDWrap) error {
	return s.service.DeleteAssert(ctx, id)
}
