package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLifecycleCallsEscapeAccountID(t *testing.T) {
	cases := []struct {
		name     string
		call     func(*AtlassianClient, context.Context, string) error
		wantPath string
	}{
		{name: "deactivate", call: (*AtlassianClient).DeactivateUser, wantPath: "/users/a%2Fb/manage/lifecycle/disable"},
		{name: "activate", call: (*AtlassianClient).ActivateUser, wantPath: "/users/a%2Fb/manage/lifecycle/enable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
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

			if err := tc.call(atlassianClient, ctx, "a/b"); err != nil {
				t.Fatal(err)
			}
			if gotPath != tc.wantPath {
				t.Errorf("path = %q, want %q", gotPath, tc.wantPath)
			}
		})
	}
}
