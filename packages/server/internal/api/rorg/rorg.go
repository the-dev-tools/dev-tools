//nolint:revive // exported
package rorg

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/api"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/morg"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sorg"
	v1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/organization/v1"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/organization/v1/organizationv1connect"
)

type OrgServiceRPC struct {
	organizationv1connect.UnimplementedOrganizationServiceHandler
	reader *sorg.Reader
}

type OrgServiceRPCDeps struct {
	Reader *sorg.Reader
}

func (d *OrgServiceRPCDeps) Validate() error {
	if d.Reader == nil {
		return fmt.Errorf("reader is required")
	}
	return nil
}

func New(deps OrgServiceRPCDeps) OrgServiceRPC {
	if err := deps.Validate(); err != nil {
		panic(fmt.Sprintf("OrgServiceRPC Deps validation failed: %v", err))
	}
	return OrgServiceRPC{
		reader: deps.Reader,
	}
}

func CreateService(srv OrgServiceRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := organizationv1connect.NewOrganizationServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func roleToProto(r morg.Role) v1.OrganizationRole {
	switch r {
	case morg.RoleOwner:
		return v1.OrganizationRole_ORGANIZATION_ROLE_OWNER
	case morg.RoleAdmin:
		return v1.OrganizationRole_ORGANIZATION_ROLE_ADMIN
	case morg.RoleMember:
		return v1.OrganizationRole_ORGANIZATION_ROLE_MEMBER
	default:
		return v1.OrganizationRole_ORGANIZATION_ROLE_UNSPECIFIED
	}
}

func invitationStatusToProto(s morg.InvitationStatus) v1.InvitationStatus {
	switch s {
	case morg.StatusPending:
		return v1.InvitationStatus_INVITATION_STATUS_PENDING
	case morg.StatusAccepted:
		return v1.InvitationStatus_INVITATION_STATUS_ACCEPTED
	case morg.StatusRejected:
		return v1.InvitationStatus_INVITATION_STATUS_REJECTED
	case morg.StatusCanceled:
		return v1.InvitationStatus_INVITATION_STATUS_CANCELED
	default:
		return v1.InvitationStatus_INVITATION_STATUS_UNSPECIFIED
	}
}

func toAPIOrganization(o *morg.Organization) *v1.Organization {
	org := &v1.Organization{
		OrganizationId: o.ID.Bytes(),
		Name:           o.Name,
		Slug:           o.Slug,
		Logo:           o.Logo,
		Metadata:       o.Metadata,
		CreatedAt:      &o.CreatedAt,
	}
	return org
}

func toAPIMember(m *morg.Member) *v1.OrganizationMember {
	return &v1.OrganizationMember{
		MemberId:       m.ID.Bytes(),
		OrganizationId: m.OrganizationID.Bytes(),
		UserId:         m.UserID.Bytes(),
		Role:           roleToProto(m.Role),
		CreatedAt:      &m.CreatedAt,
	}
}

func toAPIInvitation(inv *morg.Invitation) *v1.OrganizationInvitation {
	return &v1.OrganizationInvitation{
		InvitationId:   inv.ID.Bytes(),
		OrganizationId: inv.OrganizationID.Bytes(),
		InviterId:      inv.InviterID.Bytes(),
		Email:          inv.Email,
		Role:           roleToProto(inv.Role),
		Status:         invitationStatusToProto(inv.Status),
		CreatedAt:      &inv.CreatedAt,
		ExpiresAt:      inv.ExpiresAt,
	}
}

func (c *OrgServiceRPC) OrganizationCollection(ctx context.Context, _ *connect.Request[emptypb.Empty]) (*connect.Response[v1.OrganizationCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Look up the user's external ID (BetterAuth auth_user ULID) to query memberships
	orgs, err := c.reader.ListOrganizationsForUser(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	items := make([]*v1.Organization, 0, len(orgs))
	for _, o := range orgs {
		items = append(items, toAPIOrganization(o))
	}

	return connect.NewResponse(&v1.OrganizationCollectionResponse{
		Items: items,
	}), nil
}

func (c *OrgServiceRPC) OrganizationMemberCollection(ctx context.Context, _ *connect.Request[emptypb.Empty]) (*connect.Response[v1.OrganizationMemberCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// List all orgs the user belongs to, then collect members across all of them
	orgs, err := c.reader.ListOrganizationsForUser(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var items []*v1.OrganizationMember
	for _, o := range orgs {
		members, err := c.reader.ListMembers(ctx, o.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, m := range members {
			items = append(items, toAPIMember(m))
		}
	}

	return connect.NewResponse(&v1.OrganizationMemberCollectionResponse{
		Items: items,
	}), nil
}

func (c *OrgServiceRPC) OrganizationInvitationCollection(ctx context.Context, _ *connect.Request[emptypb.Empty]) (*connect.Response[v1.OrganizationInvitationCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// List orgs the user belongs to, then collect invitations for orgs where user is admin/owner
	orgs, err := c.reader.ListOrganizationsForUser(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var items []*v1.OrganizationInvitation
	for _, o := range orgs {
		// CHECK: caller must be admin or owner to see invitations
		member, err := c.reader.GetMemberByUserAndOrg(ctx, userID, o.ID)
		if err != nil {
			if errors.Is(err, sorg.ErrMemberNotFound) {
				continue
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if member.Role != morg.RoleOwner && member.Role != morg.RoleAdmin {
			continue
		}

		invitations, err := c.reader.ListInvitations(ctx, o.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, inv := range invitations {
			items = append(items, toAPIInvitation(inv))
		}
	}

	return connect.NewResponse(&v1.OrganizationInvitationCollectionResponse{
		Items: items,
	}), nil
}

// OrganizationInsert returns unimplemented — org creation is handled by BetterAuth.
func (c *OrgServiceRPC) OrganizationInsert(_ context.Context, _ *connect.Request[v1.OrganizationInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("organization creation is managed by BetterAuth"))
}

// OrganizationUpdate returns unimplemented — org updates are handled by BetterAuth.
func (c *OrgServiceRPC) OrganizationUpdate(_ context.Context, _ *connect.Request[v1.OrganizationUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("organization updates are managed by BetterAuth"))
}

