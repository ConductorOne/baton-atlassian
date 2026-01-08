package main

import (
	"context"
	"fmt"
	"os"

	"github.com/conductorone/baton-atlassian/pkg/config"
	connectorSchema "github.com/conductorone/baton-atlassian/pkg/connector"
	sdkConfig "github.com/conductorone/baton-sdk/pkg/config"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/field"
	"github.com/conductorone/baton-sdk/pkg/types"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

var version = "dev"

func main() {
	ctx := context.Background()

	_, cmd, err := sdkConfig.DefineConfiguration(
		ctx,
		"baton-atlassian",
		getConnector,
		config.Configuration,
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

func getConnector(ctx context.Context, cfg *config.Atlassian) (types.ConnectorServer, error) {
	l := ctxzap.Extract(ctx)

	if err := field.Validate(config.Configuration, cfg); err != nil {
		return nil, err
	}

	connectorBuilder, err := connectorSchema.New(
		ctx,
		cfg.AccessToken,
		cfg.ScimToken,
		cfg.OrganizationId,
		cfg.ScimBaseUrl,
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
