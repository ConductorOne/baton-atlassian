package connector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/conductorone/baton-atlassian/pkg/client"
	"github.com/conductorone/baton-sdk/pkg/annotations"
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
