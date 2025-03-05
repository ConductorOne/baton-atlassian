package client

import (
	"testing"
)

func TestTrelloClient_AddCredentials(t *testing.T) {
	mockUserEmail := "user@test.com"
	mockApiToken := "api-token"

	client := NewClient(mockUserEmail, mockApiToken, "", "")

	if client.userEmail != mockUserEmail {
		t.Errorf("Set user email failed. Expected %s, got %s", mockUserEmail, client.userEmail)
	}
	if client.apiToken != mockApiToken {
		t.Errorf("Set API token failed. Expected %s, got %s", mockApiToken, client.apiToken)
	}
}
