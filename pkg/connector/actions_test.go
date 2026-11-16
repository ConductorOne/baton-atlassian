package connector

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/conductorone/baton-atlassian/pkg/client"
	config_sdk "github.com/conductorone/baton-sdk/pb/c1/config/v1"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/actions"
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

type actionHandler struct {
	name    string
	handler func(*Connector, context.Context, *structpb.Struct) (*structpb.Struct, annotations.Annotations, error)
}

var lifecycleActionHandlers = []actionHandler{
	{name: "deactivate_user", handler: (*Connector).handleDeactivateUser},
	{name: "activate_user", handler: (*Connector).handleActivateUser},
}

var directoryActionHandlers = []actionHandler{
	{name: "disable_user", handler: (*Connector).handleDisableUser},
	{name: "enable_user", handler: (*Connector).handleEnableUser},
}

func TestLifecycleActionsRejectInvalidUserID(t *testing.T) {
	cases := []struct {
		name string
		args *structpb.Struct
	}{
		{name: "missing user_id", args: &structpb.Struct{}},
		{name: "empty user_id", args: &structpb.Struct{Fields: map[string]*structpb.Value{"user_id": structpb.NewStringValue("")}}},
	}
	for _, action := range append(append([]actionHandler{}, lifecycleActionHandlers...), directoryActionHandlers...) {
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

type directoryPage struct {
	ids  []string
	next string
}

// directoryAPI fakes the directories lookup and the per-directory suspend/restore endpoints.
type directoryAPI struct {
	t            *testing.T
	pages        map[string]directoryPage
	lookupStatus int
	postStatus   map[string]int
	postBody     map[string]string

	mu          sync.Mutex
	lookups     []string
	lookupQuery []url.Values
	posts       []string
}

func (f *directoryAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodGet && r.URL.Path == "/v2/orgs/org/directories" {
		f.lookups = append(f.lookups, r.URL.EscapedPath())
		f.lookupQuery = append(f.lookupQuery, r.URL.Query())
		if f.lookupStatus != 0 {
			w.WriteHeader(f.lookupStatus)
			_, _ = w.Write([]byte(`{"errors":[{"status":"500","code":"ERR","title":"t","detail":"lookup failed"}]}`))
			return
		}
		page := f.pages[r.URL.Query().Get("cursor")]
		data := make([]map[string]string, 0, len(page.ids))
		for _, id := range page.ids {
			data = append(data, map[string]string{"directoryId": id, "name": "Directory " + id})
		}
		if err := json.NewEncoder(w).Encode(map[string]any{"data": data, "links": map[string]string{"next": page.next}}); err != nil {
			f.t.Errorf("encode directories: %v", err)
		}
		return
	}

	if r.Method != http.MethodPost {
		f.t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	f.posts = append(f.posts, r.URL.EscapedPath())
	parts := strings.Split(r.URL.Path, "/")
	directoryID := ""
	if len(parts) > 5 {
		directoryID = parts[5]
	}
	if code, ok := f.postStatus[directoryID]; ok {
		w.WriteHeader(code)
		_, _ = w.Write([]byte(f.postBody[directoryID]))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func newDirectoryTestConnector(t *testing.T, api *directoryAPI) *Connector {
	t.Helper()
	api.t = t
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)
	c, err := client.New(context.Background(), client.WithAccessToken("token"), client.WithOrganizationID("org"), client.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	return &Connector{client: c}
}

func userArgs(id string) *structpb.Struct {
	return &structpb.Struct{Fields: map[string]*structpb.Value{"user_id": structpb.NewStringValue(id)}}
}

func directoryActionSuffix(name string) string {
	if name == "disable_user" {
		return "suspend"
	}
	return "restore"
}

func TestDirectoryActionsCallEveryDirectory(t *testing.T) {
	cases := []struct {
		name      string
		pages     map[string]directoryPage
		wantDirs  []string
		wantPages int
	}{
		{
			name:      "one directory",
			pages:     map[string]directoryPage{"": {ids: []string{"d1"}}},
			wantDirs:  []string{"d1"},
			wantPages: 1,
		},
		{
			name:      "two directories in order",
			pages:     map[string]directoryPage{"": {ids: []string{"d1", "d2"}}},
			wantDirs:  []string{"d1", "d2"},
			wantPages: 1,
		},
		{
			name: "directories across two pages",
			pages: map[string]directoryPage{
				"":      {ids: []string{"d1"}, next: "page2"},
				"page2": {ids: []string{"d2"}},
			},
			wantDirs:  []string{"d1", "d2"},
			wantPages: 2,
		},
	}
	for _, action := range directoryActionHandlers {
		for _, tc := range cases {
			t.Run(action.name+"/"+tc.name, func(t *testing.T) {
				api := &directoryAPI{pages: tc.pages}
				conn := newDirectoryTestConnector(t, api)

				result, _, err := action.handler(conn, context.Background(), userArgs(testAccountID))
				if err != nil {
					t.Fatalf("handler error: %v", err)
				}
				if !result.GetFields()[successKey].GetBoolValue() {
					t.Errorf("%s = %v, want true", successKey, result.GetFields()[successKey])
				}

				if len(api.lookupQuery) != tc.wantPages {
					t.Fatalf("directory lookups = %d, want %d", len(api.lookupQuery), tc.wantPages)
				}
				for i, q := range api.lookupQuery {
					if q.Get("accountId") != testAccountID || q.Get("limit") != "100" {
						t.Errorf("lookup %d query = %v, want accountId=%s and limit=100", i, q, testAccountID)
					}
				}
				if tc.wantPages > 1 && api.lookupQuery[1].Get("cursor") != "page2" {
					t.Errorf("second lookup cursor = %q, want page2", api.lookupQuery[1].Get("cursor"))
				}

				var want []string
				for _, d := range tc.wantDirs {
					want = append(want, "/v2/orgs/org/directories/"+d+"/users/"+url.PathEscape(testAccountID)+"/"+directoryActionSuffix(action.name))
				}
				if !slices.Equal(api.posts, want) {
					t.Errorf("posts = %v, want %v", api.posts, want)
				}
			})
		}
	}
}

func TestDirectoryActionsNoDirectoriesIsNotFound(t *testing.T) {
	for _, action := range directoryActionHandlers {
		t.Run(action.name, func(t *testing.T) {
			api := &directoryAPI{pages: map[string]directoryPage{"": {}}}
			conn := newDirectoryTestConnector(t, api)

			_, _, err := action.handler(conn, context.Background(), userArgs(testAccountID))
			if status.Code(err) != codes.NotFound {
				t.Errorf("status code = %v, want %v (err: %v)", status.Code(err), codes.NotFound, err)
			}
			if len(api.posts) != 0 {
				t.Errorf("posts = %v, want none", api.posts)
			}
		})
	}
}

func TestDirectoryActionsLookupFailureSendsNoPost(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		wantCode codes.Code
	}{
		{name: "server error", status: http.StatusInternalServerError, wantCode: codes.Unavailable},
		{name: "unauthorized", status: http.StatusUnauthorized, wantCode: codes.Unauthenticated},
	}
	for _, action := range directoryActionHandlers {
		for _, tc := range cases {
			t.Run(action.name+"/"+tc.name, func(t *testing.T) {
				api := &directoryAPI{lookupStatus: tc.status}
				conn := newDirectoryTestConnector(t, api)

				_, _, err := action.handler(conn, context.Background(), userArgs(testAccountID))
				if err == nil {
					t.Fatal("handler error = nil, want a lookup error")
				}
				if status.Code(err) != tc.wantCode {
					t.Errorf("status code = %v, want %v (err: %v)", status.Code(err), tc.wantCode, err)
				}
				if len(api.posts) != 0 {
					t.Errorf("posts = %v, want none", api.posts)
				}
			})
		}
	}
}

func TestDirectoryActionsPartialFailure(t *testing.T) {
	const forbiddenBody = `{"errors":[{"status":"403","code":"ADMIN-403","title":"Forbidden","detail":"Directory is managed by an identity provider"}]}`
	cases := []struct {
		name       string
		failingDir string
		otherDir   string
	}{
		{name: "second directory fails", failingDir: "d2", otherDir: "d1"},
		{name: "first directory fails and the loop continues", failingDir: "d1", otherDir: "d2"},
	}
	for _, action := range directoryActionHandlers {
		for _, tc := range cases {
			t.Run(action.name+"/"+tc.name, func(t *testing.T) {
				api := &directoryAPI{
					pages:      map[string]directoryPage{"": {ids: []string{"d1", "d2"}}},
					postStatus: map[string]int{tc.failingDir: http.StatusForbidden},
					postBody:   map[string]string{tc.failingDir: forbiddenBody},
				}
				conn := newDirectoryTestConnector(t, api)

				_, _, err := action.handler(conn, context.Background(), userArgs(testAccountID))
				if err == nil {
					t.Fatal("handler error = nil, want a partial failure")
				}
				msg := err.Error()
				for _, want := range []string{tc.failingDir + ":", "1 of 2 directories", "Directory is managed by an identity provider"} {
					if !strings.Contains(msg, want) {
						t.Errorf("error = %q, want it to contain %q", msg, want)
					}
				}
				if strings.Contains(msg, tc.otherDir+":") {
					t.Errorf("error = %q, should not name %s", msg, tc.otherDir)
				}
				if status.Code(err) != codes.PermissionDenied {
					t.Errorf("status code = %v, want %v", status.Code(err), codes.PermissionDenied)
				}
				if len(api.posts) != 2 {
					t.Errorf("posts = %v, want both directories called", api.posts)
				}
			})
		}
	}
}

func TestDirectoryActionsBadRequestFails(t *testing.T) {
	for _, action := range directoryActionHandlers {
		t.Run(action.name, func(t *testing.T) {
			api := &directoryAPI{
				pages:      map[string]directoryPage{"": {ids: []string{"d1"}}},
				postStatus: map[string]int{"d1": http.StatusBadRequest},
				postBody:   map[string]string{"d1": `{"errors":[{"status":"400","code":"ADMIN-400","title":"Bad Request","detail":"invalid request"}]}`},
			}
			conn := newDirectoryTestConnector(t, api)

			_, _, err := action.handler(conn, context.Background(), userArgs(testAccountID))
			if status.Code(err) != codes.InvalidArgument {
				t.Errorf("status code = %v, want %v (err: %v)", status.Code(err), codes.InvalidArgument, err)
			}
		})
	}
}

func TestDirectoryActionsEscapeAccountID(t *testing.T) {
	for _, action := range directoryActionHandlers {
		t.Run(action.name, func(t *testing.T) {
			api := &directoryAPI{pages: map[string]directoryPage{"": {ids: []string{"d1"}}}}
			conn := newDirectoryTestConnector(t, api)

			if _, _, err := action.handler(conn, context.Background(), userArgs("a/b?c")); err != nil {
				t.Fatalf("handler error: %v", err)
			}
			want := "/v2/orgs/org/directories/d1/users/a%2Fb%3Fc/" + directoryActionSuffix(action.name)
			if len(api.posts) != 1 || api.posts[0] != want {
				t.Errorf("posts = %v, want [%s]", api.posts, want)
			}
			if got := api.lookupQuery[0].Get("accountId"); got != "a/b?c" {
				t.Errorf("accountId query = %q, want %q", got, "a/b?c")
			}
		})
	}
}

type recordingRegistry struct {
	schemas map[string]*v2.BatonActionSchema
}

func (r *recordingRegistry) Register(_ context.Context, schema *v2.BatonActionSchema, _ actions.ActionHandler) error {
	r.schemas[schema.GetName()] = schema
	return nil
}

func (r *recordingRegistry) RegisterAction(ctx context.Context, _ string, schema *v2.BatonActionSchema, handler actions.ActionHandler) error {
	return r.Register(ctx, schema, handler)
}

// C1 matches lifecycle actions by name and requires exactly one argument, so this contract must not change.
func TestDirectoryActionSchemasAreStable(t *testing.T) {
	registry := &recordingRegistry{schemas: map[string]*v2.BatonActionSchema{}}
	if err := (&Connector{}).GlobalActions(context.Background(), registry); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name        string
		displayName string
		actionType  v2.ActionType
	}{
		{name: "disable_user", displayName: "Suspend User", actionType: v2.ActionType_ACTION_TYPE_ACCOUNT_DISABLE},
		{name: "enable_user", displayName: "Restore User", actionType: v2.ActionType_ACTION_TYPE_ACCOUNT_ENABLE},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			schema, ok := registry.schemas[tc.name]
			if !ok {
				t.Fatalf("action %s not registered", tc.name)
			}
			if schema.GetDisplayName() != tc.displayName {
				t.Errorf("display name = %q, want %q", schema.GetDisplayName(), tc.displayName)
			}
			args := schema.GetArguments()
			if len(args) != 1 || args[0].GetName() != "user_id" || !args[0].GetIsRequired() || !isStringField(args[0]) {
				t.Errorf("arguments = %v, want exactly one required string user_id", args)
			}
			returns := schema.GetReturnTypes()
			if len(returns) != 1 || returns[0].GetName() != successKey || !isBoolField(returns[0]) {
				t.Errorf("return types = %v, want exactly one bool %s", returns, successKey)
			}
			if !slices.Equal(schema.GetActionType(), []v2.ActionType{tc.actionType}) {
				t.Errorf("action types = %v, want [%v]", schema.GetActionType(), tc.actionType)
			}
		})
	}
}

func isStringField(f *config_sdk.Field) bool {
	_, ok := f.GetField().(*config_sdk.Field_StringField)
	return ok
}

func isBoolField(f *config_sdk.Field) bool {
	_, ok := f.GetField().(*config_sdk.Field_BoolField)
	return ok
}

func TestNoRemovedSuspendEndpoints(t *testing.T) {
	pkgFS := os.DirFS("..")
	err := fs.WalkDir(pkgFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, err := fs.ReadFile(pkgFS, path)
		if err != nil {
			return err
		}
		for _, removed := range []string{"suspend-access", "restore-access"} {
			if strings.Contains(string(content), removed) {
				t.Errorf("%s still references the removed %s endpoint", path, removed)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
