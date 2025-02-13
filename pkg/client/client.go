package client

import (
	"context"
	encoding "encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/ratelimit"
	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
)

type AtlassianClient struct {
	wrapper        *uhttp.BaseHttpClient
	UserEmail      string
	ApiToken       string
	OrganizationID string
	SiteID         string
}

type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

type GraphQLResponse struct {
	Data   map[string]interface{} `json:"data"`
	Errors interface{}            `json:"errors"`
}

const (
	baseUrl = "https://team.atlassian.com/gateway/api/graphql"
)

func New(ctx context.Context, atlassianClient *AtlassianClient) (*AtlassianClient, error) {
	var (
		userEmail      = atlassianClient.UserEmail
		apiToken       = atlassianClient.ApiToken
		organizationID = atlassianClient.OrganizationID
		siteID         = atlassianClient.SiteID
	)

	httpClient, err := uhttp.NewClient(ctx, uhttp.WithLogger(true, ctxzap.Extract(ctx)))
	if err != nil {
		return nil, err
	}

	cli, err := uhttp.NewBaseHttpClientWithContext(context.Background(), httpClient)
	if err != nil {
		return nil, err
	}

	client := AtlassianClient{
		wrapper:        cli,
		UserEmail:      userEmail,
		ApiToken:       apiToken,
		OrganizationID: organizationID,
		SiteID:         siteID,
	}

	return &client, nil
}

func NewClient(userEmail, apiToken, organizationID, siteID string, httpClient ...*uhttp.BaseHttpClient) *AtlassianClient {
	var wrapper = &uhttp.BaseHttpClient{}
	if httpClient != nil || len(httpClient) != 0 {
		wrapper = httpClient[0]
	}
	return &AtlassianClient{
		wrapper:        wrapper,
		UserEmail:      userEmail,
		ApiToken:       apiToken,
		OrganizationID: organizationID,
		SiteID:         siteID,
	}
}

func (c *AtlassianClient) ListTeams(ctx context.Context, options PageOptions) ([]TeamEdge, string, annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)
	var res TeamQuery
	var teams []TeamEdge
	var annotation annotations.Annotations
	nextPageToken := ""

	queryVariables := map[string]interface{}{
		"organizationId": c.OrganizationID,
		"siteId":         "None",
		"firstTeam":      getPageSize(options.PageSize),
	}
	if options.PageToken != "" {
		queryVariables["afterTeam"] = options.PageToken
	}
	body := parseGraphQLQuery("Teams.query.graphql", queryVariables)
	_, err := c.getResourcesFromAPI(ctx, &res, &body)
	if err != nil {
		l.Error(fmt.Sprintf("Error getting resources: %s", err))
		return nil, "", nil, err
	}

	for _, teamEdge := range res.Team.TeamSearch.Edges {
		teamEdgeCopy := teamEdge
		team := teamEdgeCopy.Node.Team
		subQueryNextPageToken := ""
		subQueryVariables := queryVariables

		var allMembers []MemberEdge
		var lastPageInfo PageInfo

		for {
			subQueryVariables["firstMember"] = getPageSize(options.PageSize)
			if subQueryNextPageToken != "" {
				subQueryVariables["afterMember"] = subQueryNextPageToken
			}

			membersBody := parseGraphQLQuery("Teams.query.graphql", subQueryVariables)

			var memberResp TeamQuery
			_, err := c.getResourcesFromAPI(ctx, &memberResp, &membersBody)
			if err != nil {
				l.Error(fmt.Sprintf("Error getting resources: %s", err))
				return nil, "", nil, err
			}

			members := memberResp.Team.TeamSearch.Edges
			for _, edge := range members {
				if edge.Node.Team.ID == team.ID {
					allMembers = append(allMembers, edge.Node.Team.Members.Edges...)
					lastPageInfo = edge.Node.Team.Members.PageInfo
				}
			}

			if lastPageInfo.hasNextPage {
				subQueryNextPageToken = lastPageInfo.endCursor
			} else {
				break
			}
		}

		team.Members.Edges = allMembers
		team.Members.PageInfo = lastPageInfo
		teamEdgeCopy.Node.Team = team
		teams = append(teams, teamEdgeCopy)
	}

	if res.Team.TeamSearch.PageInfo.hasNextPage {
		nextPageToken = res.Team.TeamSearch.PageInfo.endCursor
	}

	return teams, nextPageToken, annotation, nil
}

func (c *AtlassianClient) getResourcesFromAPI(
	ctx context.Context,
	resources any,
	body any,
) (annotations.Annotations, error) {
	var res GraphQLResponse
	_, annotation, err := c.doRequest(ctx, &res, &body)

	if err != nil || res.Errors != nil {
		return nil, err
	}

	jsonBytes, err := json.Marshal(res.Data)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(jsonBytes, &resources)
	if err != nil {
		return nil, err
	}

	return annotation, nil
}

func (c *AtlassianClient) doRequest(
	ctx context.Context,
	res interface{},
	body interface{},
) (http.Header, annotations.Annotations, error) {
	var (
		resp *http.Response
		err  error
	)

	urlAddress, err := url.Parse(baseUrl)
	if err != nil {
		return nil, nil, err
	}

	authorizationToken := encoding.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", c.UserEmail, c.ApiToken)))

	req, err := c.wrapper.NewRequest(
		ctx,
		http.MethodPost,
		urlAddress,
		uhttp.WithContentTypeJSONHeader(),
		uhttp.WithAccept("*/*"),
		uhttp.WithHeader("Authorization", "Basic "+authorizationToken),
		uhttp.WithJSONBody(body),
	)

	if err != nil {
		return nil, nil, err
	}

	var doOptions []uhttp.DoOption
	if res != nil {
		doOptions = append(doOptions, uhttp.WithResponse(&res))
	}
	resp, err = c.wrapper.Do(req, doOptions...)
	if resp != nil {
		defer resp.Body.Close()
	}

	if err != nil {
		return nil, nil, err
	}

	annotation := annotations.Annotations{}
	if resp != nil {
		if desc, err := ratelimit.ExtractRateLimitData(resp.StatusCode, &resp.Header); err == nil {
			annotation.WithRateLimiting(desc)
		} else {
			return nil, annotation, err
		}

		return resp.Header, annotation, nil
	}

	return nil, nil, err
}

func parseGraphQLQuery(query string, queryVariables map[string]interface{}) interface{} {
	dirName := GetEnv("GRAPHQL_DIR", "pkg/graphql/")
	queryBytes, err := os.ReadFile(dirName + query)
	if err != nil {
		log.Fatalf("Error reading file query: %v", err)
	}

	requestBody := GraphQLRequest{
		Query:     strings.TrimSpace(string(queryBytes)),
		Variables: queryVariables,
	}

	return requestBody
}
