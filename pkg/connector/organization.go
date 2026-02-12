package connector

import (
	"context"
	"fmt"

	"github.com/conductorone/baton-atlassian/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/resource"
)

// organizationRoles are the roles that can be assigned at the organization/platform level.
// Valid values according to Atlassian Admin API documentation:
// https://developer.atlassian.com/cloud/admin/organization/rest/api-group-users/#api-v2-orgs-orgid-directories-directoryid-users-get
var organizationRoles = []string{
	"atlassian/org-admin",
	"atlassian/site-admin",
	"atlassian/user-access-admin",
	"atlassian/ai-access",
}

type organizationBuilder struct {
	resourceType *v2.ResourceType
	client       *client.AtlassianClient
}

func (b *organizationBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return organizationResourceType
}

// List returns a single organization resource representing the Atlassian organization.
func (b *organizationBuilder) List(ctx context.Context, _ *v2.ResourceId, _ *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	// Get organization details including the name
	org, err := b.client.GetOrganization(ctx)
	if err != nil {
		return nil, "", nil, err
	}

	// Use organization name if available, fallback to ID
	displayName := org.Attributes.Name
	if displayName == "" {
		displayName = org.ID
	}

	// Create the organization resource using the organization name
	orgResource, err := resource.NewResource(
		displayName,
		organizationResourceType,
		org.ID,
	)
	if err != nil {
		return nil, "", nil, err
	}

	return []*v2.Resource{orgResource}, "", nil, nil
}

// Entitlements returns entitlements for org-admin and site-admin roles on the organization.
func (b *organizationBuilder) Entitlements(_ context.Context, res *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	var orgEntitlements []*v2.Entitlement

	for _, role := range organizationRoles {
		displayName := fmt.Sprintf("%s on %s", role, res.DisplayName)
		description := fmt.Sprintf("Role '%s' applied at organization level.", role)
		assignmentOptions := []entitlement.EntitlementOption{
			// Only users can be granted organization-level roles directly.
			// Groups with platform roles are not synced to avoid redundancy
			// (platform roles are synced directly from user role-assignments).
			entitlement.WithGrantableTo(userResourceType),
			entitlement.WithDescription(description),
			entitlement.WithDisplayName(displayName),
		}
		roleID := fmt.Sprintf("role:%s", role)
		orgEntitlements = append(orgEntitlements, entitlement.NewPermissionEntitlement(res, roleID, assignmentOptions...))
	}

	return orgEntitlements, "", nil, nil
}

// Grants returns empty grants - grants are created from the user and group builders.
func (b *organizationBuilder) Grants(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func newOrganizationBuilder(c *client.AtlassianClient) *organizationBuilder {
	return &organizationBuilder{
		resourceType: organizationResourceType,
		client:       c,
	}
}
