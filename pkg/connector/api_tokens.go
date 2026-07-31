package connector

import (
	"context"
	"time"

	"github.com/conductorone/baton-atlassian/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/resource"
)

type apiTokenBuilder struct {
	resourceType *v2.ResourceType
	client       *client.AtlassianClient
}

func (b *apiTokenBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return apiTokenResourceType
}

func (b *apiTokenBuilder) List(ctx context.Context, _ *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	var resources []*v2.Resource

	bag, pageToken, err := getToken(pToken, apiTokenResourceType)
	if err != nil {
		return nil, "", nil, err
	}

	tokens, nextPageToken, err := b.client.ListAPITokens(ctx, pageToken)
	if err != nil {
		return nil, "", nil, err
	}

	for _, token := range tokens {
		tokenResource, err := parseIntoAPITokenResource(token)
		if err != nil {
			return nil, "", nil, err
		}
		resources = append(resources, tokenResource)
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

func (b *apiTokenBuilder) Entitlements(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func (b *apiTokenBuilder) Grants(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func parseIntoAPITokenResource(token client.APIToken) (*v2.Resource, error) {
	// "atlassian.api_token" is the platform-specific credential kind stamped on the
	// SecretTrait, per the NHI ontology (§2.8, dotted lowercase).
	traitOpts := []resource.SecretTraitOption{
		resource.WithSecretType(v2.SecretTrait_CREDENTIAL_TYPE_STATIC_SECRET),
		resource.WithSecretDetail("atlassian.api_token"),
	}

	var resourceOpts []resource.ResourceOption
	if ts, ok := parseAtlassianTime(token.CreatedAt); ok {
		resourceOpts = append(resourceOpts, resource.WithResourceCreatedAt(ts))
	}
	if ts, ok := parseAtlassianTime(token.ExpiresAt); ok {
		traitOpts = append(traitOpts, resource.WithSecretExpiresAt(ts))
	}
	if ts, ok := parseAtlassianTime(token.LastActiveAt); ok {
		traitOpts = append(traitOpts, resource.WithSecretLastUsedAt(ts))
	}

	// Link the token back to the user that owns it so the secret correlates to its identity.
	if token.User.ID != "" {
		traitOpts = append(traitOpts, resource.WithSecretIdentityID(&v2.ResourceId{
			ResourceType: userResourceType.Id,
			Resource:     token.User.ID,
		}))
	}

	displayName := token.Label
	if displayName == "" {
		displayName = token.ID
	}

	return resource.NewSecretResource(
		displayName,
		apiTokenResourceType,
		token.ID,
		traitOpts,
		resourceOpts...,
	)
}

func parseAtlassianTime(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	ts, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, false
	}
	return ts, true
}

func newAPITokenBuilder(c *client.AtlassianClient) *apiTokenBuilder {
	return &apiTokenBuilder{
		resourceType: apiTokenResourceType,
		client:       c,
	}
}
