package main

import (
	"context"
	"fmt"
	"os"

	cfg "github.com/conductorone/baton-atlassian/pkg/config"
	connectorSchema "github.com/conductorone/baton-atlassian/pkg/connector"
	"github.com/conductorone/baton-sdk/pkg/config"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/field"
	"github.com/conductorone/baton-sdk/pkg/types"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

var version = "dev"

func main() {
	ctx := context.Background()

	_, cmd, err := config.DefineConfiguration(
		ctx,
		"baton-atlassian",
		getConnector,
		cfg.Configuration,
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

func getConnector(ctx context.Context, config *cfg.Atlassian) (types.ConnectorServer, error) {
	l := ctxzap.Extract(ctx)

	if err := field.Validate(cfg.Configuration, config); err != nil {
		return nil, err
	}

	accessToken := config.GetString(cfg.AccessTokenField.FieldName)
	organizationID := config.GetString(cfg.OrganizationIDField.FieldName)

	connectorBuilder, err := connectorSchema.New(
		ctx,
		accessToken,
		organizationID,
	)

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
