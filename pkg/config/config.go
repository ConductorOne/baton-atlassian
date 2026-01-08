//go:generate go run ./gen
package config

import (
	"github.com/conductorone/baton-sdk/pkg/field"
)

var (
	AccessTokenField = field.StringField(
		"access-token",
		field.WithDescription("Access Token used to authenticate with the Atlassian API."),
		field.WithRequired(true),
		field.WithIsSecret(true),
		field.WithDisplayName("Access Token"),
	)

	OrganizationIDField = field.StringField(
		"organization-id",
		field.WithDescription("Organization that will be synchronized."),
		field.WithRequired(true),
		field.WithDisplayName("Organization ID"),
	)

	ScimTokenField = field.StringField(
		"scim-token",
		field.WithDescription("SCIM Token used for user provisioning operations."),
		field.WithRequired(false),
		field.WithIsSecret(true),
		field.WithDisplayName("SCIM Token"),
	)

	ScimBaseUrlField = field.StringField(
		"scim-base-url",
		field.WithDescription("The SCIM base URL for user provisioning (you can get it along with the token)"),
		field.WithRequired(false),
		field.WithDisplayName("SCIM Base URL"),
	)

	ConfigurationFields = []field.SchemaField{
		AccessTokenField,
		OrganizationIDField,
		ScimTokenField,
		ScimBaseUrlField,
	}
)

var Configuration = field.NewConfiguration(
	ConfigurationFields,
	field.WithConnectorDisplayName("Atlassian"),
	field.WithHelpUrl("/docs/baton/atlassian"),
	field.WithIconUrl("/static/app-icons/atlassian.svg"),
)
