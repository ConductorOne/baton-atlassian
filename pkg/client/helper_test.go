package client

import (
	"encoding/json"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAPIErrorMessage(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "errors envelope keeps the detail",
			body: `{"errors":[{"id":"65e99414","code":"ADMIN-UAM-404-1","status":"404","title":"Unknown Resource","detail":"One or more user not found in the directory"}]}`,
			want: "API error response detail: One or more user not found in the directory",
		},
		{
			// TCS/org-routing envelope: the reason lives in "message", not "errors".
			name: "message envelope keeps the reason instead of Error response empty",
			body: `{"timestamp":"2026-08-31T15:42:56.847+00:00","path":"/api/admin/v2/orgs/x","status":404,"error":"Not Found","requestId":"e4cdc7ba","message":"TCS validation failed"}`,
			want: "API error response detail: TCS validation failed",
		},
		{
			name: "lifecycle envelope keeps the errorDetail",
			body: `{"key":"accountNotFound","context":"Error: User account not found","errorKey":"account-not-found","errorDetail":"Error: User account not found"}`,
			want: "API error response detail: Error: User account not found",
		},
		{
			name: "lifecycle bad request keeps the object errorDetail",
			body: `{"key":"badRequestFormat","errorKey":"bad-request-format","errorDetail":{"message":"Invalid content type (application/octet-stream).  These are valid: application/json"}}`,
			want: "API error response detail: Invalid content type (application/octet-stream).  These are valid: application/json",
		},
		{
			name: "lifecycle conflict keeps the context message",
			body: `{"key":"conflict.lifecycleErrors","context":{"message":"Account is not eligible","accountId":"a","errorCodes":[{"code":"x"}]}}`,
			want: "API error response detail: Account is not eligible (x)",
		},
		{
			name: "lifecycle conflict lists every error code",
			body: `{"key":"conflict.lifecycleErrors","context":{"message":"Account is not eligible","accountId":"a","errorCodes":[{"code":"x"},{"code":"y"}]}}`,
			want: "API error response detail: Account is not eligible (x, y)",
		},
		{
			name: "lifecycle conflict with empty errorCodes keeps only the message",
			body: `{"key":"conflict.lifecycleErrors","context":{"message":"Account is not eligible","accountId":"a","errorCodes":[]}}`,
			want: "API error response detail: Account is not eligible",
		},
		{
			name: "lifecycle forbidden keeps the key and the reason key",
			body: `{"key":"forbidden.action","context":{"allowed":false,"reason":{"key":"externalDirectory.scim"}}}`,
			want: "API error response detail: forbidden.action (externalDirectory.scim)",
		},
		{
			name: "lifecycle forbidden with errorDetail keeps the detail and the reason key",
			body: `{"key":"forbidden.action","errorKey":"forbidden-action","errorDetail":"Error: Action not allowed","context":{"allowed":false,"reason":{"key":"externalDirectory.scim"}}}`,
			want: "API error response detail: Error: Action not allowed (externalDirectory.scim)",
		},
		{
			name: "object errorDetail keeps the message and the reason key",
			body: `{"key":"forbidden.action","errorDetail":{"message":"Action not allowed on this account"},"context":{"allowed":false,"reason":{"key":"externalDirectory.scim"}}}`,
			want: "API error response detail: Action not allowed on this account (externalDirectory.scim)",
		},
		{
			name: "lifecycle conflict renders the message of each error code that has one",
			body: `{"key":"conflict.lifecycleErrors","context":{"message":"Account is not eligible","accountId":"a","errorCodes":[{"code":"A","message":"msg"},{"code":"B"}]}}`,
			want: "API error response detail: Account is not eligible (A: msg, B)",
		},
		{
			name: "lifecycle conflict with errorDetail prefers the context message",
			body: `{"key":"conflict.lifecycleErrors","errorDetail":"Error: Lifecycle errors","context":{"message":"Not eligible","accountId":"a","errorCodes":[{"code":"A","message":"msg"}]}}`,
			want: "API error response detail: Not eligible (A: msg)",
		},
		{
			name: "lifecycle conflict with empty message keeps the key and the error codes",
			body: `{"key":"conflict.lifecycleErrors","context":{"message":"","accountId":"a","errorCodes":[{"code":"A"}]}}`,
			want: "API error response detail: conflict.lifecycleErrors (A)",
		},
		{
			name: "SCIM error keeps the detail",
			body: `{"schemas":["urn:ietf:params:scim:api:messages:2.0:Error"],"status":"404","detail":"Directory does not exist"}`,
			want: "API error response detail: Directory does not exist",
		},
		{
			name: "SCIM error without detail keeps the scimType",
			body: `{"schemas":["urn:ietf:params:scim:api:messages:2.0:Error"],"status":"400","scimType":"invalidValue"}`,
			want: "API error response detail: invalidValue",
		},
		{
			name: "SCIM failure keeps the error",
			body: `{"error":"Invalid token"}`,
			want: "API error response detail: Invalid token",
		},
		{
			name: "lifecycle key-only body keeps the key",
			body: `{"key":"forbidden.targetUnverified"}`,
			want: "API error response detail: forbidden.targetUnverified",
		},
		{
			name: "empty body",
			body: `{}`,
			want: "Error response empty",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var apiErr APIError
			if err := json.Unmarshal([]byte(tc.body), &apiErr); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got := apiErr.Message(); got != tc.want {
				t.Errorf("Message() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIsAlreadyMember(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "409 with resource-conflict code is benign",
			err:  conflict("ADMIN-UAM-409-2"),
			want: true,
		},
		{
			name: "409 licence-limit code is a real failure",
			err:  conflict("ADMIN-UAM-409-3"),
			want: false,
		},
		{
			name: "409 other licence-limit code is a real failure",
			err:  conflict("ADMIN-409-3"),
			want: false,
		},
		{
			name: "benign code but not a 409",
			err: &requestError{
				grpcErr: status.Error(codes.NotFound, "404"),
				body:    APIError{Errors: []APIErrorDetail{{Code: "ADMIN-UAM-409-2"}}},
			},
			want: false,
		},
		{
			name: "still detected through fmt.Errorf wrapping",
			err:  fmt.Errorf("baton-atlassian: failed to add user to group: %w", conflict("ADMIN-UAM-409-2")),
			want: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsAlreadyMember(tc.err); got != tc.want {
				t.Errorf("IsAlreadyMember() = %v, want %v", got, tc.want)
			}
		})
	}
}

// A numeric errors[].status must not fail the whole-body unmarshal and drop Code.
func TestAPIErrorDetailStatusUnmarshal(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "string status", body: `{"errors":[{"status":"404","code":"ADMIN-UAM-404-1"}]}`},
		{name: "numeric status", body: `{"errors":[{"status":404,"code":"ADMIN-UAM-404-1"}]}`},
		{name: "null status", body: `{"errors":[{"status":null,"code":"ADMIN-UAM-404-1"}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var apiErr APIError
			if err := json.Unmarshal([]byte(tc.body), &apiErr); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got := apiErr.FirstCode(); got != "ADMIN-UAM-404-1" {
				t.Errorf("FirstCode() = %q, want ADMIN-UAM-404-1", got)
			}
		})
	}
}

func conflict(code string) error {
	return &requestError{
		grpcErr: status.Error(codes.AlreadyExists, "409"),
		body:    APIError{Errors: []APIErrorDetail{{Code: code}}},
	}
}
