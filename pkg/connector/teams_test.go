package connector

import (
	"context"
	encoding "encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/conductorone/baton-atlassian/pkg/client"
	"github.com/conductorone/baton-atlassian/test"
	"github.com/conductorone/baton-sdk/pkg/uhttp"
)

type MemberNode struct {
	Member client.Member `json:"member"`
	Role   string        `json:"role"`
}

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}

func setupTestCase(t *testing.T) func(t *testing.T) {
	err := os.Setenv("GRAPHQL_DIR", "../graphql/")
	if err != nil {
		t.Fatalf("Failed to set GRAPHQL_DIR: %v", err)
	}

	return func(t *testing.T) {
		// Nothing to tear down
	}
}

// Tests that the client can fetch teams and users based on the documented API below.
// https://developer.atlassian.com/platform/atlassian-graphql-api/graphql/#teams_teamSearchV2
func TestAtlassianClient_GetTeamsAndUsers(t *testing.T) {
	teardown := setupTestCase(t)
	defer teardown(t)

	// Create a mock response.
	mockResponseBody, err := ReadFile("Teams.json")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	mockResponse := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(mockResponseBody)),
	}
	mockResponse.Header.Set("Content-Type", "application/json")

	// Create a test client with the mock response.
	testClient := test.NewTestClient(mockResponse, nil)

	// Call GetUsers
	ctx := context.Background()
	result, nextPageToken, _, err := testClient.ListTeams(ctx, client.PageOptions{
		PageSize:  5,
		PageToken: "",
	})

	// Check for errors.
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify the result.
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Check count.
	if len(result) != 2 {
		t.Errorf("Expected Count to be 2, got %d", len(result))
	}

	for index, item := range result {
		team := item.Node.Team
		memberEdges := team.Members.Edges

		expectedTeam := client.Team{
			ID:             fmt.Sprintf("ari:cloud:identity::team/teamTest%d", index+1),
			OrganizationID: test.OrganizationID,
			DisplayName:    fmt.Sprintf("Team %d", index+1),
		}

		if !reflect.DeepEqual(team.ID, expectedTeam.ID) && !reflect.DeepEqual(team.OrganizationID, expectedTeam.OrganizationID) && !reflect.DeepEqual(team.DisplayName, expectedTeam.DisplayName) {
			t.Errorf("Unexpected team: got %+v, want %+v", team, expectedTeam)
		}

		expectedMembers := []client.MemberEdge{
			{
				Node: MemberNode(struct {
					Member client.Member
					Role   string
				}{
					Member: client.Member{
						ID:        fmt.Sprintf("ari:cloud:identity::user/%s", test.UserIDs[0]),
						Name:      "User 1",
						AccountID: test.UserIDs[0],
					},
					Role: "REGULAR",
				}),
			},
		}

		if index == 1 {
			expectedMembers = append(expectedMembers, client.MemberEdge{
				Node: MemberNode(struct {
					Member client.Member
					Role   string
				}{
					Member: client.Member{
						ID:        fmt.Sprintf("ari:cloud:identity::user/%s", test.UserIDs[1]),
						Name:      "User 2",
						AccountID: test.UserIDs[1],
					},
					Role: "ADMIN",
				}),
			})
		}
		if !reflect.DeepEqual(memberEdges, expectedMembers) {
			t.Errorf("Unexpected team members: got %+v, want %+v", memberEdges, expectedMembers)
		}
	}

	// Check next page token.
	if nextPageToken != "" {
		t.Fatal("Expected empty next page token")
	}
}

func TestAtlassianClient_GetTeamsAndUsers_RequestDetails(t *testing.T) {
	teardown := setupTestCase(t)
	defer teardown(t)

	// Create a custom RoundTripper to capture the request.
	var capturedRequest *http.Request
	mockTransport := &test.MockRoundTripper{
		Response: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{}`)),
			Header:     make(http.Header),
		},
		Err: nil,
	}
	mockTransport.Response.Header.Set("Content-Type", "application/json")

	mockRoundTrip := func(req *http.Request) (*http.Response, error) {
		capturedRequest = req
		return mockTransport.Response, mockTransport.Err
	}
	mockTransport.SetRoundTrip(mockRoundTrip)

	// Create a test client with the mock transport.
	httpClient := &http.Client{Transport: mockTransport}
	baseHttpClient := uhttp.NewBaseHttpClient(httpClient)
	userEmailMock := "user@test.com"
	apiTokenMock := "api-token"
	testClient := client.NewClient(userEmailMock, apiTokenMock, test.OrganizationID, "", baseHttpClient)

	// Call GetUsers.
	ctx := context.Background()
	_, _, _, err := testClient.ListTeams(ctx, client.PageOptions{
		PageSize:  5,
		PageToken: "",
	})

	// Check for errors.
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify the request details.
	if capturedRequest == nil {
		t.Fatal("No request was captured")
	}

	// Check URL components.
	expectedURL := "https://team.atlassian.com/gateway/api/graphql"
	if capturedRequest.URL.String() != expectedURL {
		t.Errorf("Expected URL %s, got %s", expectedURL, capturedRequest.URL.String())
	}

	// Check headers.
	expectedAuthToken := encoding.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", userEmailMock, apiTokenMock)))
	expectedHeaders := map[string]string{
		"Accept":        "*/*",
		"Content-Type":  "application/json",
		"Authorization": fmt.Sprintf("Basic %s", expectedAuthToken),
	}

	for key, expectedValue := range expectedHeaders {
		if value := capturedRequest.Header.Get(key); value != expectedValue {
			t.Errorf("Expected header %s to be %s, got %s", key, expectedValue, value)
		}
	}
}
func ReadFile(fileName string) (string, error) {
	data, err := os.ReadFile("../../test/mockResponses/" + fileName)
	if err != nil {
		return "", err
	}

	return string(data), nil
}
