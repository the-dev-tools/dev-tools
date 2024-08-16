package rorg

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/pkg/model/morg"
	"dev-tools-backend/pkg/model/morguser"
	"dev-tools-backend/pkg/service/sorg"
	"dev-tools-backend/pkg/service/sorguser"
	orgv1 "dev-tools-services/gen/organization/v1"
	"dev-tools-services/gen/organization/v1/organizationv1connect"
	"errors"

	"connectrpc.com/connect"
	"github.com/oklog/ulid/v2"
)

type OrgService struct{}

func (c *OrgService) GetOrganization(ctx context.Context, req *connect.Request[orgv1.GetOrganizationRequest]) (*connect.Response[orgv1.GetOrganizationResponse], error) {
	orgID, err := ulid.Parse(req.Msg.OrganizationId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}

	org, err := sorg.GetOrgByUserIDAndOrgID(userID, orgID)
	if err != nil {
		if errors.Is(err, sorg.ErrOrgNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &orgv1.GetOrganizationResponse{
		Organization: &orgv1.Organization{
			OrganizationId: org.ID.String(),
			Name:           org.Name,
		},
	}

	return connect.NewResponse(resp), nil
}

func (c *OrgService) GetOrganizations(ctx context.Context, req *connect.Request[orgv1.GetOrganizationsRequest]) (*connect.Response[orgv1.GetOrganizationsResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}

	orgs, err := sorg.GetOrgsByUserID(userID)
	if err != nil {
		if errors.Is(err, sorg.ErrOrgNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	respOrgs := make([]*orgv1.Organization, len(orgs))
	for i, org := range orgs {
		respOrgs[i] = &orgv1.Organization{
			OrganizationId: org.ID.String(),
			Name:           org.Name,
		}
	}

	resp := &orgv1.GetOrganizationsResponse{
		Organizations: respOrgs,
	}
	return connect.NewResponse(resp), nil
}

func (c *OrgService) CreateOrganization(ctx context.Context, req *connect.Request[orgv1.CreateOrganizationRequest]) (*connect.Response[orgv1.CreateOrganizationResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}

	ulidID := ulid.Make()

	org := &morg.Org{
		ID:   ulidID,
		Name: req.Msg.Name,
	}

	// TODO: add transaction
	err = sorg.CreateOrg(org)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	orgUser := &morguser.OrgUser{
		ID:     ulid.Make(),
		OrgID:  org.ID,
		UserID: userID,
	}

	err = sorguser.CreateOrgUser(orgUser)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &orgv1.CreateOrganizationResponse{
		Organization: &orgv1.Organization{
			OrganizationId: org.ID.String(),
			Name:           org.Name,
		},
	}
	return connect.NewResponse(resp), nil
}

func (c *OrgService) UpdateOrganization(ctx context.Context, req *connect.Request[orgv1.UpdateOrganizationRequest]) (*connect.Response[orgv1.UpdateOrganizationResponse], error) {
	userUlid, err := mwauth.GetContextUserOrgID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}

	orgUlid, err := ulid.Parse(req.Msg.OrganizationId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}

	org, err := sorg.GetOrgByUserIDAndOrgID(orgUlid, userUlid)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("organization not found"))
		}
	}

	org.Name = req.Msg.Name
	err = sorg.UpdateOrg(org)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&orgv1.UpdateOrganizationResponse{}), nil
}

func (c *OrgService) DeleteOrganization(ctx context.Context, req *connect.Request[orgv1.DeleteOrganizationRequest]) (*connect.Response[orgv1.DeleteOrganizationResponse], error) {
	userUlid, err := mwauth.GetContextUserOrgID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}

	orgUlid, err := ulid.Parse(req.Msg.OrganizationId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	org, err := sorg.GetOrgByUserIDAndOrgID(orgUlid, userUlid)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("organization not found"))
		}
	}

	err = sorg.DeleteOrg(org.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&orgv1.DeleteOrganizationResponse{}), nil
}

func CreateService(secret []byte) (*api.Service, error) {
	AuthInterceptorFunc := mwauth.NewAuthInterceptor(secret)
	server := &OrgService{}
	path, handler := organizationv1connect.NewOrganizationServiceHandler(server, connect.WithInterceptors(AuthInterceptorFunc))
	return &api.Service{Path: path, Handler: handler}, nil
}
