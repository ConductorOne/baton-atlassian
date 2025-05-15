package connector

import (
	"context"
	"fmt"

	"github.com/conductorone/baton-atlassian/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/resource"
)

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

	for _, workspace := range *workspaces {
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

func (b *workspaceBuilder) Entitlements(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func (b *workspaceBuilder) Grants(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
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
