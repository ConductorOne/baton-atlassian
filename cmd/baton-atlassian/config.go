package main

import (
	"github.com/conductorone/baton-sdk/pkg/field"
	"github.com/spf13/viper"
)

var (
	// TODO: Delete userEmailField
	userEmailField = field.StringField(
		"user-email",
		field.WithRequired(false),
		field.WithDescription("User email used to authenticate to Atlassian API"),
	)
	// TODO: Delete apiTokenField
	apiTokenField = field.StringField(
		"api-token",
		field.WithRequired(false),
		field.WithDescription("The API token to get access to Atlassian API."),
	)
	// TODO: Delete organizationField
	organizationField = field.StringField(
		"organization",
		field.WithDescription("Limit syncing to specific organization by providing organization ID."),
		field.WithRequired(false),
	)

	accessTokenField = field.StringField(
		"access-token",
		field.WithDescription("Access Token used to authenticate with the Atlassian API."),
		field.WithRequired(true),
	)

	organizationIDField = field.StringField(
		"organization-id",
		field.WithDescription("Organization that will be synchronized."),
		field.WithRequired(true),
	)

	// TODO: Delete siteIDField
	siteIdField = field.StringField(
		"site-id",
		field.WithDescription("Limit syncing to specific sites by providing site slugs."),
		field.WithRequired(false),
		field.WithDefaultValue("None"),
	)

	ConfigurationFields = []field.SchemaField{
		accessTokenField, organizationIDField,
		userEmailField, apiTokenField, organizationField, siteIdField,
	}

	FieldRelationships = []field.SchemaFieldRelationship{}
)

// ValidateConfig is run after the configuration is loaded, and should return an
// error if it isn't valid. Implementing this function is optional, it only
// needs to perform extra validations that cannot be encoded with configuration
// parameters.
func ValidateConfig(_ *viper.Viper) error {
	return nil
}
