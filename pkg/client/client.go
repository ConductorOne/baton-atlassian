package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	baseURL          = "https://api.atlassian.com/admin"
	lifecycleBaseURL = "https://api.atlassian.com"

	usersEP                = "v2/orgs/%s/directories/-/users"
	usersSearchEP          = "v2/orgs/%s/directories/%s/users/search"
	workspacesEP           = "v2/orgs/%s/workspaces"
	groupsEP               = "v2/orgs/%s/directories/-/groups"
	usersRoleAssignmentEP  = "v2/orgs/%s/directories/-/users/%s/role-assignments"
	groupsRoleAssignmentEP = "v2/orgs/%s/directories/-/groups/%s/role-assignments"
	groupDetailEP          = "v2/orgs/%s/directories/-/groups/%s"
	groupMembershipsEP     = "v2/orgs/%s/directories/%s/groups/%s/memberships"
	groupMembershipEP      = "v2/orgs/%s/directories/%s/groups/%s/memberships/%s"
	directoriesEP          = "v2/orgs/%s/directories"
	userDirectorySuspendEP = "v2/orgs/%s/directories/%s/users/%s/suspend"
	userDirectoryRestoreEP = "v2/orgs/%s/directories/%s/users/%s/restore"

	userAssignRolesEP = "v1/orgs/%s/users/%s/roles/assign"
	userRevokeRolesEP = "v1/orgs/%s/users/%s/roles/revoke"

	userLifecycleDeleteEP  = "users/%s/manage/lifecycle/delete"
	userLifecycleDisableEP = "users/%s/manage/lifecycle/disable"
	userLifecycleEnableEP  = "users/%s/manage/lifecycle/enable"

	organizationEP = "v1/orgs/%s"

	apiTokensEP = "api-access/v1/orgs/%s/api-tokens"
)

type AtlassianClient struct {
	wrapper *uhttp.BaseHttpClient
	config  Config
}

type Config struct {
	accessToken    string
	scimToken      string
	organizationID string
	scimBaseUrl    string
	baseURL        string
}

type Option func(*AtlassianClient)

func WithAccessToken(accessToken string) Option {
	return func(c *AtlassianClient) {
		c.config.accessToken = accessToken
	}
}

func WithScimToken(scimToken string) Option {
	return func(c *AtlassianClient) {
		c.config.scimToken = scimToken
	}
}

func WithScimBaseUrl(scimBaseUrl string) Option {
	return func(c *AtlassianClient) {
		c.config.scimBaseUrl = scimBaseUrl
	}
}

func WithOrganizationID(orgID string) Option {
	return func(c *AtlassianClient) {
		c.config.organizationID = orgID
	}
}

func WithBaseURL(baseURL string) Option {
	return func(c *AtlassianClient) {
		c.config.baseURL = baseURL
	}
}

func (c *AtlassianClient) getBaseURL() string {
	if c.config.baseURL != "" {
		return c.config.baseURL
	}
	return baseURL
}

func (c *AtlassianClient) getLifecycleBaseURL() string {
	if c.config.baseURL != "" {
		return c.config.baseURL
	}
	return lifecycleBaseURL
}

// GetOrganizationID returns the organization ID from the client configuration.
func (c *AtlassianClient) GetOrganizationID() string {
	return c.config.organizationID
}

// GetOrganization returns the organization details including its name.
func (c *AtlassianClient) GetOrganization(ctx context.Context) (*Organization, error) {
	var orgResponse OrganizationResponse
	requestURL, err := url.JoinPath(c.getBaseURL(), fmt.Sprintf(organizationEP, url.PathEscape(c.config.organizationID)))
	if err != nil {
		return nil, err
	}

	_, err = c.doRequest(ctx,
		http.MethodGet,
		requestURL,
		&orgResponse,
		nil,
	)
	if err != nil {
		return nil, err
	}

	return &orgResponse.Data, nil
}

// ListDirectories returns a page of the organization's user directories. GET v2/orgs/{org}/directories.
// Requires read:directories:admin.
func (c *AtlassianClient) ListDirectories(ctx context.Context, pageToken string) ([]Directory, string, error) {
	var directoriesResponse DirectoriesResponse
	requestURL, err := url.JoinPath(c.getBaseURL(), fmt.Sprintf(directoriesEP, url.PathEscape(c.config.organizationID)))
	if err != nil {
		return nil, "", err
	}

	reqOpts := []RequestOpt{WithPageSize(maxItemsPerPage)}
	if pageToken != "" {
		reqOpts = append(reqOpts, WithPageToken(pageToken))
	}
	_, err = c.doRequest(ctx,
		http.MethodGet,
		requestURL,
		&directoriesResponse,
		nil,
		reqOpts...,
	)
	if err != nil {
		return nil, "", err
	}

	return directoriesResponse.Data, directoriesResponse.Links.Next, nil
}

// SearchUsersWithGroups returns a page of the users in one directory, each with the groups it
// belongs to in that directory. POST v2/orgs/{org}/directories/{directoryID}/users/search with
// expand=groups.
//
// This reads group membership from the user side, so its cost grows with the number of users
// rather than with the size of any one group. Listing a very large group's members with the
// users?groupIds= filter instead can exceed Atlassian's gateway timeout on every attempt.
func (c *AtlassianClient) SearchUsersWithGroups(ctx context.Context, directoryID, pageToken string) ([]SearchUser, string, error) {
	var searchResponse UserSearchResponse
	requestURL, err := url.JoinPath(c.getBaseURL(), fmt.Sprintf(usersSearchEP, url.PathEscape(c.config.organizationID), url.PathEscape(directoryID)))
	if err != nil {
		return nil, "", err
	}

	body := UserSearchRequest{
		Limit:  maxItemsPerPage,
		Expand: []string{"groups"},
		Cursor: pageToken,
	}
	_, err = c.doRequest(ctx,
		http.MethodPost,
		requestURL,
		&searchResponse,
		body,
	)
	if err != nil {
		return nil, "", err
	}

	return searchResponse.Data, searchResponse.Links.Next, nil
}

func (c *AtlassianClient) ListUsers(ctx context.Context, pageToken string) ([]User, string, error) {
	var usersResponse UserResponse
	requestURL, err := url.JoinPath(c.getBaseURL(), fmt.Sprintf(usersEP, url.PathEscape(c.config.organizationID)))
	if err != nil {
		return nil, "", err
	}

	reqOpts := []RequestOpt{WithPageSize(maxItemsPerPage)}
	if pageToken != "" {
		reqOpts = append(reqOpts, WithPageToken(pageToken))
	}
	_, err = c.doRequest(ctx,
		http.MethodGet,
		requestURL,
		&usersResponse,
		nil,
		reqOpts...,
	)
	if err != nil {
		return nil, "", err
	}

	nextPageToken := usersResponse.Links.Next

	return usersResponse.Data, nextPageToken, nil
}

// ListAPITokens enumerates all user API tokens in the organization via the admin
// API-access endpoint. Requires the organization API key (the access token this client
// already uses); Forge/OAuth2 apps cannot access this resource.
func (c *AtlassianClient) ListAPITokens(ctx context.Context, pageToken string) ([]APIToken, string, error) {
	var tokensResponse APITokenResponse
	requestURL, err := url.JoinPath(c.getBaseURL(), fmt.Sprintf(apiTokensEP, url.PathEscape(c.config.organizationID)))
	if err != nil {
		return nil, "", err
	}

	reqOpts := []RequestOpt{WithPageSize(maxItemsPerPage)}
	if pageToken != "" {
		reqOpts = append(reqOpts, WithPageToken(pageToken))
	}
	_, err = c.doRequest(ctx,
		http.MethodGet,
		requestURL,
		&tokensResponse,
		nil,
		reqOpts...,
	)
	if err != nil {
		return nil, "", err
	}

	nextPageToken := tokensResponse.Links.Next

	return tokensResponse.Data, nextPageToken, nil
}

func (c *AtlassianClient) ListWorkspaces(ctx context.Context, pageToken string) ([]Workspace, string, error) {
	var workspacesResponse WorkspaceResponse
	requestURL, err := url.JoinPath(c.getBaseURL(), fmt.Sprintf(workspacesEP, url.PathEscape(c.config.organizationID)))
	if err != nil {
		return nil, "", err
	}

	// Pagination for this endpoint must be handled by sending the data in a json body instead of a query param.
	// If you sent the 'cursor' field, no others field can be provided, so the limit cannot be specified and should leave the API use the default value.
	body := struct {
		Cursor string `json:"cursor,omitempty"`
	}{
		Cursor: pageToken,
	}
	_, err = c.doRequest(ctx,
		http.MethodPost,
		requestURL,
		&workspacesResponse,
		body,
	)
	if err != nil {
		return nil, "", err
	}

	nextPageToken := workspacesResponse.Links.Next
	return workspacesResponse.Data, nextPageToken, nil
}

func (c *AtlassianClient) ListGroups(ctx context.Context, pageToken string) ([]Group, string, error) {
	var groupsResponse GroupResponse
	requestURL, err := url.JoinPath(c.getBaseURL(), fmt.Sprintf(groupsEP, url.PathEscape(c.config.organizationID)))
	if err != nil {
		return nil, "", err
	}

	reqOpts := []RequestOpt{WithPageSize(maxItemsPerPage)}
	if pageToken != "" {
		reqOpts = append(reqOpts, WithPageToken(pageToken))
	}
	_, err = c.doRequest(ctx,
		http.MethodGet,
		requestURL,
		&groupsResponse,
		nil,
		reqOpts...,
	)
	if err != nil {
		return nil, "", err
	}

	nextPageToken := groupsResponse.Links.Next

	return groupsResponse.Data, nextPageToken, nil
}

func (c *AtlassianClient) GetUserRoleAssignments(ctx context.Context, pageToken, userID string) ([]RoleAssignment, string, error) {
	var roleAssignmentsResponse RoleAssignmentsResponse

	requestURL, err := url.JoinPath(c.getBaseURL(), fmt.Sprintf(usersRoleAssignmentEP, url.PathEscape(c.config.organizationID), url.PathEscape(userID)))
	if err != nil {
		return nil, "", err
	}

	reqOpts := []RequestOpt{WithPageSize(maxItemsPerPage)}
	if pageToken != "" {
		reqOpts = append(reqOpts, WithPageToken(pageToken))
	}
	_, err = c.doRequest(ctx,
		http.MethodGet,
		requestURL,
		&roleAssignmentsResponse,
		nil,
		reqOpts...,
	)
	if err != nil {
		return nil, "", err
	}

	nextPageToken := roleAssignmentsResponse.Links.Next
	return roleAssignmentsResponse.Data, nextPageToken, nil
}

func (c *AtlassianClient) GetGroupRoleAssignments(ctx context.Context, pageToken, groupID string) ([]RoleAssignment, string, error) {
	var roleAssignmentsResponse RoleAssignmentsResponse

	requestURL, err := url.JoinPath(c.getBaseURL(), fmt.Sprintf(groupsRoleAssignmentEP, url.PathEscape(c.config.organizationID), url.PathEscape(groupID)))
	if err != nil {
		return nil, "", err
	}

	reqOpts := []RequestOpt{WithPageSize(maxItemsPerPage)}
	if pageToken != "" {
		reqOpts = append(reqOpts, WithPageToken(pageToken))
	}
	_, err = c.doRequest(ctx,
		http.MethodGet,
		requestURL,
		&roleAssignmentsResponse,
		nil,
		reqOpts...,
	)
	if err != nil {
		return nil, "", err
	}

	nextPageToken := roleAssignmentsResponse.Links.Next
	return roleAssignmentsResponse.Data, nextPageToken, nil
}

// GetGroupDirectoryID resolves a group's directory id. GET v2/orgs/{org}/directories/-/groups/{groupID}. Requires read:groups:admin.
func (c *AtlassianClient) GetGroupDirectoryID(ctx context.Context, groupID string) (string, error) {
	var groupResponse GroupDetailResponse
	requestURL, err := url.JoinPath(c.getBaseURL(), fmt.Sprintf(groupDetailEP, url.PathEscape(c.config.organizationID), url.PathEscape(groupID)))
	if err != nil {
		return "", err
	}

	_, err = c.doRequest(ctx,
		http.MethodGet,
		requestURL,
		&groupResponse,
		nil,
	)
	if err != nil {
		return "", err
	}

	if groupResponse.Data.DirectoryId == "" {
		return "", status.Error(codes.Internal, "group detail response missing directory id")
	}

	return groupResponse.Data.DirectoryId, nil
}

// AddUserToGroup adds a user to a group. POST v2/orgs/{org}/directories/{directoryID}/groups/{groupID}/memberships. Requires an unscoped organization API key.
func (c *AtlassianClient) AddUserToGroup(ctx context.Context, directoryID, groupID, accountID string) error {
	requestBody := struct {
		AccountId string `json:"accountId"`
	}{
		AccountId: accountID,
	}

	requestURL, err := url.JoinPath(c.getBaseURL(), fmt.Sprintf(groupMembershipsEP, url.PathEscape(c.config.organizationID), url.PathEscape(directoryID), url.PathEscape(groupID)))
	if err != nil {
		return err
	}

	_, err = c.doRequest(ctx,
		http.MethodPost,
		requestURL,
		nil,
		requestBody,
	)
	if err != nil {
		return err
	}

	return nil
}

// RemoveUserFromGroup removes a user from a group. DELETE v2/orgs/{org}/directories/{directoryID}/groups/{groupID}/memberships/{accountID}. Requires an unscoped organization API key.
func (c *AtlassianClient) RemoveUserFromGroup(ctx context.Context, directoryID, groupID, accountID string) error {
	requestURL, err := url.JoinPath(c.getBaseURL(), fmt.Sprintf(groupMembershipEP,
		url.PathEscape(c.config.organizationID), url.PathEscape(directoryID), url.PathEscape(groupID), url.PathEscape(accountID)))
	if err != nil {
		return err
	}

	_, err = c.doRequest(ctx,
		http.MethodDelete,
		requestURL,
		nil,
		nil,
	)
	if err != nil {
		return err
	}

	return nil
}

func (c *AtlassianClient) AssignRoleToUser(ctx context.Context, userID, workspaceID, roleID string) error {
	requestBody := RoleAssignmentBody{
		Role:     roleID,
		Resource: workspaceID,
	}

	requestURL, err := url.JoinPath(c.getBaseURL(), fmt.Sprintf(userAssignRolesEP, url.PathEscape(c.config.organizationID), url.PathEscape(userID)))
	if err != nil {
		return err
	}

	_, err = c.doRequest(ctx,
		http.MethodPost,
		requestURL,
		nil,
		requestBody,
	)
	if err != nil {
		return err
	}

	return nil
}

func (c *AtlassianClient) RevokeRoleFromUser(ctx context.Context, userID, workspaceID, roleID string) error {
	requestBody := RoleAssignmentBody{
		Role:     roleID,
		Resource: workspaceID,
	}

	requestURL, err := url.JoinPath(c.getBaseURL(), fmt.Sprintf(userRevokeRolesEP, url.PathEscape(c.config.organizationID), url.PathEscape(userID)))
	if err != nil {
		return err
	}

	_, err = c.doRequest(ctx,
		http.MethodPost,
		requestURL,
		nil,
		requestBody,
	)
	if err != nil {
		return err
	}

	return nil
}

// ListUserDirectoryIDs returns the IDs of every directory in the organization the user belongs to.
// It collects all pages because this backs a single action call, not a sync.
// https://developer.atlassian.com/cloud/admin/organization/rest/api-group-directory/#api-v2-orgs-orgid-directories-get
func (c *AtlassianClient) ListUserDirectoryIDs(ctx context.Context, accountID string) ([]string, error) {
	requestURL, err := url.JoinPath(c.getBaseURL(), fmt.Sprintf(directoriesEP, url.PathEscape(c.config.organizationID)))
	if err != nil {
		return nil, err
	}

	var directoryIDs []string
	seenCursors := make(map[string]struct{})
	cursor := ""
	for {
		var directoriesResponse DirectoriesResponse
		reqOpts := []RequestOpt{
			WithPageSize(maxItemsPerPage),
			WithQueryParam("accountId", accountID),
		}
		if cursor != "" {
			reqOpts = append(reqOpts, WithPageToken(cursor))
		}
		_, err = c.doRequest(ctx,
			http.MethodGet,
			requestURL,
			&directoriesResponse,
			nil,
			reqOpts...,
		)
		if err != nil {
			return nil, err
		}

		for _, directory := range directoriesResponse.Data {
			if directory.DirectoryId != "" {
				directoryIDs = append(directoryIDs, directory.DirectoryId)
			}
		}

		cursor = directoriesResponse.Links.Next
		if cursor == "" {
			return directoryIDs, nil
		}
		if _, seen := seenCursors[cursor]; seen {
			return nil, status.Errorf(codes.Internal, "directories response repeated cursor %q", cursor)
		}
		seenCursors[cursor] = struct{}{}
	}
}

// SuspendUserInDirectory suspends a user's access in one directory. Requires an unscoped organization API key,
// since Atlassian offers no scope for this endpoint.
// https://developer.atlassian.com/cloud/admin/organization/rest/api-group-users/#api-v2-orgs-orgid-directories-directoryid-users-accountid-suspend-post
func (c *AtlassianClient) SuspendUserInDirectory(ctx context.Context, directoryID, accountID string) error {
	return c.postUserDirectoryAction(ctx, userDirectorySuspendEP, directoryID, accountID)
}

// RestoreUserInDirectory restores a user's access in one directory. Requires an unscoped organization API key,
// since Atlassian offers no scope for this endpoint.
// https://developer.atlassian.com/cloud/admin/organization/rest/api-group-users/#api-v2-orgs-orgid-directories-directoryid-users-accountid-restore-post
func (c *AtlassianClient) RestoreUserInDirectory(ctx context.Context, directoryID, accountID string) error {
	return c.postUserDirectoryAction(ctx, userDirectoryRestoreEP, directoryID, accountID)
}

// postUserDirectoryAction sends a body-less POST to v2/orgs/{org}/directories/{directoryID}/users/{accountID}/{suspend|restore}.
func (c *AtlassianClient) postUserDirectoryAction(ctx context.Context, endpoint, directoryID, accountID string) error {
	requestURL, err := url.JoinPath(c.getBaseURL(), fmt.Sprintf(endpoint,
		url.PathEscape(c.config.organizationID), url.PathEscape(directoryID), url.PathEscape(accountID)))
	if err != nil {
		return err
	}

	_, err = c.doRequest(ctx,
		http.MethodPost,
		requestURL,
		nil,
		nil,
	)
	return err
}

// DeleteUser deletes a managed account after Atlassian's 14-day grace period. Requires an unscoped organization API key.
// https://developer.atlassian.com/cloud/admin/user-management/rest/api-group-lifecycle/#api-users-account-id-manage-lifecycle-delete-post
func (c *AtlassianClient) DeleteUser(ctx context.Context, accountID string) error {
	requestURL, err := url.JoinPath(c.getLifecycleBaseURL(), fmt.Sprintf(userLifecycleDeleteEP, url.PathEscape(accountID)))
	if err != nil {
		return err
	}

	_, err = c.doRequest(ctx,
		http.MethodPost,
		requestURL,
		nil,
		nil,
	)
	if err != nil {
		return err
	}

	return nil
}

// DeactivateUser deactivates a managed account across every Atlassian organization. Requires an unscoped organization API key.
// https://developer.atlassian.com/cloud/admin/user-management/rest/api-group-lifecycle/#api-users-account-id-manage-lifecycle-disable-post
func (c *AtlassianClient) DeactivateUser(ctx context.Context, accountID string) error {
	requestURL, err := url.JoinPath(c.getLifecycleBaseURL(), fmt.Sprintf(userLifecycleDisableEP, url.PathEscape(accountID)))
	if err != nil {
		return err
	}

	// The disable and enable endpoints reject a POST without a JSON content type, even with no fields to send.
	_, err = c.doRequest(ctx,
		http.MethodPost,
		requestURL,
		nil,
		struct{}{},
	)
	if err != nil {
		return err
	}

	return nil
}

// ActivateUser reactivates a deactivated managed account across every Atlassian organization. Requires an unscoped organization API key.
// https://developer.atlassian.com/cloud/admin/user-management/rest/api-group-lifecycle/#api-users-account-id-manage-lifecycle-enable-post
func (c *AtlassianClient) ActivateUser(ctx context.Context, accountID string) error {
	requestURL, err := url.JoinPath(c.getLifecycleBaseURL(), fmt.Sprintf(userLifecycleEnableEP, url.PathEscape(accountID)))
	if err != nil {
		return err
	}

	// The disable and enable endpoints reject a POST without a JSON content type, even with no fields to send.
	_, err = c.doRequest(ctx,
		http.MethodPost,
		requestURL,
		nil,
		struct{}{},
	)
	if err != nil {
		return err
	}

	return nil
}

// https://developer.atlassian.com/cloud/admin/user-provisioning/rest/api-group-users/#api-scim-directory-directoryid-users-post
func (c *AtlassianClient) CreateUser(ctx context.Context, request SCIMCreateUserRequest) (*SCIMUserResponse, error) {
	if c.config.scimBaseUrl == "" {
		return nil, fmt.Errorf("SCIM base URL is not configured")
	}
	if c.config.scimToken == "" {
		return nil, fmt.Errorf("SCIM token is not configured")
	}

	var scimResponse SCIMUserResponse
	requestURL := fmt.Sprintf("%s/Users", strings.TrimSuffix(c.config.scimBaseUrl, "/"))

	_, err := c.doRequest(ctx,
		http.MethodPost,
		requestURL,
		&scimResponse,
		request,
		WithToken(c.config.scimToken),
	)
	if err != nil {
		return nil, err
	}

	return &scimResponse, nil
}

type requestConfig struct {
	token string
	url   *url.URL
}

type RequestOpt func(*requestConfig)

func WithToken(token string) RequestOpt {
	return func(rc *requestConfig) {
		rc.token = token
	}
}

func (c *AtlassianClient) doRequest(
	ctx context.Context,
	method string,
	endpointUrl string,
	res interface{},
	body interface{},
	requestOpts ...RequestOpt,
) (http.Header, error) {
	var (
		resp   *http.Response
		apiErr APIError
		err    error
	)

	urlAddress, err := url.Parse(endpointUrl)
	if err != nil {
		return nil, err
	}

	reqConfig := &requestConfig{
		token: c.config.accessToken,
		url:   urlAddress,
	}

	for _, opt := range requestOpts {
		opt(reqConfig)
	}

	reqOptions := []uhttp.RequestOption{
		uhttp.WithBearerToken(reqConfig.token),
	}
	if body != nil {
		reqOptions = append(reqOptions, uhttp.WithJSONBody(body))
	}

	req, err := c.wrapper.NewRequest(
		ctx,
		method,
		reqConfig.url,
		reqOptions...,
	)
	if err != nil {
		return nil, err
	}

	switch method {
	case http.MethodGet, http.MethodPut, http.MethodPost:
		doOptions := []uhttp.DoOption{
			uhttp.WithErrorResponse(&apiErr),
		}

		if res != nil {
			doOptions = append(doOptions, uhttp.WithResponse(&res))
		}
		resp, err = c.wrapper.Do(req, doOptions...)
		if resp != nil {
			defer resp.Body.Close()
		}

	case http.MethodDelete:
		resp, err = c.wrapper.Do(req, uhttp.WithErrorResponse(&apiErr))
		if resp != nil {
			defer resp.Body.Close()
		}
	}
	if err != nil {
		return nil, &requestError{grpcErr: err, body: apiErr}
	}

	return resp.Header, nil
}

func New(ctx context.Context, clientOptions ...Option) (*AtlassianClient, error) {
	httpClient, err := uhttp.NewClient(ctx, uhttp.WithLogger(true, ctxzap.Extract(ctx)))
	if err != nil {
		return nil, err
	}

	cli, err := uhttp.NewBaseHttpClientWithContext(context.Background(), httpClient)
	if err != nil {
		return nil, err
	}

	client := AtlassianClient{
		wrapper: cli,
	}

	for _, opt := range clientOptions {
		opt(&client)
	}

	return &client, nil
}
