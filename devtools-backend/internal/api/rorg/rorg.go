package rorg

import (
	"context"
	"devtools-backend/internal/api"
	"devtools-backend/internal/api/middleware/mwauth"
	"devtools-backend/pkg/service/sorg"
	orgv1 "devtools-services/gen/org/v1"
	"devtools-services/gen/org/v1/orgv1connect"
	"errors"

	"connectrpc.com/connect"
	"github.com/oklog/ulid/v2"
)

type OrgService struct{}

func (c *OrgService) GetOrg(ctx context.Context, req *connect.Request[orgv1.GetOrgRequest]) (*connect.Response[orgv1.Org], error) {
	orgID, err := ulid.Parse(req.Msg.OrgId)
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

	resp := &orgv1.Org{
		OrgId:       org.ID.String(),
		Name:        org.Name,
		DisplayName: org.Name,
	}

	return connect.NewResponse(resp), nil
}

func (c *OrgService) GetOrgs(ctx context.Context, req *connect.Request[orgv1.GetOrgsRequest]) (*connect.Response[orgv1.GetOrgsResponse], error) {
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
	respOrgs := make([]*orgv1.Org, len(orgs))
	for i, org := range orgs {
		respOrgs[i] = &orgv1.Org{
			OrgId:       org.ID.String(),
			Name:        org.Name,
			DisplayName: org.Name,
		}
	}

	resp := &orgv1.GetOrgsResponse{
		Orgs: respOrgs,
	}
	return connect.NewResponse(resp), nil
}

func CreateService(secret []byte) (*api.Service, error) {
	AuthInterceptorFunc := mwauth.NewAuthInterceptor(secret)
	server := &OrgService{}
	path, handler := orgv1connect.NewOrgServiceHandler(server, connect.WithInterceptors(AuthInterceptorFunc))
	return &api.Service{Path: path, Handler: handler}, nil
}
