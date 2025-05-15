package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
)

const (
	baseURL = "https://api.atlassian.com/admin/v2/orgs"

	usersEP          = "%s/directories/-/users"
	workspacesEP     = "%s/workspaces"
	roleAssignmentEP = "%s/directories/-/users/%s/role-assignments"
)

type AtlassianClient struct {
	wrapper *uhttp.BaseHttpClient
	config  Config
}

type Config struct {
	accessToken    string
	organizationID string
}

type Option func(*AtlassianClient)

func WithAccessToken(accessToken string) Option {
	return func(c *AtlassianClient) {
		c.config.accessToken = accessToken
	}
}

func WithOrganizationID(orgID string) Option {
	return func(c *AtlassianClient) {
		c.config.organizationID = orgID
	}
}

func (c *AtlassianClient) ListUsers(ctx context.Context, pageToken string) (*[]User, string, error) {
	var usersResponse UserResponse
	requestURL, err := url.JoinPath(baseURL, fmt.Sprintf(usersEP, c.config.organizationID))
	if err != nil {
		return nil, "", err
	}

	reqOpts := []ReqOpt{WithPageSize(maxItemsPerPage)}
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

	return &usersResponse.Data, nextPageToken, nil
}

func (c *AtlassianClient) ListWorkspaces(ctx context.Context, pageToken string) (*[]Workspace, string, error) {
	var workspacesResponse WorkspaceResponse
	requestURL, err := url.JoinPath(baseURL, fmt.Sprintf(workspacesEP, c.config.organizationID))
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
	return &workspacesResponse.Data, nextPageToken, nil
}

func (c *AtlassianClient) GetUserRoleAssignments(ctx context.Context, pageToken, userID string) (*[]RoleAssignment, string, error) {
	var roleAssignmentsResponse RoleAssignmentsResponse

	requestURL, err := url.JoinPath(baseURL, fmt.Sprintf(roleAssignmentEP, c.config.organizationID, userID))
	if err != nil {
		return nil, "", err
	}

	reqOpts := []ReqOpt{WithPageSize(1)}
	if pageToken != "" {
		reqOpts = append(reqOpts, WithPageToken(pageToken))
	}

	_, err = c.doRequest(ctx,
		http.MethodGet,
		requestURL,
		&roleAssignmentsResponse,
		nil,
	)
	if err != nil {
		return nil, "", err
	}

	nextPageToken := roleAssignmentsResponse.Links.Next
	return &roleAssignmentsResponse.Data, nextPageToken, nil
}

func (c *AtlassianClient) doRequest(
	ctx context.Context,
	method string,
	endpointUrl string,
	res interface{},
	body interface{},
	reqOpts ...ReqOpt,
) (http.Header, error) {
	var (
		resp *http.Response
		err  error
	)

	urlAddress, err := url.Parse(endpointUrl)
	if err != nil {
		return nil, err
	}

	for _, o := range reqOpts {
		o(urlAddress)
	}

	reqOptions := []uhttp.RequestOption{uhttp.WithBearerToken(c.config.accessToken)}
	if body != nil {
		reqOptions = append(reqOptions, uhttp.WithJSONBody(body))
	}

	req, err := c.wrapper.NewRequest(
		ctx,
		method,
		urlAddress,
		reqOptions...,
	)
	if err != nil {
		return nil, err
	}

	switch method {
	case http.MethodGet, http.MethodPut, http.MethodPost:
		doOptions := []uhttp.DoOption{}
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
