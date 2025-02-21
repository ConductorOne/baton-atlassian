package main

import (
	"github.com/conductorone/baton-sdk/pkg/field"
	"github.com/spf13/viper"
)

var (
	userEmailField = field.StringField(
		"user-email",
		field.WithRequired(true),
		field.WithDescription("User email used to authenticate to Atlassian API"),
	)
	apiTokenField = field.StringField(
		"api-token",
		field.WithRequired(true),
		field.WithDescription("The API token to get access to Atlassian API."),
	)
	organizationField = field.StringField(
		"organization",
		field.WithDescription("Limit syncing to specific organization by providing organization ID."),
		field.WithRequired(true),
	)
	siteIdField = field.StringField(
		"site-id",
		field.WithDescription("Limit syncing to specific sites by providing site slugs."),
		field.WithRequired(false),
		field.WithDefaultValue("None"),
	)
	// ConfigurationFields defines the external configuration required for the
	// connector to run. Note: these fields can be marked as optional or
	// required.
	ConfigurationFields = []field.SchemaField{userEmailField, apiTokenField, organizationField, siteIdField}

	// FieldRelationships defines relationships between the fields listed in
	// ConfigurationFields that can be automatically validated. For example, a
	// username and password can be required together, or an access token can be
	// marked as mutually exclusive from the username password pair.
	FieldRelationships = []field.SchemaFieldRelationship{}
)

// ValidateConfig is run after the configuration is loaded, and should return an
// error if it isn't valid. Implementing this function is optional, it only
// needs to perform extra validations that cannot be encoded with configuration
// parameters.
func ValidateConfig(v *viper.Viper) error {
	return nil
}
