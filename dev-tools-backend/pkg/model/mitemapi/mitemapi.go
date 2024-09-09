package mitemapi

import (
	"dev-tools-backend/pkg/model/mitemapiexample"

	"github.com/oklog/ulid/v2"
)

type ItemApi struct {
	ID           ulid.ULID
	CollectionID ulid.ULID
	ParentID     *ulid.ULID
	Name         string
	Url          string
	Method       string
	Prev         *ulid.ULID
	Next         *ulid.ULID
}

type ItemApiWithExamples struct {
	ItemApi

	DefaultExample mitemapiexample.ItemApiExample
	Examples       []mitemapiexample.ItemApiExampleMeta
}
