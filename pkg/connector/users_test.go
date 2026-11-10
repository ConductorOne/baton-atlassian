package connector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/conductorone/baton-atlassian/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
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

func TestCreateAccountKeepsAPIStatusCode(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		body        string
		wantCode    codes.Code
		wantMessage string
	}{
		{
			name:        "existing user",
			status:      http.StatusConflict,
			body:        `{"schemas":["urn:ietf:params:scim:api:messages:2.0:Error"],"status":"409","scimType":"uniqueness","detail":"User already exists"}`,
			wantCode:    codes.AlreadyExists,
			wantMessage: "User already exists",
		},
		{
			name:        "forbidden",
			status:      http.StatusForbidden,
			body:        `{"error":"Invalid token"}`,
			wantCode:    codes.PermissionDenied,
			wantMessage: "Invalid token",
		},
		{
			name:        "unavailable",
			status:      http.StatusServiceUnavailable,
			body:        `{"schemas":["urn:ietf:params:scim:api:messages:2.0:Error"],"status":"503","detail":"Service unavailable"}`,
			wantCode:    codes.Unavailable,
			wantMessage: "Service unavailable",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			ctx := context.Background()
			c, err := client.New(ctx, client.WithScimToken("token"), client.WithScimBaseUrl(srv.URL))
			if err != nil {
				t.Fatal(err)
			}
			profile, err := structpb.NewStruct(map[string]any{"userName": "jdoe", "email": "jdoe@example.com"})
			if err != nil {
				t.Fatal(err)
			}

			_, _, _, err = newUserBuilder(c).CreateAccount(ctx, &v2.AccountInfo{Profile: profile}, nil)
			if got := status.Code(err); got != tc.wantCode {
				t.Fatalf("code = %v, want %v (err: %v)", got, tc.wantCode, err)
			}
			if !strings.Contains(err.Error(), tc.wantMessage) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantMessage)
			}
		})
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

func TestParseIntoUserResourceStatus(t *testing.T) {
	cases := []struct {
		name             string
		status           string
		accountStatus    string
		membershipStatus string
		want             v2.Status_ResourceStatus
	}{
		// Known composite values win, even when the sub-statuses disagree.
		{name: "active", status: statusActive, accountStatus: "active", membershipStatus: "active", want: v2.Status_RESOURCE_STATUS_ENABLED},
		{name: "suspended", status: statusSuspended, accountStatus: "active", membershipStatus: "suspended", want: v2.Status_RESOURCE_STATUS_DISABLED},
		{name: "not_invited", status: statusNotInvited, accountStatus: "active", membershipStatus: "no_membership", want: v2.Status_RESOURCE_STATUS_DISABLED},
		{name: "deactivated", status: statusDeactivated, accountStatus: "inactive", membershipStatus: "active", want: v2.Status_RESOURCE_STATUS_DISABLED},
		{name: "for_deletion", status: statusForDeletion, accountStatus: "active", membershipStatus: "active", want: v2.Status_RESOURCE_STATUS_DISABLED},
		{name: "known status without sub-statuses", status: statusActive, want: v2.Status_RESOURCE_STATUS_ENABLED},

		// Unknown composite value: classify from the sub-statuses.
		{name: "unknown, both active", status: "unknown", accountStatus: "active", membershipStatus: "active", want: v2.Status_RESOURCE_STATUS_ENABLED},
		{name: "unknown, account closed", status: "unknown", accountStatus: "closed", membershipStatus: "active", want: v2.Status_RESOURCE_STATUS_DISABLED},
		{name: "unknown, account inactive", status: "unknown", accountStatus: "inactive", membershipStatus: "active", want: v2.Status_RESOURCE_STATUS_DISABLED},
		{name: "unknown, membership suspended", status: "unknown", accountStatus: "active", membershipStatus: "suspended", want: v2.Status_RESOURCE_STATUS_DISABLED},
		{name: "unknown, no membership", status: "unknown", accountStatus: "active", membershipStatus: "no_membership", want: v2.Status_RESOURCE_STATUS_DISABLED},
		{name: "unknown, unrecognised sub-status", status: "unknown", accountStatus: "active", membershipStatus: "pending", want: v2.Status_RESOURCE_STATUS_DISABLED},
		{name: "unknown, account inactive, membership suspended", status: "unknown", accountStatus: "inactive", membershipStatus: "suspended", want: v2.Status_RESOURCE_STATUS_DISABLED},

		// One sub-status missing: a non-active one still decides; an active one cannot confirm access.
		{name: "unknown, account missing, membership suspended", status: "unknown", membershipStatus: "suspended", want: v2.Status_RESOURCE_STATUS_DISABLED},
		{name: "unknown, account missing, no membership", status: "unknown", membershipStatus: "no_membership", want: v2.Status_RESOURCE_STATUS_DISABLED},
		{name: "unknown, account closed, membership missing", status: "unknown", accountStatus: "closed", want: v2.Status_RESOURCE_STATUS_DISABLED},
		{name: "unknown, account inactive, membership missing", status: "unknown", accountStatus: "inactive", want: v2.Status_RESOURCE_STATUS_DISABLED},
		{name: "unknown, account active, membership missing", status: "unknown", accountStatus: "active", want: v2.Status_RESOURCE_STATUS_UNSPECIFIED},
		{name: "unknown, account missing, membership active", status: "unknown", membershipStatus: "active", want: v2.Status_RESOURCE_STATUS_UNSPECIFIED},
		{name: "unknown, no sub-statuses", status: "unknown", want: v2.Status_RESOURCE_STATUS_UNSPECIFIED},

		// Status missing entirely.
		{name: "empty status, both active", accountStatus: "active", membershipStatus: "active", want: v2.Status_RESOURCE_STATUS_ENABLED},
		{name: "empty input", want: v2.Status_RESOURCE_STATUS_UNSPECIFIED},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := parseIntoUserResource(client.User{
				AccountId:        testAccountID,
				Status:           tc.status,
				AccountStatus:    tc.accountStatus,
				MembershipStatus: tc.membershipStatus,
			})
			if err != nil {
				t.Fatal(err)
			}
			if got := r.GetStatus().GetStatus(); got != tc.want {
				t.Errorf("status = %v, want %v", got, tc.want)
			}
			if got := r.GetStatus().GetDetails(); got != tc.status {
				t.Errorf("details = %q, want %q", got, tc.status)
			}

			profile := r.GetProfile().AsMap()
			for key, want := range map[string]string{
				"status":            tc.status,
				"account_status":    tc.accountStatus,
				"membership_status": tc.membershipStatus,
			} {
				got, ok := profile[key]
				switch {
				case want == "" && ok:
					t.Errorf("profile[%q] = %v, want key omitted", key, got)
				case want != "" && got != want:
					t.Errorf("profile[%q] = %v, want %q", key, got, want)
				}
			}
		})
	}
}

// Account creation builds the user from SCIM, which only has a boolean "active".
// It must not invent sub-statuses or publish empty profile keys.
func TestSCIMUserResourceProfile(t *testing.T) {
	for _, tc := range []struct {
		active     bool
		wantStatus string
		want       v2.Status_ResourceStatus
	}{
		{active: true, wantStatus: statusActive, want: v2.Status_RESOURCE_STATUS_ENABLED},
		{active: false, wantStatus: statusSuspended, want: v2.Status_RESOURCE_STATUS_DISABLED},
	} {
		t.Run(tc.wantStatus, func(t *testing.T) {
			user := scimUserToUser(&client.SCIMUserResponse{ID: "scim-uuid", Active: tc.active})
			if user.AccountStatus != "" {
				t.Errorf("AccountStatus = %q, want empty", user.AccountStatus)
			}

			r, err := parseIntoUserResource(user)
			if err != nil {
				t.Fatal(err)
			}
			if got := r.GetStatus().GetStatus(); got != tc.want {
				t.Errorf("status = %v, want %v", got, tc.want)
			}

			profile := r.GetProfile().AsMap()
			if got := profile["status"]; got != tc.wantStatus {
				t.Errorf("profile[status] = %v, want %q", got, tc.wantStatus)
			}
			for _, key := range []string{"account_type", "account_status", "membership_status"} {
				if got, ok := profile[key]; ok {
					t.Errorf("profile[%q] = %v, want key omitted", key, got)
				}
			}
		})
	}
}
