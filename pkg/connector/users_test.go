package connector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/conductorone/baton-atlassian/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const testAccountID = "712020:e19eff6d-a9d7-49e0-b743-044f410d5b00"

// Bodies are the lifecycle API's live responses (2026-10-05).
func TestUserDelete(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		body     string
		wantCode codes.Code
	}{
		{name: "deleted", status: http.StatusOK, body: `{"message":"AccountId x has been deleted."}`, wantCode: codes.OK},
		{
			name:     "account already gone is success",
			status:   http.StatusNotFound,
			body:     `{"key":"accountNotFound","errorKey":"account-not-found","errorDetail":"Error: User account not found"}`,
			wantCode: codes.OK,
		},
		{name: "generic notFound key propagates", status: http.StatusNotFound, body: `{"key":"notFound"}`, wantCode: codes.NotFound},
		{
			name:     "unmanaged account propagates",
			status:   http.StatusForbidden,
			body:     `{"key":"forbidden","errorKey":"forbidden","errorDetail":"Error: Caller must be a verified org admin of targeted account"}`,
			wantCode: codes.PermissionDenied,
		},
		{name: "conflict propagates", status: http.StatusConflict, body: `{}`, wantCode: codes.AlreadyExists},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod, gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath = r.Method, r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			ctx := context.Background()
			c, err := client.New(ctx, client.WithAccessToken("token"), client.WithOrganizationID("org"), client.WithBaseURL(srv.URL))
			if err != nil {
				t.Fatal(err)
			}

			_, err = newUserBuilder(c).Delete(ctx, &v2.ResourceId{ResourceType: userResourceType.Id, Resource: testAccountID})
			if got := status.Code(err); got != tc.wantCode {
				t.Fatalf("code = %v, want %v (err: %v)", got, tc.wantCode, err)
			}
			if gotMethod != http.MethodPost || gotPath != "/users/"+testAccountID+"/manage/lifecycle/delete" {
				t.Errorf("request = %s %s", gotMethod, gotPath)
			}
		})
	}
}

func TestUserDeleteEmptyIDNeverCallsAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	ctx := context.Background()
	c, err := client.New(ctx, client.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newUserBuilder(c).Delete(ctx, &v2.ResourceId{ResourceType: userResourceType.Id}); err == nil {
		t.Fatal("expected error for empty account ID")
	}
}

// The SCIM id differs from the Atlassian accountId that sync uses as the resource ID.
func TestSCIMUserToUserUsesAtlassianAccountID(t *testing.T) {
	var resp client.SCIMUserResponse
	body := `{"id":"scim-uuid","urn:scim:schemas:extension:atlassian-external:1.0":{"atlassianAccountId":"` + testAccountID + `"}}`
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatal(err)
	}
	if got := scimUserToUser(&resp).AccountId; got != testAccountID {
		t.Errorf("AccountId = %q, want %q", got, testAccountID)
	}
}
