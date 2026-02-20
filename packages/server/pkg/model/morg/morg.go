//nolint:revive // exported
package morg

import "github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"

type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

type InvitationStatus string

const (
	StatusPending  InvitationStatus = "pending"
	StatusAccepted InvitationStatus = "accepted"
	StatusRejected InvitationStatus = "rejected"
	StatusCanceled InvitationStatus = "canceled"
)

type Organization struct {
	ID        idwrap.IDWrap
	Name      string
	Slug      *string
	Logo      *string
	Metadata  *string
	CreatedAt int64
}

type Member struct {
	ID             idwrap.IDWrap
	UserID         idwrap.IDWrap
	OrganizationID idwrap.IDWrap
	Role           Role
	CreatedAt      int64
}

type Invitation struct {
	ID             idwrap.IDWrap
	Email          string
	InviterID      idwrap.IDWrap
	OrganizationID idwrap.IDWrap
	Role           Role
	Status         InvitationStatus
	CreatedAt      int64
	ExpiresAt      int64
}
