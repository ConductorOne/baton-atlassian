package connector

import (
	"context"
	"fmt"

	"github.com/conductorone/baton-atlassian/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	"github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	statusActive    = "active"
	statusSuspended = "suspended"
)

type userBuilder struct {
	resourceType *v2.ResourceType
	client       *client.AtlassianClient
}

func (b *userBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return userResourceType
}

func (b *userBuilder) List(ctx context.Context, _ *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	var resources []*v2.Resource

	bag, pageToken, err := getToken(pToken, userResourceType)
	if err != nil {
		return nil, "", nil, err
	}

	users, nextPageToken, err := b.client.ListUsers(ctx, pageToken)
	if err != nil {
		return nil, "", nil, err
	}

	for _, user := range users {
		userResource, err := parseIntoUserResource(user)
		if err != nil {
			return nil, "", nil, err
		}
		resources = append(resources, userResource)
	}

	err = bag.Next(nextPageToken)
	if err != nil {
		return nil, "", nil, err
	}
	nextPageToken, err = bag.Marshal()
	if err != nil {
		return nil, "", nil, err
	}

	return resources, nextPageToken, nil, nil
}

func (b *userBuilder) Entitlements(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

// Grants function will be creating the grants for the Workspaces.
//
// Users have Roles assigned that gives them permissions on each Workspace (product sites).
func (b *userBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	var grantResources []*v2.Grant

	bag, pageToken, err := getToken(pToken, userResourceType)
	if err != nil {
		return nil, "", nil, err
	}

	userID := resource.Id.Resource
	roleAssignments, nextPageToken, err := b.client.GetUserRoleAssignments(ctx, pageToken, userID)
	if err != nil {
		return nil, "", nil, err
	}

	for _, roleAssignment := range roleAssignments {
		// We only want to sync the role assignments for the Atlassian Products.
		// The ignored scopes refers to:
		//     - project: This is a specific context within Jira. Roles with 'project' as the resource owner are typically project roles defined
		// within a particular Jira project (e.g., "Administrators", "Developers", "Users" within a single Jira project).
		//     - goal: This likely refers to roles related to Atlas Goals. If the organization uses Atlas for setting and tracking goals,
		// roles assigned within that product might have 'goal' as the resource owner.
		//     - platform: This generally refers to roles related to the core Atlassian platform itself. This could include roles related
		// to user accounts, organization settings not tied to a specific product, or platform-wide permissions.
		if roleAssignment.ResourceOwner == resourceOwnerProject || roleAssignment.ResourceOwner == resourceOwnerGoal || roleAssignment.ResourceOwner == resourceOwnerPlatform {
			continue
		}

		workspaceResource := &v2.Resource{
			Id: &v2.ResourceId{
				ResourceType: workspaceResourceType.Id,
				Resource:     roleAssignment.ResourceId,
			},
		}

		for _, role := range roleAssignment.Roles {
			entitlementID := fmt.Sprintf("role:%s", role)
			grantResources = append(
				grantResources,
				grant.NewGrant(workspaceResource, entitlementID, resource),
			)
		}
	}

	err = bag.Next(nextPageToken)
	if err != nil {
		return nil, "", nil, err
	}
	nextPageToken, err = bag.Marshal()
	if err != nil {
		return nil, "", nil, err
	}

	return grantResources, nextPageToken, nil, nil
}

func parseIntoUserResource(user client.User) (*v2.Resource, error) {
	var userStatus = v2.UserTrait_Status_STATUS_UNSPECIFIED

	profile := map[string]interface{}{
		"account_id":     user.AccountId,
		"account_type":   user.AccountType,
		"username":       user.Name,
		"email_verified": user.EmailVerified,
	}

	switch user.Status {
	case statusActive:
		userStatus = v2.UserTrait_Status_STATUS_ENABLED
	case statusSuspended:
		userStatus = v2.UserTrait_Status_STATUS_DISABLED
	}

	userTraits := []resource.UserTraitOption{
		resource.WithUserProfile(profile),
		resource.WithStatus(userStatus),
		resource.WithUserLogin(user.Email),
		resource.WithEmail(user.Email, true),
	}

	return resource.NewUserResource(
		user.Email,
		userResourceType,
		user.AccountId,
		userTraits,
	)
}

func (b *userBuilder) CreateAccountCapabilityDetails(ctx context.Context) (*v2.CredentialDetailsAccountProvisioning, annotations.Annotations, error) {
	return &v2.CredentialDetailsAccountProvisioning{
		SupportedCredentialOptions: []v2.CapabilityDetailCredentialOption{
			v2.CapabilityDetailCredentialOption_CAPABILITY_DETAIL_CREDENTIAL_OPTION_NO_PASSWORD,
		},
		PreferredCredentialOption: v2.CapabilityDetailCredentialOption_CAPABILITY_DETAIL_CREDENTIAL_OPTION_NO_PASSWORD,
	}, nil, nil
}

func (b *userBuilder) CreateAccount(ctx context.Context, accountInfo *v2.AccountInfo, credentialOptions *v2.LocalCredentialOptions) (
	connectorbuilder.CreateAccountResponse,
	[]*v2.PlaintextData,
	annotations.Annotations,
	error,
) {
	profileFields := accountInfo.Profile.GetFields()

	userNameVal := profileFields["userName"]
	if userNameVal == nil || userNameVal.GetStringValue() == "" {
		return nil, nil, nil, status.Error(codes.InvalidArgument, "userName is required")
	}
	userName := userNameVal.GetStringValue()

	emailVal := profileFields["email"]
	if emailVal == nil || emailVal.GetStringValue() == "" {
		return nil, nil, nil, status.Error(codes.InvalidArgument, "email is required")
	}
	email := emailVal.GetStringValue()

	directoryIDVal := profileFields["directoryId"]
	if directoryIDVal == nil || directoryIDVal.GetStringValue() == "" {
		return nil, nil, nil, status.Error(codes.InvalidArgument, "directoryId is required")
	}
	directoryID := directoryIDVal.GetStringValue()

	request := client.SCIMCreateUserRequest{
		UserName: userName,
		Emails: []client.SCIMEmail{
			{
				Value:   email,
				Type:    "work", // work or personal
				Primary: true,
			},
		},
		Active: true,
	}

	if givenNameVal := profileFields["givenName"]; givenNameVal != nil && givenNameVal.GetStringValue() != "" {
		request.Name.GivenName = givenNameVal.GetStringValue()
	}
	if familyNameVal := profileFields["familyName"]; familyNameVal != nil && familyNameVal.GetStringValue() != "" {
		request.Name.FamilyName = familyNameVal.GetStringValue()
	}
	if displayNameVal := profileFields["displayName"]; displayNameVal != nil && displayNameVal.GetStringValue() != "" {
		request.DisplayName = displayNameVal.GetStringValue()
	}

	scimResponse, err := b.client.CreateUser(ctx, directoryID, request)
	if err != nil {
		return nil, nil, nil, uhttp.WrapErrors(codes.Internal, "failed to create user", err)
	}

	user := scimUserToUser(scimResponse)

	userResource, err := parseIntoUserResource(user)
	if err != nil {
		return nil, nil, nil, uhttp.WrapErrors(codes.Internal, "failed to parse user resource", err)
	}

	return &v2.CreateAccountResponse_SuccessResult{
		Resource: userResource,
	}, nil, nil, nil
}

func scimUserToUser(scimUser *client.SCIMUserResponse) client.User {
	var email string
	for _, e := range scimUser.Emails {
		if e.Primary {
			email = e.Value
			break
		}
	}
	if email == "" && len(scimUser.Emails) > 0 {
		email = scimUser.Emails[0].Value
	}

	status := statusSuspended
	if scimUser.Active {
		status = statusActive
	}

	return client.User{
		AccountId:     scimUser.ID,
		Status:        status,
		AccountStatus: status,
		Name:          scimUser.DisplayName,
		Email:         email,
		EmailVerified: true,
	}
}

func newUserBuilder(c *client.AtlassianClient) *userBuilder {
	return &userBuilder{
		resourceType: userResourceType,
		client:       c,
	}
}
