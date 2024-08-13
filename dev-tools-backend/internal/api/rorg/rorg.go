package rorg

import (
	"context"
	"devtools-backend/internal/api"
	"devtools-backend/internal/api/middleware/mwauth"
	"devtools-backend/pkg/service/sorg"
	orgv1 "devtools-services/gen/organization/v1"
	"devtools-services/gen/organization/v1/organizationv1connect"
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

	org, err := sorg.GetOrgByUserIDAndOrgID(&orgID, userID)
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

	orgs, err := sorg.GetOrgsByUserID(*userID)
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

func CreateService(secret []byte) (*api.Service, error) {
	AuthInterceptorFunc := mwauth.NewAuthInterceptor(secret)
	server := &OrgService{}
	path, handler := organizationv1connect.NewOrganizationServiceHandler(server, connect.WithInterceptors(AuthInterceptorFunc))
	return &api.Service{Path: path, Handler: handler}, nil
}
