package test

import (
	"log"
	"net/http"
	"os"

	"github.com/conductorone/baton-atlassian/pkg/client"
	"github.com/conductorone/baton-sdk/pkg/uhttp"
)

var (
	UserIDs        = []string{"ea960e6c-f613-4bed-8852-ab012603915b", "8b21d0aa-39a4-4c09-86d2-d29dff8d261f"}
	OrganizationID = "organizationTest"
)

// Custom RoundTripper for testing.
type TestRoundTripper struct {
	response *http.Response
	err      error
}

type MockRoundTripper struct {
	Response  *http.Response
	Err       error
	roundTrip func(*http.Request) (*http.Response, error)
}

func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTrip(req)
}

func (m *MockRoundTripper) SetRoundTrip(roundTrip func(*http.Request) (*http.Response, error)) {
	m.roundTrip = roundTrip
}

func (t *TestRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return t.response, t.err
}

// Helper function to create a test client with custom transport.
func NewTestClient(response *http.Response, err error) *client.AtlassianClient {
	transport := &TestRoundTripper{response: response, err: err}
	httpClient := &http.Client{Transport: transport}
	baseHttpClient := uhttp.NewBaseHttpClient(httpClient)
	return client.NewClient("", "", OrganizationID, "", baseHttpClient)
}

func ReadFile(fileName string) string {
	data, err := os.ReadFile("../../test/mockResponses/" + fileName)
	if err != nil {
		log.Fatal(err)
	}

	return string(data)
}
