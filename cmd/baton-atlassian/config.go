package main

import (
	"github.com/conductorone/baton-sdk/pkg/field"
	"github.com/spf13/viper"
)

var (
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

	ConfigurationFields = []field.SchemaField{
		accessTokenField, organizationIDField,
	}
)

// ValidateConfig is run after the configuration is loaded, and should return an
// error if it isn't valid. Implementing this function is optional, it only
// needs to perform extra validations that cannot be encoded with configuration
// parameters.
func ValidateConfig(_ *viper.Viper) error {
	return nil
}
