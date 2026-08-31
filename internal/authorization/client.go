package authorization

import (
	"context"
	"errors"

	"github.com/lihongjie0209/billing-service/internal/outbound"
	platformauthz "github.com/lihongjie0209/microservice-platform-go/authz"
	platformprincipal "github.com/lihongjie0209/microservice-platform-go/principal"
	authorizationv1 "github.com/lihongjie0209/platform-protos/gen/go/platform/authorization/v1"
)

type Client struct {
	client authorizationv1.AuthorizationServiceClient
}

func New(registry *outbound.Registry) platformauthz.Authorizer {
	connection, ok := registry.GRPC("authorization")
	if !ok {
		return &Client{}
	}
	return &Client{client: authorizationv1.NewAuthorizationServiceClient(connection)}
}
func (c *Client) Authorize(ctx context.Context, identity platformprincipal.Principal, requirement platformauthz.Requirement) error {
	if c.client == nil {
		return errors.New("authorization upstream is not configured")
	}
	subjectID, subjectType := identity.ID, authorizationv1.SubjectType_SUBJECT_TYPE_MEMBERSHIP
	if identity.MembershipID != "" {
		subjectID, subjectType = identity.MembershipID, authorizationv1.SubjectType_SUBJECT_TYPE_MEMBERSHIP
	}
	if identity.Type == platformprincipal.TypeServiceAccount {
		subjectType = authorizationv1.SubjectType_SUBJECT_TYPE_SERVICE_ACCOUNT
	}
	response, err := c.client.Check(ctx, &authorizationv1.CheckRequest{TenantId: identity.TenantID, Subject: &authorizationv1.Subject{Id: subjectID, Type: subjectType}, ResourceType: requirement.Resource, Action: requirement.Action})
	if err != nil {
		return err
	}
	if !response.GetAllowed() {
		return platformauthz.ErrDenied
	}
	return nil
}
