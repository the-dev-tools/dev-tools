package sorg

import (
	"database/sql"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/morg"
)

func nullStringToPtr(ns sql.NullString) *string {
	if ns.Valid {
		return &ns.String
	}
	return nil
}

func convertToModelOrganization(o gen.AuthOrganization) *morg.Organization {
	return &morg.Organization{
		ID:        o.ID,
		Name:      o.Name,
		Slug:      nullStringToPtr(o.Slug),
		Logo:      nullStringToPtr(o.Logo),
		Metadata:  nullStringToPtr(o.Metadata),
		CreatedAt: o.CreatedAt,
	}
}

func convertToModelMember(m gen.AuthMember) *morg.Member {
	return &morg.Member{
		ID:             m.ID,
		UserID:         m.UserID,
		OrganizationID: m.OrganizationID,
		Role:           morg.Role(m.Role),
		CreatedAt:      m.CreatedAt,
	}
}

func convertToModelInvitation(inv gen.AuthInvitation) *morg.Invitation {
	return &morg.Invitation{
		ID:             inv.ID,
		Email:          inv.Email,
		InviterID:      inv.InviterID,
		OrganizationID: inv.OrganizationID,
		Role:           morg.Role(inv.Role),
		Status:         morg.InvitationStatus(inv.Status),
		CreatedAt:      inv.CreatedAt,
		ExpiresAt:      inv.ExpiresAt,
	}
}
