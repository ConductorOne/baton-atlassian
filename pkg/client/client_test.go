package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

func TestListUserDirectoryIDsStopsOnRepeatedCursor(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests > 3 {
			t.Fatalf("pagination did not stop after %d requests", requests)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"directoryId":"d1"}],"links":{"next":"same"}}`))
	}))
	defer srv.Close()

	ctx := context.Background()
	atlassianClient, err := New(ctx, WithAccessToken("token"), WithOrganizationID("org"), WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := atlassianClient.ListUserDirectoryIDs(ctx, "acc"); status.Code(err) != codes.Internal {
		t.Errorf("status code = %v, want %v (err: %v)", status.Code(err), codes.Internal, err)
	}
}

func TestDirectoryUserCallsEscapePathSegments(t *testing.T) {
	cases := []struct {
		name     string
		call     func(*AtlassianClient, context.Context, string, string) error
		wantPath string
	}{
		{name: "suspend", call: (*AtlassianClient).SuspendUserInDirectory, wantPath: "/v2/orgs/org/directories/d%2F1/users/a%2Fb/suspend"},
		{name: "restore", call: (*AtlassianClient).RestoreUserInDirectory, wantPath: "/v2/orgs/org/directories/d%2F1/users/a%2Fb/restore"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod, gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath = r.Method, r.URL.EscapedPath()
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()

			ctx := context.Background()
			atlassianClient, err := New(ctx, WithAccessToken("token"), WithOrganizationID("org"), WithBaseURL(srv.URL))
			if err != nil {
				t.Fatal(err)
			}

			if err := tc.call(atlassianClient, ctx, "d/1", "a/b"); err != nil {
				t.Fatal(err)
			}
			if gotMethod != http.MethodPost || gotPath != tc.wantPath {
				t.Errorf("request = %s %s, want POST %s", gotMethod, gotPath, tc.wantPath)
			}
		})
	}
}
