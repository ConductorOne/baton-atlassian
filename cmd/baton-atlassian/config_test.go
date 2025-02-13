package main

import (
	"testing"

	"github.com/conductorone/baton-sdk/pkg/field"
	"github.com/conductorone/baton-sdk/pkg/test"
)

func TestConfigs(t *testing.T) {
	configurationSchema := field.NewConfiguration(
		ConfigurationFields,
		FieldRelationships...,
	)

	test.ExerciseTestCases(t, configurationSchema, ValidateConfig, []test.TestCase{})
}
