package connector

import (
	"context"

	"fmt"
	"sync"

	"github.com/conductorone/baton-atlassian/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	"github.com/conductorone/baton-sdk/pkg/types/resource"
)

type teamBuilder struct {
	resourceType *v2.ResourceType
	client       *client.AtlassianClient
	teams        []client.TeamEdge
	teamsMutex   sync.RWMutex
}

var teamMembershipRoles = []string{"REGULAR", "ADMIN"}

func (o *teamBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return teamResourceType
}

func (o *teamBuilder) List(ctx context.Context, _ *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	var resources []*v2.Resource

	nextPageToken, annotation, err := o.GetTeams(ctx, pToken)

	if err != nil {
		return nil, "", nil, err
	}

	for _, team := range o.teams {
		teamCopy := team.Node.Team
		teamResource, err := parseIntoTeamResource(ctx, &teamCopy, nil)

		if err != nil {
			return nil, "", nil, err
		}
		resources = append(resources, teamResource)
	}

	return resources, nextPageToken, annotation, nil
}

func (o *teamBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	var entitlements []*v2.Entitlement
	for _, teamMembershipRole := range teamMembershipRoles {
		assigmentOptions := []entitlement.EntitlementOption{
			entitlement.WithGrantableTo(userResourceType),
			entitlement.WithDescription(fmt.Sprintf("Team Membership Role %s for team %s", teamMembershipRole, resource.DisplayName)),
			entitlement.WithDisplayName(fmt.Sprintf("%s Team %s", resource.DisplayName, teamMembershipRole)),
		}

		entitlements = append(entitlements, entitlement.NewPermissionEntitlement(resource, teamMembershipRole, assigmentOptions...))
	}

	return entitlements, "", nil, nil
}

func (o *teamBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	var grants []*v2.Grant

	nextPageToken, annotation, err := o.GetTeams(ctx, pToken)

	if err != nil {
		return nil, "", nil, err
	}

	for _, team := range o.teams {
		if team.Node.Team.ID == resource.Id.Resource {
			for _, member := range team.Node.Team.Members.Edges {
				memberCopy := member.Node.Member
				memberRole := member.Node.Role

				userResource, _ := parseIntoUserResource(ctx, &memberCopy, resource.Id)
				memberGrant := grant.NewGrant(resource, memberRole, userResource, grant.WithAnnotation(&v2.V1Identifier{
					Id: fmt.Sprintf("team-grant:%s:%s:%s", resource.Id.Resource, memberCopy.ID, memberRole),
				}))
				grants = append(grants, memberGrant)
			}
		}
	}

	return grants, nextPageToken, annotation, nil
}

func newTeamBuilder(c *client.AtlassianClient) *teamBuilder {
	return &teamBuilder{
		resourceType: teamResourceType,
		client:       c,
	}
}

func parseIntoTeamResource(_ context.Context, team *client.Team, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"team_id":      team.ID,
		"display_name": team.DisplayName,
		"description":  team.Description,
	}

	groupTraits := []resource.GroupTraitOption{
		resource.WithGroupProfile(profile),
	}

	displayName := team.DisplayName

	ret, err := resource.NewGroupResource(
		displayName,
		teamResourceType,
		team.ID,
		groupTraits,
		resource.WithParentResourceID(parentResourceID),
	)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (o *teamBuilder) GetTeams(ctx context.Context, pToken *pagination.Token) (string, annotations.Annotations, error) {
	o.teamsMutex.RLock()
	defer o.teamsMutex.RUnlock()

	if o.teams != nil || len(o.teams) > 0 {
		return "", nil, nil
	}

	bag, pageToken, err := getToken(pToken, userResourceType)
	if err != nil {
		return "", nil, err
	}
	teams, nextPageToken, _, err := o.client.ListTeams(ctx, client.PageOptions{
		PageSize:  pToken.Size,
		PageToken: pageToken,
	})

	if err != nil {
		return "", nil, err
	}

	err = bag.Next(nextPageToken)
	if err != nil {
		return "", nil, err
	}

	o.teams = teams

	nextPageToken, err = bag.Marshal()
	if err != nil {
		return "", nil, err
	}

	return nextPageToken, nil, nil
}
