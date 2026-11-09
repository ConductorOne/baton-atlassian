package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeactivateUserEscapesAccountID(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	ctx := context.Background()
	atlassianClient, err := New(ctx, WithAccessToken("token"), WithOrganizationID("org"), WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}

	if err := atlassianClient.DeactivateUser(ctx, "a/b"); err != nil {
		t.Fatal(err)
	}
	if want := "/users/a%2Fb/manage/lifecycle/disable"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
}
