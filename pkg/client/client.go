package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
)

const (
	baseURL = "https://api.atlassian.com/admin"

	usersEP                = "v2/orgs/%s/directories/-/users"
	workspacesEP           = "v2/orgs/%s/workspaces"
	groupsEP               = "v2/orgs/%s/directories/-/groups"
	usersRoleAssignmentEP  = "v2/orgs/%s/directories/-/users/%s/role-assignments"
	groupsRoleAssignmentEP = "v2/orgs/%s/directories/-/groups/%s/role-assignments"

	userAssignRolesEP = "v1/orgs/%s/users/%s/roles/assign"
	userRevokeRolesEP = "v1/orgs/%s/users/%s/roles/revoke"
	// Note: suspend/restore access endpoints cannot be used on organization administrators (returns 400 error).
	userSuspendAccessEP = "v1/orgs/%s/directory/users/%s/suspend-access"
	userRestoreAccessEP = "v1/orgs/%s/directory/users/%s/restore-access"

	organizationEP = "v1/orgs/%s"
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

// GetOrganizationID returns the organization ID from the client configuration.
func (c *AtlassianClient) GetOrganizationID() string {
	return c.config.organizationID
}

// GetOrganization returns the organization details including its name.
func (c *AtlassianClient) GetOrganization(ctx context.Context) (*Organization, error) {
	var orgResponse OrganizationResponse
	requestURL, err := url.JoinPath(c.getBaseURL(), fmt.Sprintf(organizationEP, c.config.organizationID))
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

// GetGroupMembers returns a list of users that are members of a specific group.
func (c *AtlassianClient) GetGroupMembers(ctx context.Context, pageToken, groupID string) ([]User, string, error) {
	var usersResponse UserResponse
	requestURL, err := url.JoinPath(c.getBaseURL(), fmt.Sprintf(usersEP, c.config.organizationID))
	if err != nil {
		return nil, "", err
	}

	reqOpts := []RequestOpt{
		WithPageSize(maxItemsPerPage),
		WithQueryParam("groupIds", groupID),
	}
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

func (c *AtlassianClient) ListUsers(ctx context.Context, pageToken string) ([]User, string, error) {
	var usersResponse UserResponse
	requestURL, err := url.JoinPath(c.getBaseURL(), fmt.Sprintf(usersEP, c.config.organizationID))
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

func (c *AtlassianClient) ListWorkspaces(ctx context.Context, pageToken string) ([]Workspace, string, error) {
	var workspacesResponse WorkspaceResponse
	requestURL, err := url.JoinPath(c.getBaseURL(), fmt.Sprintf(workspacesEP, c.config.organizationID))
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
	requestURL, err := url.JoinPath(c.getBaseURL(), fmt.Sprintf(groupsEP, c.config.organizationID))
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

	requestURL, err := url.JoinPath(c.getBaseURL(), fmt.Sprintf(usersRoleAssignmentEP, c.config.organizationID, userID))
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

	requestURL, err := url.JoinPath(c.getBaseURL(), fmt.Sprintf(groupsRoleAssignmentEP, c.config.organizationID, groupID))
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

func (c *AtlassianClient) AssignRoleToUser(ctx context.Context, userID, workspaceID, roleID string) error {
	requestBody := RoleAssignmentBody{
		Role:     roleID,
		Resource: workspaceID,
	}

	requestURL, err := url.JoinPath(c.getBaseURL(), fmt.Sprintf(userAssignRolesEP, c.config.organizationID, userID))
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

	requestURL, err := url.JoinPath(c.getBaseURL(), fmt.Sprintf(userRevokeRolesEP, c.config.organizationID, userID))
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

// https://developer.atlassian.com/cloud/admin/organization/rest/api-group-directory/#api-v1-orgs-orgid-directory-users-accountid-suspend-access-post
func (c *AtlassianClient) DisableUser(ctx context.Context, accountID string) error {
	requestURL, err := url.JoinPath(c.getBaseURL(), fmt.Sprintf(userSuspendAccessEP, c.config.organizationID, accountID))
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

// https://developer.atlassian.com/cloud/admin/organization/rest/api-group-directory/#api-v1-orgs-orgid-directory-users-accountid-restore-access-post
func (c *AtlassianClient) EnableUser(ctx context.Context, accountID string) error {
	requestURL, err := url.JoinPath(c.getBaseURL(), fmt.Sprintf(userRestoreAccessEP, c.config.organizationID, accountID))
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
		resp, err = c.wrapper.Do(req)
		if resp != nil {
			defer resp.Body.Close()
		}
	}
	if err != nil {
		return nil, err
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
