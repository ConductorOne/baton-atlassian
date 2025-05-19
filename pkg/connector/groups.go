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

type groupBuilder struct {
	resourceType *v2.ResourceType
	client       *client.AtlassianClient
}

func (b *groupBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return groupResourceType
}

func (b *groupBuilder) List(ctx context.Context, _ *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	var groupResources []*v2.Resource

	bag, pageToken, err := getToken(pToken, userResourceType)
	if err != nil {
		return nil, "", nil, err
	}

	groups, nextPageToken, err := b.client.ListGroups(ctx, pageToken)
	if err != nil {
		return nil, "", nil, err
	}

	for _, group := range *groups {
		groupResource, err := parseIntoGroupResource(group)
		if err != nil {
			return nil, "", nil, err
		}
		groupResources = append(groupResources, groupResource)
	}

	err = bag.Next(nextPageToken)
	if err != nil {
		return nil, "", nil, err
	}
	nextPageToken, err = bag.Marshal()
	if err != nil {
		return nil, "", nil, err
	}

	return groupResources, nextPageToken, nil, nil
}

func (b *groupBuilder) Entitlements(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func (b *groupBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	var grantResources []*v2.Grant

	bag, pageToken, err := getToken(pToken, userResourceType)
	if err != nil {
		return nil, "", nil, err
	}

	groupID := resource.Id.Resource
	roleAssignments, nextPageToken, err := b.client.GetGroupRoleAssignments(ctx, pageToken, groupID)
	if err != nil {
		return nil, "", nil, err
	}

	for _, roleAssignment := range *roleAssignments {
		// We only want to sync the role assignments for the Atlassian Products.
		// The ignored scopes refers to:
		//     - project: This is a specific context within Jira. Roles with 'project' as the resource owner are typically project roles defined
		// within a particular Jira project (e.g., "Administrators", "Developers", "Users" within a single Jira project).
		//     - goal: This likely refers to roles related to Atlas Goals. If the organization uses Atlas for setting and tracking goals,
		// roles assigned within that product might have 'goal' as the resource owner.
		//     - platform: This generally refers to roles related to the core Atlassian platform itself. This could include roles related
		// to user accounts, organization settings not tied to a specific product, or platform-wide permissions.
		if roleAssignment.ResourceOwner == "project" || roleAssignment.ResourceOwner == "goal" || roleAssignment.ResourceOwner == "platform" {
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

func parseIntoGroupResource(group client.Group) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"name":        group.Name,
		"description": group.Description,
	}

	groupTraits := []resource.GroupTraitOption{
		resource.WithGroupProfile(profile),
	}
	return resource.NewGroupResource(
		group.Name,
		groupResourceType,
		group.ID,
		groupTraits,
	)
}

func newGroupBuilder(c *client.AtlassianClient) *groupBuilder {
	return &groupBuilder{
		resourceType: groupResourceType,
		client:       c,
	}
}
