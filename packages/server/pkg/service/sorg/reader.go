package sorg

import (
	"context"
	"database/sql"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/morg"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/tgeneric"
)

type Reader struct {
	queries *gen.Queries
}

func NewReader(db *sql.DB) *Reader {
	return &Reader{queries: gen.New(db)}
}

func (r *Reader) GetOrganization(ctx context.Context, id idwrap.IDWrap) (*morg.Organization, error) {
	o, err := r.queries.AuthGetOrganization(ctx, id)
	err = tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrOrganizationNotFound, err)
	if err != nil {
		return nil, err
	}
	return convertToModelOrganization(o), nil
}

func (r *Reader) GetOrganizationBySlug(ctx context.Context, slug string) (*morg.Organization, error) {
	o, err := r.queries.AuthGetOrganizationBySlug(ctx, sql.NullString{String: slug, Valid: true})
	err = tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrOrganizationNotFound, err)
	if err != nil {
		return nil, err
	}
	return convertToModelOrganization(o), nil
}

func (r *Reader) ListOrganizationsForUser(ctx context.Context, userID idwrap.IDWrap) ([]*morg.Organization, error) {
	members, err := r.queries.AuthListMembersByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	orgs := make([]*morg.Organization, 0, len(members))
	for _, m := range members {
		o, err := r.queries.AuthGetOrganization(ctx, m.OrganizationID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, err
		}
		orgs = append(orgs, convertToModelOrganization(o))
	}
	return orgs, nil
}

func (r *Reader) ListMembers(ctx context.Context, orgID idwrap.IDWrap) ([]*morg.Member, error) {
	rows, err := r.queries.AuthListMembersByOrganization(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return tgeneric.MassConvert(rows, convertToModelMember), nil
}

func (r *Reader) GetMemberByUserAndOrg(ctx context.Context, userID, orgID idwrap.IDWrap) (*morg.Member, error) {
	m, err := r.queries.AuthGetMemberByUserAndOrg(ctx, gen.AuthGetMemberByUserAndOrgParams{
		UserID:         userID,
		OrganizationID: orgID,
	})
	err = tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrMemberNotFound, err)
	if err != nil {
		return nil, err
	}
	return convertToModelMember(m), nil
}

func (r *Reader) ListInvitations(ctx context.Context, orgID idwrap.IDWrap) ([]*morg.Invitation, error) {
	rows, err := r.queries.AuthListInvitationsByOrganization(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return tgeneric.MassConvert(rows, convertToModelInvitation), nil
}

func (r *Reader) ListInvitationsForEmail(ctx context.Context, email string) ([]*morg.Invitation, error) {
	rows, err := r.queries.AuthListInvitationsByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	return tgeneric.MassConvert(rows, convertToModelInvitation), nil
}
