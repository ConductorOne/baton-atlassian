package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
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

const (
	baseURL = "https://api.atlassian.com/admin/v2/orgs"
	usersEP = "%s/directories/-/users"
)

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

func (c *AtlassianClient) ListUsers(ctx context.Context, pageToken string) (*[]User, string, error) {
	var usersResponse UserResponse
	requestURL, err := url.JoinPath(baseURL, fmt.Sprintf(usersEP, c.config.organizationID))
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

	req, err := c.wrapper.NewRequest(
		ctx,
		method,
		urlAddress,
		uhttp.WithBearerToken(c.config.accessToken),
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
