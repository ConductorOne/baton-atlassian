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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
		DisplayName: "Suspend User",
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
		return fmt.Errorf("baton-atlassian: failed to register disable_user action: %w", err)
	}

	l.Info("registered disable_user action")

	enableUserSchema := &v2.BatonActionSchema{
		Name:        "enable_user",
		DisplayName: "Restore User",
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
		return fmt.Errorf("baton-atlassian: failed to register enable_user action: %w", err)
	}

	l.Info("registered enable_user action")

	deactivateUserSchema := &v2.BatonActionSchema{
		Name:        "deactivate_user",
		DisplayName: "Deactivate User",
		Description: "Deactivate a managed Atlassian account across all organizations",
		Arguments: []*config_sdk.Field{
			{
				Name:        "user_id",
				DisplayName: "User ID",
				Description: "The Atlassian account ID to deactivate",
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

	err = registry.Register(ctx, deactivateUserSchema, func(ctx context.Context, args *structpb.Struct) (*structpb.Struct, annotations.Annotations, error) {
		return c.handleDeactivateUser(ctx, args)
	})
	if err != nil {
		return fmt.Errorf("baton-atlassian: failed to register deactivate_user action: %w", err)
	}

	l.Info("registered deactivate_user action")

	activateUserSchema := &v2.BatonActionSchema{
		Name:        "activate_user",
		DisplayName: "Activate User",
		Description: "Reactivate a deactivated Atlassian account",
		Arguments: []*config_sdk.Field{
			{
				Name:        "user_id",
				DisplayName: "User ID",
				Description: "The Atlassian account ID to activate",
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

	err = registry.Register(ctx, activateUserSchema, func(ctx context.Context, args *structpb.Struct) (*structpb.Struct, annotations.Annotations, error) {
		return c.handleActivateUser(ctx, args)
	})
	if err != nil {
		return fmt.Errorf("baton-atlassian: failed to register activate_user action: %w", err)
	}

	l.Info("registered activate_user action")
	return nil
}

// handleDisableUser suspends an Atlassian user's access to the organization.
func (c *Connector) handleDisableUser(
	ctx context.Context,
	args *structpb.Struct,
) (*structpb.Struct, annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)

	userID, err := actions.RequireStringArg(args, "user_id")
	if err != nil {
		return nil, nil, status.Errorf(codes.InvalidArgument, "baton-atlassian: %v", err)
	}
	if userID == "" {
		return nil, nil, status.Error(codes.InvalidArgument, "baton-atlassian: user_id cannot be empty")
	}

	l.Debug("disabling user", zap.String("user_id", userID))

	err = c.client.DisableUser(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("baton-atlassian: failed to disable user %s: %w", userID, err)
	}

	l.Info("user disabled successfully", zap.String("user_id", userID))

	return actions.NewReturnValues(true), nil, nil
}

// handleEnableUser restores an Atlassian user's access to the organization.
func (c *Connector) handleEnableUser(
	ctx context.Context,
	args *structpb.Struct,
) (*structpb.Struct, annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)

	userID, err := actions.RequireStringArg(args, "user_id")
	if err != nil {
		return nil, nil, status.Errorf(codes.InvalidArgument, "baton-atlassian: %v", err)
	}
	if userID == "" {
		return nil, nil, status.Error(codes.InvalidArgument, "baton-atlassian: user_id cannot be empty")
	}

	l.Debug("enabling user", zap.String("user_id", userID))

	err = c.client.EnableUser(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("baton-atlassian: failed to enable user %s: %w", userID, err)
	}

	l.Info("user enabled successfully", zap.String("user_id", userID))

	return actions.NewReturnValues(true), nil, nil
}

// handleDeactivateUser deactivates a managed Atlassian account in every organization.
func (c *Connector) handleDeactivateUser(
	ctx context.Context,
	args *structpb.Struct,
) (*structpb.Struct, annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)

	userID, err := actions.RequireStringArg(args, "user_id")
	if err != nil {
		return nil, nil, status.Errorf(codes.InvalidArgument, "baton-atlassian: %v", err)
	}
	if userID == "" {
		return nil, nil, status.Error(codes.InvalidArgument, "baton-atlassian: user_id cannot be empty")
	}

	l.Debug("deactivating user", zap.String("user_id", userID))

	err = c.client.DeactivateUser(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("baton-atlassian: failed to deactivate user %s: %w", userID, err)
	}

	l.Info("user deactivated successfully", zap.String("user_id", userID))

	return actions.NewReturnValues(true), nil, nil
}

// handleActivateUser reactivates a deactivated Atlassian account.
func (c *Connector) handleActivateUser(
	ctx context.Context,
	args *structpb.Struct,
) (*structpb.Struct, annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)

	userID, err := actions.RequireStringArg(args, "user_id")
	if err != nil {
		return nil, nil, status.Errorf(codes.InvalidArgument, "baton-atlassian: %v", err)
	}
	if userID == "" {
		return nil, nil, status.Error(codes.InvalidArgument, "baton-atlassian: user_id cannot be empty")
	}

	l.Debug("activating user", zap.String("user_id", userID))

	err = c.client.ActivateUser(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("baton-atlassian: failed to activate user %s: %w", userID, err)
	}

	l.Info("user activated successfully", zap.String("user_id", userID))

	return actions.NewReturnValues(true), nil, nil
}
