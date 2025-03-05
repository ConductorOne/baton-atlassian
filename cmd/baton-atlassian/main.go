package main

import (
	"context"
	"fmt"
	"os"

	connectorSchema "github.com/conductorone/baton-atlassian/pkg/connector"
	"github.com/conductorone/baton-sdk/pkg/config"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/field"
	"github.com/conductorone/baton-sdk/pkg/types"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var version = "dev"

func main() {
	ctx := context.Background()

	_, cmd, err := config.DefineConfiguration(
		ctx,
		"baton-atlassian",
		getConnector,
		field.Configuration{
			Fields: ConfigurationFields,
		},
	)
	if err != nil {
		_, err := fmt.Fprintln(os.Stderr, err.Error())
		if err != nil {
			return
		}
		os.Exit(1)
	}

	cmd.Version = version

	err = cmd.Execute()
	if err != nil {
		_, err := fmt.Fprintln(os.Stderr, err.Error())
		if err != nil {
			return
		}
		os.Exit(1)
	}
}

func getConnector(ctx context.Context, v *viper.Viper) (types.ConnectorServer, error) {
	l := ctxzap.Extract(ctx)

	if err := ValidateConfig(v); err != nil {
		return nil, err
	}

	userEmail := v.GetString(userEmailField.FieldName)
	apiToken := v.GetString(apiTokenField.FieldName)
	organization := v.GetString(organizationField.FieldName)
	siteId := v.GetString(siteIdField.FieldName)

	connectorBuilder, err := connectorSchema.New(ctx, userEmail, apiToken, organization, siteId)
	if err != nil {
		l.Error("error creating connector", zap.Error(err))
		return nil, err
	}

	opts := make([]connectorbuilder.Opt, 0)

	connector, err := connectorbuilder.NewConnector(ctx, connectorBuilder, opts...)
	if err != nil {
		l.Error("error creating connector", zap.Error(err))
		return nil, err
	}
	return connector, nil
}
