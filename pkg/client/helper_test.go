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

func TestIsNotFoundThroughWrapping(t *testing.T) {
	err := fmt.Errorf("baton-atlassian: resolve directory: %w",
		&requestError{grpcErr: status.Error(codes.NotFound, "404"), body: APIError{}})
	if !IsNotFound(err) {
		t.Errorf("IsNotFound() = false, want true")
	}
	if status.Code(err) != codes.NotFound {
		t.Errorf("status.Code() = %v, want NotFound", status.Code(err))
	}
}

func conflict(code string) error {
	return &requestError{
		grpcErr: status.Error(codes.AlreadyExists, "409"),
		body:    APIError{Errors: []APIErrorDetail{{Code: code}}},
	}
}
