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

	ConfigurationFields = []field.SchemaField{
		AccessTokenField,
		OrganizationIDField,
	}
)

var Configuration = field.NewConfiguration(
	ConfigurationFields,
	field.WithConnectorDisplayName("Atlassian"),
	field.WithHelpUrl("/docs/baton/atlassian"),
	field.WithIconUrl("/static/app-icons/atlassian.svg"),
)
