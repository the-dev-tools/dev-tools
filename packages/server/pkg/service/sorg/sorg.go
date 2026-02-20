//nolint:revive // exported
package sorg

import (
	"database/sql"
	"errors"
)

var (
	ErrOrganizationNotFound = errors.New("sorg: organization not found")
	ErrMemberNotFound       = errors.New("sorg: member not found")
)

// OrgService is a read-only facade for organization data.
// BetterAuth handles all mutations via the adapter; this service
// exposes org data through the app's own API layer.
type OrgService struct {
	reader *Reader
}

func New(db *sql.DB) OrgService {
	return OrgService{reader: NewReader(db)}
}

func (s OrgService) Reader() *Reader { return s.reader }
