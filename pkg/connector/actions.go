package connector

import (
	"context"
	"fmt"

	config_sdk "github.com/conductorone/baton-sdk/pb/c1/config/v1"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/actions"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	successKey = "success"
)

// GlobalActions implements the GlobalActionProvider interface.
// It registers global actions for the connector.
func (c *Connector) GlobalActions(ctx context.Context, registry actions.ActionRegistry) error {
	l := ctxzap.Extract(ctx)

	disableUserSchema := &v2.BatonActionSchema{
		Name:        "disable_user",
		DisplayName: "Disable User",
		Description: "Suspend an Atlassian user's access to the organization",
		Arguments: []*config_sdk.Field{
			{
				Name:        "user_id",
				DisplayName: "User ID",
				Description: "The Atlassian account ID to disable",
				IsRequired:  true,
				Field:       &config_sdk.Field_StringField{},
			},
		},
		ReturnTypes: []*config_sdk.Field{
			{
				Name:        successKey,
				DisplayName: "Success",
				Field:       &config_sdk.Field_BoolField{},
			},
		},
		ActionType: []v2.ActionType{
			v2.ActionType_ACTION_TYPE_ACCOUNT_DISABLE,
		},
	}

	err := registry.Register(ctx, disableUserSchema, func(ctx context.Context, args *structpb.Struct) (*structpb.Struct, annotations.Annotations, error) {
		return c.handleDisableUser(ctx, args)
	})
	if err != nil {
		l.Error("failed to register disable_user action", zap.Error(err))
		return err
	}

	l.Info("registered disable_user action")

	enableUserSchema := &v2.BatonActionSchema{
		Name:        "enable_user",
		DisplayName: "Enable User",
		Description: "Restore an Atlassian user's access to the organization",
		Arguments: []*config_sdk.Field{
			{
				Name:        "user_id",
				DisplayName: "User ID",
				Description: "The Atlassian account ID to enable",
				IsRequired:  true,
				Field:       &config_sdk.Field_StringField{},
			},
		},
		ReturnTypes: []*config_sdk.Field{
			{
				Name:        successKey,
				DisplayName: "Success",
				Field:       &config_sdk.Field_BoolField{},
			},
		},
		ActionType: []v2.ActionType{
			v2.ActionType_ACTION_TYPE_ACCOUNT_ENABLE,
		},
	}

	err = registry.Register(ctx, enableUserSchema, func(ctx context.Context, args *structpb.Struct) (*structpb.Struct, annotations.Annotations, error) {
		return c.handleEnableUser(ctx, args)
	})
	if err != nil {
		l.Error("failed to register enable_user action", zap.Error(err))
		return err
	}

	l.Info("registered enable_user action")
	return nil
}

// handleDisableUser suspends an Atlassian user's access to the organization.
func (c *Connector) handleDisableUser(
	ctx context.Context,
	args *structpb.Struct,
) (*structpb.Struct, annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)

	userIDValue, ok := args.Fields["user_id"]
	if !ok {
		return nil, nil, fmt.Errorf("user_id parameter is required")
	}

	userID := userIDValue.GetStringValue()
	if userID == "" {
		return nil, nil, fmt.Errorf("user_id cannot be empty")
	}

	l.Debug("disabling user", zap.String("user_id", userID))

	err := c.client.DisableUser(ctx, userID)
	if err != nil {
		l.Error("failed to disable user", zap.String("user_id", userID), zap.Error(err))
		return nil, nil, fmt.Errorf("failed to disable user %s: %w", userID, err)
	}

	l.Info("user disabled successfully", zap.String("user_id", userID))

	return &structpb.Struct{
		Fields: map[string]*structpb.Value{
			successKey: structpb.NewBoolValue(true),
		},
	}, nil, nil
}

// handleEnableUser restores an Atlassian user's access to the organization.
func (c *Connector) handleEnableUser(
	ctx context.Context,
	args *structpb.Struct,
) (*structpb.Struct, annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)

	userIDValue, ok := args.Fields["user_id"]
	if !ok {
		return nil, nil, fmt.Errorf("user_id parameter is required")
	}

	userID := userIDValue.GetStringValue()
	if userID == "" {
		return nil, nil, fmt.Errorf("user_id cannot be empty")
	}

	l.Debug("enabling user", zap.String("user_id", userID))

	err := c.client.EnableUser(ctx, userID)
	if err != nil {
		l.Error("failed to enable user", zap.String("user_id", userID), zap.Error(err))
		return nil, nil, fmt.Errorf("failed to enable user %s: %w", userID, err)
	}

	l.Info("user enabled successfully", zap.String("user_id", userID))

	return &structpb.Struct{
		Fields: map[string]*structpb.Value{
			successKey: structpb.NewBoolValue(true),
		},
	}, nil, nil
}
