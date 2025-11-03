package shttp

import (
	"database/sql"

	"the-dev-tools/server/pkg/idwrap"
)

// Utility functions for type conversions used across all shttp services

func stringToNull(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *s, Valid: true}
}

func nullToString(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

func idWrapToBytes(id *idwrap.IDWrap) []byte {
	if id == nil {
		return nil
	}
	return id.Bytes()
}

func bytesToIDWrap(b []byte) *idwrap.IDWrap {
	if b == nil {
		return nil
	}
	id, err := idwrap.NewFromBytes(b)
	if err != nil {
		return nil
	}
	return &id
}
