package connector

import (
	"context"
	"fmt"

	"github.com/conductorone/baton-atlassian/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	"github.com/conductorone/baton-sdk/pkg/types/resource"
)

// Pagination phases for group grants.
const (
	groupGrantsPhaseMembers         = "members"
	groupGrantsPhaseRoleAssignments = "role-assignments"
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

	bag, pageToken, err := getToken(pToken, groupResourceType)
	if err != nil {
		return nil, "", nil, err
	}

	groups, nextPageToken, err := b.client.ListGroups(ctx, pageToken)
	if err != nil {
		return nil, "", nil, err
	}

	for _, group := range groups {
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

func (b *groupBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	var groupEntitlements []*v2.Entitlement

	memberEntitlement := entitlement.NewAssignmentEntitlement(
		resource,
		"member",
		entitlement.WithGrantableTo(userResourceType),
		entitlement.WithDisplayName(fmt.Sprintf("%s member", resource.DisplayName)),
		entitlement.WithDescription(fmt.Sprintf("Member of the %s group", resource.DisplayName)),
	)
	groupEntitlements = append(groupEntitlements, memberEntitlement)

	return groupEntitlements, "", nil, nil
}

// Grants function will be creating the grants for the Workspaces, Organization, and group memberships.
//
// Groups have Roles assigned that gives them permissions on each Workspace (product sites)
// and on the Organization level (org-admin, site-admin).
// Additionally, we sync group memberships (users that belong to each group).
//
// Pagination is handled in two phases:
// 1. Members phase: paginate through all group members.
// 2. Role assignments phase: paginate through all role assignments.
func (b *groupBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	var grantResources []*v2.Grant

	bag := &pagination.Bag{}
	err := bag.Unmarshal(pToken.Token)
	if err != nil {
		return nil, "", nil, err
	}

	groupID := resource.Id.Resource

	if bag.Current() == nil {
		bag.Push(pagination.PageState{
			ResourceTypeID: groupGrantsPhaseMembers,
		})
	}

	currentPhase := bag.Current().ResourceTypeID
	pageToken := bag.PageToken()

	// Phase 1: Get group members and create membership grants
	if currentPhase == groupGrantsPhaseMembers {
		members, membersNextPageToken, err := b.client.GetGroupMembers(ctx, pageToken, groupID)
		if err != nil {
			return nil, "", nil, err
		}

		for _, member := range members {
			userResource := &v2.Resource{
				Id: &v2.ResourceId{
					ResourceType: userResourceType.Id,
					Resource:     member.AccountId,
				},
			}
			grantResources = append(
				grantResources,
				grant.NewGrant(resource, "member", userResource),
			)
		}

		// If there are more members to paginate, continue with members phase
		if membersNextPageToken != "" {
			err = bag.Next(membersNextPageToken)
			if err != nil {
				return nil, "", nil, err
			}
			nextPageToken, err := bag.Marshal()
			if err != nil {
				return nil, "", nil, err
			}
			return grantResources, nextPageToken, nil, nil
		}

		// Members phase complete, transition to role assignments phase
		bag.Pop()
		bag.Push(pagination.PageState{
			ResourceTypeID: groupGrantsPhaseRoleAssignments,
		})
		pageToken = "" // Reset page token for new phase.
	}

	roleAssignments, roleAssignmentsNextPageToken, err := b.client.GetGroupRoleAssignments(ctx, pageToken, groupID)
	if err != nil {
		return nil, "", nil, err
	}

	for _, roleAssignment := range roleAssignments {
		// We only want to sync the role assignments for the Atlassian Products (Workspaces).
		// The ignored scopes refers to:
		//     - platform: Organization-level roles (org-admin, site-admin) are already synced directly from user role-assignments.
		//     - project: This is a specific context within Jira. Roles with 'project' as the resource owner are typically project roles defined
		// within a particular Jira project (e.g., "Administrators", "Developers", "Users" within a single Jira project).
		//     - goal: This likely refers to roles related to Atlas Goals. If the organization uses Atlas for setting and tracking goals,
		// roles assigned within that product might have 'goal' as the resource owner.
		if roleAssignment.ResourceOwner == resourceOwnerPlatform ||
			roleAssignment.ResourceOwner == resourceOwnerProject ||
			roleAssignment.ResourceOwner == resourceOwnerGoal {
			continue
		}

		// Handle workspace roles
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

	// If there are more role assignments to paginate, continue
	if roleAssignmentsNextPageToken != "" {
		err = bag.Next(roleAssignmentsNextPageToken)
		if err != nil {
			return nil, "", nil, err
		}
		nextPageToken, err := bag.Marshal()
		if err != nil {
			return nil, "", nil, err
		}
		return grantResources, nextPageToken, nil, nil
	}

	return grantResources, "", nil, nil
}

func (b *groupBuilder) Grant(ctx context.Context, principal *v2.Resource, entitlement *v2.Entitlement) (annotations.Annotations, error) {
	if principal.Id.ResourceType != userResourceType.Id {
		return nil, fmt.Errorf("baton-atlassian: only users can be granted group membership")
	}

	groupID := entitlement.Resource.Id.Resource
	accountID := principal.Id.Resource

	directoryID, err := b.client.GetGroupDirectoryID(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("baton-atlassian: failed to resolve group directory: %w", err)
	}

	err = b.client.AddUserToGroup(ctx, directoryID, groupID, accountID)
	if err != nil {
		if client.IsAlreadyMember(err) {
			return annotations.New(&v2.GrantAlreadyExists{}), nil
		}
		return nil, fmt.Errorf("baton-atlassian: failed to add user to group: %w", err)
	}

	return nil, nil
}

func (b *groupBuilder) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	if grant.Principal.Id.ResourceType != userResourceType.Id {
		return nil, fmt.Errorf("baton-atlassian: only users can be revoked from group membership")
	}

	groupID := grant.Entitlement.Resource.Id.Resource
	accountID := grant.Principal.Id.Resource

	directoryID, err := b.client.GetGroupDirectoryID(ctx, groupID)
	if err != nil {
		if client.IsNotFound(err) {
			return annotations.New(&v2.GrantAlreadyRevoked{}), nil
		}
		return nil, fmt.Errorf("baton-atlassian: failed to resolve group directory: %w", err)
	}

	err = b.client.RemoveUserFromGroup(ctx, directoryID, groupID, accountID)
	if err != nil {
		return nil, fmt.Errorf("baton-atlassian: failed to remove user from group: %w", err)
	}

	return nil, nil
}

func parseIntoGroupResource(group client.Group) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"name":        group.Name,
		"description": group.Description,
	}

	groupTraits := []resource.GroupTraitOption{}
	return resource.NewGroupResource(
		group.Name,
		groupResourceType,
		group.ID,
		groupTraits,
		resource.WithResourceProfile(profile),
	)
}

func newGroupBuilder(c *client.AtlassianClient) *groupBuilder {
	return &groupBuilder{
		resourceType: groupResourceType,
		client:       c,
	}
}
