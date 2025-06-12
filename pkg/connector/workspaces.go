package connector

import (
	"context"
	"fmt"
	"strings"

	"github.com/conductorone/baton-atlassian/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/resource"
)

var atlassianRoles = []string{
	"atlassian/user",
	"atlassian/user-access-admin",
	"atlassian/admin",
	"atlassian/guest",
	"atlassian/contributor",
	"atlassian/customer",
	"atlassian/basic",
	"atlassian/stakeholder",
	"atlassian/site-admin",
	"atlassian/org-admin",
}

type workspaceBuilder struct {
	resourceType *v2.ResourceType
	client       *client.AtlassianClient
}

func (b *workspaceBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return workspaceResourceType
}

func (b *workspaceBuilder) List(ctx context.Context, _ *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	var resources []*v2.Resource
	bag, pageToken, err := getToken(pToken, workspaceResourceType)

	if err != nil {
		return nil, "", nil, err
	}

	workspaces, nextPageToken, err := b.client.ListWorkspaces(ctx, pageToken)
	if err != nil {
		return nil, "", nil, err
	}

	for _, workspace := range workspaces {
		workspaceResource, err := parseIntoWorkspaceResource(workspace)
		if err != nil {
			return nil, "", nil, err
		}
		resources = append(resources, workspaceResource)
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

func (b *workspaceBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	var workspaceEntitlements []*v2.Entitlement

	for _, role := range atlassianRoles {
		displayName := fmt.Sprintf("%s on %s", role, resource.DisplayName)
		description := fmt.Sprintf("Role '%s' applied in %s.", role, resource.DisplayName)
		assigmentOptions := []entitlement.EntitlementOption{
			entitlement.WithGrantableTo(userResourceType, groupResourceType),
			entitlement.WithDescription(description),
			entitlement.WithDisplayName(displayName),
		}
		roleID := fmt.Sprintf("role:%s", role)
		workspaceEntitlements = append(workspaceEntitlements, entitlement.NewPermissionEntitlement(resource, roleID, assigmentOptions...))
	}

	return workspaceEntitlements, "", nil, nil
}

// Grants are created on both Users and Groups resources, since the model is made with that approach, and it's easier and more efficient like this.
func (b *workspaceBuilder) Grants(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func (b *workspaceBuilder) Grant(ctx context.Context, principal *v2.Resource, entitlement *v2.Entitlement) (annotations.Annotations, error) {
	userID := principal.Id.Resource
	workspaceID, roleID, err := extractIDsFromEntitlements(entitlement.Id)
	if err != nil {
		return nil, err
	}

	err = b.client.AssignRoleToUser(ctx, userID, workspaceID, roleID)
	if err != nil {
		return annotations.Annotations{}, err
	}

	return nil, nil
}

func (b *workspaceBuilder) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	userID := grant.Principal.Id.Resource
	workspaceID, roleID, err := extractIDsFromEntitlements(grant.Entitlement.Id)
	if err != nil {
		return nil, err
	}

	err = b.client.RevokeRoleFromUser(ctx, userID, workspaceID, roleID)
	if err != nil {
		return annotations.Annotations{}, err
	}

	return nil, nil
}

func extractIDsFromEntitlements(entitlementID string) (string, string, error) {
	entitlementIDSegments := strings.Split(entitlementID, ":")
	amountOfSegments := len(entitlementIDSegments)
	if amountOfSegments < 4 || entitlementIDSegments[0] != "workspace" || entitlementIDSegments[amountOfSegments-2] != "role" {
		return "", "", fmt.Errorf("invalid entitlementID format: %s", entitlementID)
	}

	roleID := entitlementIDSegments[amountOfSegments-1]
	workspaceID := strings.Join(entitlementIDSegments[1:amountOfSegments-2], ":")

	return workspaceID, roleID, nil
}

func parseIntoWorkspaceResource(workspace client.Workspace) (*v2.Resource, error) {
	displayName := fmt.Sprintf("%s: %s", workspace.Attributes.Type, workspace.Attributes.Name)
	return resource.NewResource(
		displayName,
		workspaceResourceType,
		workspace.Id,
	)
}

func newWorkspaceBuilder(c *client.AtlassianClient) *workspaceBuilder {
	return &workspaceBuilder{
		resourceType: workspaceResourceType,
		client:       c,
	}
}
