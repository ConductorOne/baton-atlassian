package connector

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/conductorone/baton-atlassian/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/resource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeAtlassian serves the endpoints the type-scoped group grants use. The directory list
// spans two pages, one directory each. Directory d1 pages its users across two search pages;
// user u1 is in both directories, with a different group in each. Group IDs come back in both bare and ARI form, as users/search returns them.
func fakeAtlassian(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("GET /v2/orgs/org/directories", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("cursor") {
		case "":
			_, _ = io.WriteString(w, `{"data":[{"directoryId":"d1","name":"one"}],"links":{"next":"directories-page2"}}`)
		case "directories-page2":
			_, _ = io.WriteString(w, `{"data":[{"directoryId":"d2","name":"two"}],"links":{}}`)
		default:
			t.Errorf("unexpected directories cursor %q", r.URL.Query().Get("cursor"))
		}
	})

	mux.HandleFunc("POST /v2/orgs/org/directories/{dir}/users/search", func(w http.ResponseWriter, r *http.Request) {
		var body client.UserSearchRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode search body: %v", err)
		}
		if len(body.Expand) != 1 || body.Expand[0] != "groups" {
			t.Errorf("search expand = %v, want [groups]", body.Expand)
		}
		if body.Limit != 100 {
			t.Errorf("search limit = %d, want 100", body.Limit)
		}
		switch dir := r.PathValue("dir"); {
		case dir == "d1" && body.Cursor == "":
			_, _ = io.WriteString(w, `{"data":[{"accountId":"u1","groups":[{"id":"ari:cloud:identity::group/g1"},{"id":"g2"}]}],"links":{"next":"d1-page2"}}`)
		case dir == "d1" && body.Cursor == "d1-page2":
			_, _ = io.WriteString(w, `{"data":[{"accountId":"u2","groups":[{"id":"ari:cloud:identity::group/g1"}]},{"accountId":"u3"}],"links":{}}`)
		case dir == "d2" && body.Cursor == "":
			_, _ = io.WriteString(w, `{"data":[{"accountId":"u1","groups":[{"id":"ari:cloud:identity::group/g3"}]}],"links":{}}`)
		default:
			t.Errorf("unexpected search: dir=%s cursor=%q", dir, body.Cursor)
			w.WriteHeader(http.StatusBadRequest)
		}
	})

	mux.HandleFunc("POST /v2/orgs/org/directories/-/groups/search", func(w http.ResponseWriter, r *http.Request) {
		var body client.GroupSearchRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode group search body: %v", err)
		}
		if body.Limit != 100 {
			t.Errorf("group search limit = %d, want 100", body.Limit)
		}
		switch body.Cursor {
		case "":
			_, _ = io.WriteString(w, `{"data":[{"id":"g1","name":"one"},{"id":"ari:cloud:identity::group/g2","name":"two"}],"links":{"next":"groups-page2"}}`)
		case "groups-page2":
			_, _ = io.WriteString(w, `{"data":[{"id":"g3","name":"three"}],"links":{}}`)
		default:
			t.Errorf("unexpected group search cursor %q", body.Cursor)
			w.WriteHeader(http.StatusBadRequest)
		}
	})

	mux.HandleFunc("GET /v2/orgs/org/directories/-/groups/{group}/role-assignments", func(w http.ResponseWriter, r *http.Request) {
		switch group, cursor := r.PathValue("group"), r.URL.Query().Get("cursor"); {
		case group == "g1" && cursor == "":
			_, _ = io.WriteString(w, `{"data":[
				{"resourceId":"ws1","resourceOwner":"jira","roles":["atlassian/user"]},
				{"resourceId":"org","resourceOwner":"platform","roles":["atlassian/org-admin"]}
			],"links":{"next":"g1-roles2"}}`)
		case group == "g1" && cursor == "g1-roles2":
			_, _ = io.WriteString(w, `{"data":[{"resourceId":"ws2","resourceOwner":"confluence","roles":["atlassian/admin","atlassian/user"]}],"links":{}}`)
		case group == "g2" && cursor == "":
			_, _ = io.WriteString(w, `{"data":[{"resourceId":"ws3","resourceOwner":"jira","roles":["atlassian/user"]}],"links":{}}`)
		default:
			_, _ = io.WriteString(w, `{"data":[],"links":{}}`)
		}
	})

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mux.ServeHTTP(w, r)
	}))
}

// drainGroupGrants runs the type-scoped call the way the syncer does: it follows every
// cursor's next token and runs every spawned token, and returns all emitted grants.
func drainGroupGrants(t *testing.T, b *groupBuilder) []*v2.Grant {
	t.Helper()
	ctx := context.Background()

	var grants []*v2.Grant
	queue := []string{""}
	for calls := 0; len(queue) > 0; calls++ {
		if calls > 100 {
			t.Fatal("too many calls; pagination is not terminating")
		}
		token := queue[0]
		queue = queue[1:]

		out, res, err := b.GrantsForResourceType(ctx, groupResourceType.Id, resource.SyncOpAttrs{PageToken: pagination.Token{Token: token}})
		if err != nil {
			t.Fatalf("GrantsForResourceType(%q): %v", token, err)
		}
		grants = append(grants, out...)
		if res == nil {
			continue
		}
		if res.NextPageToken != "" {
			queue = append(queue, res.NextPageToken)
		}
		spawned := &v2.EnqueuePageTokens{}
		ok, err := res.Annotations.Pick(spawned)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			queue = append(queue, spawned.GetPageTokens()...)
		}
	}
	return grants
}

func TestGroupGrantsForResourceType(t *testing.T) {
	srv := fakeAtlassian(t)
	defer srv.Close()

	c, err := client.New(context.Background(), client.WithAccessToken("token"), client.WithOrganizationID("org"), client.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}

	var got []string
	for _, g := range drainGroupGrants(t, newGroupBuilder(c)) {
		got = append(got, g.GetEntitlement().GetId()+" <- "+g.GetPrincipal().GetId().GetResourceType()+":"+g.GetPrincipal().GetId().GetResource())
	}
	sort.Strings(got)

	want := []string{
		"group:g1:member <- user:u1",
		"group:g1:member <- user:u2",
		"group:g2:member <- user:u1",
		"group:g3:member <- user:u1",
		"workspace:ws1:role:atlassian/user <- group:g1",
		"workspace:ws2:role:atlassian/admin <- group:g1",
		"workspace:ws2:role:atlassian/user <- group:g1",
		"workspace:ws3:role:atlassian/user <- group:g2",
	}
	sort.Strings(want)

	if len(got) != len(want) {
		t.Fatalf("got %d grants, want %d:\n got: %v\nwant: %v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("grant %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestGroupGrantsForResourceTypeRejectsBadTokens(t *testing.T) {
	b := newGroupBuilder(nil)
	cases := []struct {
		name         string
		resourceType string
		input        string
	}{
		{name: "wrong resource type", resourceType: userResourceType.Id, input: ""},
		{name: "not json", resourceType: groupResourceType.Id, input: "not-json"},
		{name: "unknown kind", resourceType: groupResourceType.Id, input: `{"kind":"other"}`},
		{name: "members without directory", resourceType: groupResourceType.Id, input: `{"kind":"members"}`},
		{name: "roles without group", resourceType: groupResourceType.Id, input: `{"kind":"roles"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := b.GrantsForResourceType(context.Background(), tc.resourceType, resource.SyncOpAttrs{PageToken: pagination.Token{Token: tc.input}})
			if status.Code(err) != codes.InvalidArgument {
				t.Errorf("code = %v, want InvalidArgument (err: %v)", status.Code(err), err)
			}
		})
	}
}

func TestGroupIDFromARI(t *testing.T) {
	cases := map[string]string{
		"ari:cloud:identity::group/abc": "abc",
		"abc":                           "abc",
		"":                              "",
	}
	for in, want := range cases {
		if got := client.GroupIDFromARI(in); got != want {
			t.Errorf("GroupIDFromARI(%q) = %q, want %q", in, got, want)
		}
	}
}
