package connector

import (
	"context"

	"io"

	"github.com/conductorone/baton-atlassian/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

type Connector struct {
	client *client.AtlassianClient
}

// ResourceSyncers returns a ResourceSyncer for each resource type that should be synced from the upstream service.
func (d *Connector) ResourceSyncers(_ context.Context) []connectorbuilder.ResourceSyncer {
	return []connectorbuilder.ResourceSyncer{
		newUserBuilder(d.client),
		newWorkspaceBuilder(d.client),
		newGroupBuilder(d.client),
	}
}

// Asset takes an input AssetRef and attempts to fetch it using the connector's authenticated http client
// It streams a response, always starting with a metadata object, following by chunked payloads for the asset.
func (d *Connector) Asset(_ context.Context, _ *v2.AssetRef) (string, io.ReadCloser, error) {
	return "", nil, nil
}

// Metadata returns metadata about the connector.
func (d *Connector) Metadata(_ context.Context) (*v2.ConnectorMetadata, error) {
	return &v2.ConnectorMetadata{
		DisplayName: "Atlassian Core",
		Description: "Connector to sync data from an Atlassian Organization",
		AccountCreationSchema: &v2.ConnectorAccountCreationSchema{
			FieldMap: map[string]*v2.ConnectorAccountCreationSchema_Field{
				"userName": {
					DisplayName: "Username",
					Description: "The username for the new user (typically their email address)",
					Required:    true,
					Placeholder: "user@example.com",
					Order:       1,
					Field: &v2.ConnectorAccountCreationSchema_Field_StringField{
						StringField: &v2.ConnectorAccountCreationSchema_StringField{},
					},
				},
				"email": {
					DisplayName: "Email",
					Description: "The primary email address for the new user",
					Required:    true,
					Placeholder: "user@example.com",
					Order:       2,
					Field: &v2.ConnectorAccountCreationSchema_Field_StringField{
						StringField: &v2.ConnectorAccountCreationSchema_StringField{},
					},
				},
				"directoryId": {
					DisplayName: "Directory ID",
					Description: "The Atlassian directory ID where the user will be created",
					Required:    true,
					Placeholder: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
					Order:       3,
					Field: &v2.ConnectorAccountCreationSchema_Field_StringField{
						StringField: &v2.ConnectorAccountCreationSchema_StringField{},
					},
				},
				"givenName": {
					DisplayName: "First Name",
					Description: "The user's first name (optional)",
					Required:    false,
					Placeholder: "John",
					Order:       4,
					Field: &v2.ConnectorAccountCreationSchema_Field_StringField{
						StringField: &v2.ConnectorAccountCreationSchema_StringField{},
					},
				},
				"familyName": {
					DisplayName: "Last Name",
					Description: "The user's last name (optional)",
					Required:    false,
					Placeholder: "Doe",
					Order:       5,
					Field: &v2.ConnectorAccountCreationSchema_Field_StringField{
						StringField: &v2.ConnectorAccountCreationSchema_StringField{},
					},
				},
				"displayName": {
					DisplayName: "Display Name",
					Description: "The user's display name (optional)",
					Required:    false,
					Placeholder: "John Doe",
					Order:       6,
					Field: &v2.ConnectorAccountCreationSchema_Field_StringField{
						StringField: &v2.ConnectorAccountCreationSchema_StringField{},
					},
				},
			},
		},
	}, nil
}

// Validate is called to ensure that the connector is properly configured. It should exercise any API credentials
// to be sure that they are valid.
func (d *Connector) Validate(_ context.Context) (annotations.Annotations, error) {
	return nil, nil
}

// New returns a new instance of the connector.
func New(ctx context.Context, accessToken, organizationID string) (*Connector, error) {
	l := ctxzap.Extract(ctx)

	atlassianClient, err := client.New(
		ctx,
		client.WithAccessToken(accessToken),
		client.WithOrganizationID(organizationID),
	)
	if err != nil {
		l.Error("error creating Atlassian client", zap.Error(err))
		return nil, err
	}

	return &Connector{
		client: atlassianClient,
	}, nil
}
