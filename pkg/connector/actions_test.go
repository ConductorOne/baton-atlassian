package connector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/conductorone/baton-atlassian/pkg/client"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

// The disable and enable lifecycle endpoints return 400 without a JSON Content-Type.
func TestLifecycleActionsSendJSONContentType(t *testing.T) {
	cases := []struct {
		name     string
		handler  func(*Connector, context.Context, *structpb.Struct) (*structpb.Struct, annotations.Annotations, error)
		wantPath string
	}{
		{
			name:     "deactivate_user",
			handler:  (*Connector).handleDeactivateUser,
			wantPath: "/users/" + testAccountID + "/manage/lifecycle/disable",
		},
		{
			name:     "activate_user",
			handler:  (*Connector).handleActivateUser,
			wantPath: "/users/" + testAccountID + "/manage/lifecycle/enable",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod, gotPath, gotContentType string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath, gotContentType = r.Method, r.URL.Path, r.Header.Get("Content-Type")
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()

			ctx := context.Background()
			c, err := client.New(ctx, client.WithAccessToken("token"), client.WithOrganizationID("org"), client.WithBaseURL(srv.URL))
			if err != nil {
				t.Fatal(err)
			}

			args := &structpb.Struct{Fields: map[string]*structpb.Value{"user_id": structpb.NewStringValue(testAccountID)}}
			result, _, err := tc.handler(&Connector{client: c}, ctx, args)
			if err != nil {
				t.Fatalf("handler error: %v", err)
			}
			if !result.GetFields()[successKey].GetBoolValue() {
				t.Errorf("%s = %v, want true", successKey, result.GetFields()[successKey])
			}
			if gotMethod != http.MethodPost || gotPath != tc.wantPath {
				t.Errorf("request = %s %s, want POST %s", gotMethod, gotPath, tc.wantPath)
			}
			if !strings.HasPrefix(gotContentType, "application/json") {
				t.Errorf("Content-Type = %q, want application/json", gotContentType)
			}
		})
	}
}

var lifecycleActionHandlers = []struct {
	name    string
	handler func(*Connector, context.Context, *structpb.Struct) (*structpb.Struct, annotations.Annotations, error)
}{
	{name: "deactivate_user", handler: (*Connector).handleDeactivateUser},
	{name: "activate_user", handler: (*Connector).handleActivateUser},
}

func TestLifecycleActionsRejectInvalidUserID(t *testing.T) {
	cases := []struct {
		name string
		args *structpb.Struct
	}{
		{name: "missing user_id", args: &structpb.Struct{}},
		{name: "empty user_id", args: &structpb.Struct{Fields: map[string]*structpb.Value{"user_id": structpb.NewStringValue("")}}},
	}
	for _, action := range lifecycleActionHandlers {
		for _, tc := range cases {
			t.Run(action.name+"/"+tc.name, func(t *testing.T) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}))
				defer srv.Close()

				ctx := context.Background()
				c, err := client.New(ctx, client.WithAccessToken("token"), client.WithOrganizationID("org"), client.WithBaseURL(srv.URL))
				if err != nil {
					t.Fatal(err)
				}

				_, _, err = action.handler(&Connector{client: c}, ctx, tc.args)
				if status.Code(err) != codes.InvalidArgument {
					t.Errorf("status code = %v, want %v (err: %v)", status.Code(err), codes.InvalidArgument, err)
				}
			})
		}
	}
}

func TestLifecycleActionsSurfaceAPIErrorMessage(t *testing.T) {
	cases := []struct {
		name        string
		statusCode  int
		body        string
		wantMessage string
	}{
		{
			name:        "forbidden keeps the key and the reason key",
			statusCode:  http.StatusForbidden,
			body:        `{"key":"forbidden.action","context":{"allowed":false,"reason":{"key":"externalDirectory.scim"}}}`,
			wantMessage: "forbidden.action (externalDirectory.scim)",
		},
		{
			name:        "conflict keeps the message and the error codes",
			statusCode:  http.StatusConflict,
			body:        `{"key":"conflict.lifecycleErrors","context":{"message":"Account is not eligible","accountId":"a","errorCodes":[{"code":"LIFECYCLE-1"}]}}`,
			wantMessage: "Account is not eligible (LIFECYCLE-1)",
		},
	}
	for _, action := range lifecycleActionHandlers {
		for _, tc := range cases {
			t.Run(action.name+"/"+tc.name, func(t *testing.T) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tc.statusCode)
					if _, err := w.Write([]byte(tc.body)); err != nil {
						t.Errorf("write response: %v", err)
					}
				}))
				defer srv.Close()

				ctx := context.Background()
				c, err := client.New(ctx, client.WithAccessToken("token"), client.WithOrganizationID("org"), client.WithBaseURL(srv.URL))
				if err != nil {
					t.Fatal(err)
				}

				args := &structpb.Struct{Fields: map[string]*structpb.Value{"user_id": structpb.NewStringValue(testAccountID)}}
				_, _, err = action.handler(&Connector{client: c}, ctx, args)
				if err == nil {
					t.Fatal("handler error = nil, want an API error")
				}
				if !strings.Contains(err.Error(), tc.wantMessage) {
					t.Errorf("error = %q, want it to contain %q", err.Error(), tc.wantMessage)
				}
			})
		}
	}
}
