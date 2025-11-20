package connector

import (
	"context"
	"fmt"

	"github.com/conductorone/baton-atlassian/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	"github.com/conductorone/baton-sdk/pkg/types/resource"
)

type userBuilder struct {
	resourceType *v2.ResourceType
	client       *client.AtlassianClient
}

func (b *userBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return userResourceType
}

func (b *userBuilder) List(ctx context.Context, _ *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	var resources []*v2.Resource

	bag, pageToken, err := getToken(pToken, userResourceType)
	if err != nil {
		return nil, "", nil, err
	}

	users, nextPageToken, err := b.client.ListUsers(ctx, pageToken)
	if err != nil {
		return nil, "", nil, err
	}

	for _, user := range users {
		userResource, err := parseIntoUserResource(user)
		if err != nil {
			return nil, "", nil, err
		}
		resources = append(resources, userResource)
	}

	err = bag.Next(nextPageToken)
	if err != nil {
		return nil, "", nil, err
	}
	nextPageToken, err = bag.Marshal()
	if err != nil {
		return nil, "", nil, err
	}

	return resources, nextPageToken, nil, nil
}

func (b *userBuilder) Entitlements(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

// Grants function will be creating the grants for the Workspaces.
//
// Users have Roles assigned that gives them permissions on each Workspace (product sites).
func (b *userBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	var grantResources []*v2.Grant

	bag, pageToken, err := getToken(pToken, userResourceType)
	if err != nil {
		return nil, "", nil, err
	}

	userID := resource.Id.Resource
	roleAssignments, nextPageToken, err := b.client.GetUserRoleAssignments(ctx, pageToken, userID)
	if err != nil {
		return nil, "", nil, err
	}

	for _, roleAssignment := range roleAssignments {
		// We only want to sync the role assignments for the Atlassian Products.
		// The ignored scopes refers to:
		//     - project: This is a specific context within Jira. Roles with 'project' as the resource owner are typically project roles defined
		// within a particular Jira project (e.g., "Administrators", "Developers", "Users" within a single Jira project).
		//     - goal: This likely refers to roles related to Atlas Goals. If the organization uses Atlas for setting and tracking goals,
		// roles assigned within that product might have 'goal' as the resource owner.
		//     - platform: This generally refers to roles related to the core Atlassian platform itself. This could include roles related
		// to user accounts, organization settings not tied to a specific product, or platform-wide permissions.
		if roleAssignment.ResourceOwner == resourceOwnerProject || roleAssignment.ResourceOwner == resourceOwnerGoal || roleAssignment.ResourceOwner == resourceOwnerPlatform {
			continue
		}

		workspaceResource := &v2.Resource{
			Id: &v2.ResourceId{
				ResourceType: workspaceResourceType.Id,
				Resource:     roleAssignment.ResourceId,
			},
		}

		for _, role := range roleAssignment.Roles {
			entitlementID := fmt.Sprintf("role:%s", role)
			grantResources = append(
				grantResources,
				grant.NewGrant(workspaceResource, entitlementID, resource),
			)
		}
	}

	err = bag.Next(nextPageToken)
	if err != nil {
		return nil, "", nil, err
	}
	nextPageToken, err = bag.Marshal()
	if err != nil {
		return nil, "", nil, err
	}

	return grantResources, nextPageToken, nil, nil
}

func parseIntoUserResource(user client.User) (*v2.Resource, error) {
	var userStatus = v2.UserTrait_Status_STATUS_UNSPECIFIED

	profile := map[string]interface{}{
		"account_id":     user.AccountId,
		"account_type":   user.AccountType,
		"username":       user.Name,
		"email_verified": user.EmailVerified,
	}

	switch user.Status {
	case "active":
		userStatus = v2.UserTrait_Status_STATUS_ENABLED
	case "deactivated":
		userStatus = v2.UserTrait_Status_STATUS_DISABLED
	}

	userTraits := []resource.UserTraitOption{
		resource.WithUserProfile(profile),
		resource.WithStatus(userStatus),
		resource.WithUserLogin(user.Email),
		resource.WithEmail(user.Email, true),
	}

	return resource.NewUserResource(
		user.Email,
		userResourceType,
		user.AccountId,
		userTraits,
	)
}

func newUserBuilder(c *client.AtlassianClient) *userBuilder {
	return &userBuilder{
		resourceType: userResourceType,
		client:       c,
	}
}
