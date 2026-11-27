package connector

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/conductorone/baton-atlassian/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	"github.com/conductorone/baton-sdk/pkg/types/resource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type groupBuilder struct {
	resourceType *v2.ResourceType
	client       *client.AtlassianClient
}

func (b *groupBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return groupResourceType
}

func (b *groupBuilder) List(ctx context.Context, _ *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	var groupResources []*v2.Resource

	bag, pageToken, err := getToken(pToken, groupResourceType)
	if err != nil {
		return nil, "", nil, err
	}

	groups, nextPageToken, err := b.client.ListGroups(ctx, pageToken)
	if err != nil {
		return nil, "", nil, err
	}

	for _, group := range groups {
		groupResource, err := parseIntoGroupResource(group)
		if err != nil {
			return nil, "", nil, err
		}
		groupResources = append(groupResources, groupResource)
	}

	err = bag.Next(nextPageToken)
	if err != nil {
		return nil, "", nil, err
	}
	nextPageToken, err = bag.Marshal()
	if err != nil {
		return nil, "", nil, err
	}

	return groupResources, nextPageToken, nil, nil
}

func (b *groupBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	var groupEntitlements []*v2.Entitlement

	memberEntitlement := entitlement.NewAssignmentEntitlement(
		resource,
		"member",
		entitlement.WithGrantableTo(userResourceType),
		entitlement.WithDisplayName(fmt.Sprintf("%s member", resource.DisplayName)),
		entitlement.WithDescription(fmt.Sprintf("Member of the %s group", resource.DisplayName)),
	)
	groupEntitlements = append(groupEntitlements, memberEntitlement)

	return groupEntitlements, "", nil, nil
}

// Grants is never called for groups: the group type carries the TypeScopedGrants annotation,
// so the syncer calls GrantsForResourceType (below) for the whole type instead. It errors
// rather than returning nothing, because an empty result would read as every membership being
// removed.
func (b *groupBuilder) Grants(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, status.Error(codes.Unimplemented, "baton-atlassian: group grants are synced type-scoped; use GrantsForResourceType")
}

// Group grants are synced type-scoped: the syncer calls GrantsForResourceType for the whole
// group type instead of Grants once per group. That lets membership be read from the user
// side (each user's groups) in one paged pass per directory, instead of listing each group's
// members with the users?groupIds= filter, which times out on very large groups.
//
// The first call (empty token) spawns two independent cursors and returns no grants:
//   - a "directories" cursor paging the directory list, which spawns a "members" cursor per
//     directory to page users/search with expand=groups;
//   - a "groups" cursor paging the group list, which spawns a "roles" cursor per group to page
//     that group's role assignments.
//
// Each token is self-contained, because the SDK may run a cursor on another worker or after
// a resume.
const (
	groupTokenDirectories = "directories"
	groupTokenMembers     = "members"
	groupTokenGroups      = "groups"
	groupTokenRoles       = "roles"
)

type groupGrantsToken struct {
	Kind        string `json:"kind"`
	DirectoryID string `json:"directoryId,omitempty"`
	GroupID     string `json:"groupId,omitempty"`
	Cursor      string `json:"cursor,omitempty"`
}

func (t groupGrantsToken) encode() (string, error) {
	b, err := json.Marshal(t)
	if err != nil {
		return "", fmt.Errorf("baton-atlassian: failed to encode group grants token: %w", err)
	}
	return string(b), nil
}

// next returns the token for the following page of the same cursor, or "" when done.
func (t groupGrantsToken) next(cursor string) (string, error) {
	if cursor == "" {
		return "", nil
	}
	t.Cursor = cursor
	return t.encode()
}

func groupResourceID(groupID string) *v2.Resource {
	return &v2.Resource{
		Id: &v2.ResourceId{
			ResourceType: groupResourceType.Id,
			Resource:     groupID,
		},
	}
}

func (b *groupBuilder) GrantsForResourceType(ctx context.Context, resourceTypeID string, opts resource.SyncOpAttrs) ([]*v2.Grant, *resource.SyncOpResults, error) {
	if resourceTypeID != groupResourceType.Id {
		return nil, nil, status.Errorf(codes.InvalidArgument, "baton-atlassian: type-scoped grants requested for unexpected resource type %q", resourceTypeID)
	}

	if opts.PageToken.Token == "" {
		return b.planGroupGrants()
	}

	var token groupGrantsToken
	if err := json.Unmarshal([]byte(opts.PageToken.Token), &token); err != nil {
		return nil, nil, status.Errorf(codes.InvalidArgument, "baton-atlassian: invalid group grants token: %v", err)
	}

	switch token.Kind {
	case groupTokenDirectories:
		return b.spawnGroupMemberCursors(ctx, token)
	case groupTokenMembers:
		return b.groupMemberGrants(ctx, token)
	case groupTokenGroups:
		return b.spawnGroupRoleCursors(ctx, token)
	case groupTokenRoles:
		return b.groupRoleGrants(ctx, token)
	default:
		return nil, nil, status.Errorf(codes.InvalidArgument, "baton-atlassian: unknown group grants token kind %q", token.Kind)
	}
}

// planGroupGrants spawns the directories cursor and the groups cursor. It returns no grants.
func (b *groupBuilder) planGroupGrants() ([]*v2.Grant, *resource.SyncOpResults, error) {
	directoriesToken, err := groupGrantsToken{Kind: groupTokenDirectories}.encode()
	if err != nil {
		return nil, nil, err
	}
	groupsToken, err := groupGrantsToken{Kind: groupTokenGroups}.encode()
	if err != nil {
		return nil, nil, err
	}
	return nil, &resource.SyncOpResults{Annotations: enqueueTokens([]string{directoriesToken, groupsToken})}, nil
}

// spawnGroupMemberCursors pages the directory list and spawns a members cursor for each
// directory on the page. It returns no grants itself.
func (b *groupBuilder) spawnGroupMemberCursors(ctx context.Context, token groupGrantsToken) ([]*v2.Grant, *resource.SyncOpResults, error) {
	directories, next, err := b.client.ListDirectories(ctx, token.Cursor)
	if err != nil {
		return nil, nil, orgWideListError("list directories", err)
	}

	tokens := make([]string, 0, len(directories))
	for _, directory := range directories {
		if directory.DirectoryId == "" {
			return nil, nil, status.Error(codes.Internal, "baton-atlassian: directory list returned an entry without a directory id")
		}
		membersToken, err := groupGrantsToken{Kind: groupTokenMembers, DirectoryID: directory.DirectoryId}.encode()
		if err != nil {
			return nil, nil, err
		}
		tokens = append(tokens, membersToken)
	}

	nextToken, err := token.next(next)
	if err != nil {
		return nil, nil, err
	}
	return nil, &resource.SyncOpResults{NextPageToken: nextToken, Annotations: enqueueTokens(tokens)}, nil
}

// groupMemberGrants emits a member grant for every group of every user on one page of a
// directory's users.
func (b *groupBuilder) groupMemberGrants(ctx context.Context, token groupGrantsToken) ([]*v2.Grant, *resource.SyncOpResults, error) {
	if token.DirectoryID == "" {
		return nil, nil, status.Error(codes.InvalidArgument, "baton-atlassian: members token is missing a directory id")
	}

	users, next, err := b.client.SearchUsersWithGroups(ctx, token.DirectoryID, token.Cursor)
	if err != nil {
		return nil, nil, fmt.Errorf("baton-atlassian: failed to search users with groups in directory %s: %w", token.DirectoryID, err)
	}

	var grants []*v2.Grant
	for _, user := range users {
		userResource := &v2.Resource{
			Id: &v2.ResourceId{
				ResourceType: userResourceType.Id,
				Resource:     user.AccountId,
			},
		}
		for _, group := range user.Groups {
			groupID := client.GroupIDFromARI(group.ID)
			if groupID == "" {
				continue
			}
			grants = append(grants, grant.NewGrant(groupResourceID(groupID), "member", userResource))
		}
	}

	nextToken, err := token.next(next)
	if err != nil {
		return nil, nil, err
	}
	return grants, &resource.SyncOpResults{NextPageToken: nextToken}, nil
}

// spawnGroupRoleCursors pages the group list and spawns a roles cursor for each group on the
// page. It returns no grants itself.
func (b *groupBuilder) spawnGroupRoleCursors(ctx context.Context, token groupGrantsToken) ([]*v2.Grant, *resource.SyncOpResults, error) {
	groups, next, err := b.client.SearchGroups(ctx, token.Cursor)
	if err != nil {
		return nil, nil, orgWideListError("search groups", err)
	}

	tokens := make([]string, 0, len(groups))
	for _, group := range groups {
		groupID := client.GroupIDFromARI(group.ID)
		if groupID == "" {
			return nil, nil, status.Error(codes.Internal, "baton-atlassian: group search returned an entry without a group id")
		}
		roleToken, err := groupGrantsToken{Kind: groupTokenRoles, GroupID: groupID}.encode()
		if err != nil {
			return nil, nil, err
		}
		tokens = append(tokens, roleToken)
	}

	nextToken, err := token.next(next)
	if err != nil {
		return nil, nil, err
	}
	return nil, &resource.SyncOpResults{NextPageToken: nextToken, Annotations: enqueueTokens(tokens)}, nil
}

// groupRoleGrants emits the workspace role grants for one page of a group's role assignments.
func (b *groupBuilder) groupRoleGrants(ctx context.Context, token groupGrantsToken) ([]*v2.Grant, *resource.SyncOpResults, error) {
	if token.GroupID == "" {
		return nil, nil, status.Error(codes.InvalidArgument, "baton-atlassian: roles token is missing a group id")
	}

	roleAssignments, next, err := b.client.GetGroupRoleAssignments(ctx, token.Cursor, token.GroupID)
	if err != nil {
		return nil, nil, fmt.Errorf("baton-atlassian: failed to get role assignments for group %s: %w", token.GroupID, err)
	}

	grants := groupRoleAssignmentGrants(groupResourceID(token.GroupID), roleAssignments)

	nextToken, err := token.next(next)
	if err != nil {
		return nil, nil, err
	}
	return grants, &resource.SyncOpResults{NextPageToken: nextToken}, nil
}

// orgWideListError wraps an error from an org-wide list (directories, groups search) whose
// cursor spawns every members or roles cursor. The SDK treats NotFound from a grants call as a
// skipped resource and still finishes the sync, which is right for one deleted directory or
// group, but here it would drop every grant the cursor spawns, and C1 would read them all as
// removed. An org-wide list cannot be legitimately missing, so NotFound becomes Internal and
// fails the sync instead.
func orgWideListError(operation string, err error) error {
	if status.Code(err) == codes.NotFound {
		return status.Errorf(codes.Internal, "baton-atlassian: failed to %s: the org-wide list returned not found: %v", operation, err)
	}
	return fmt.Errorf("baton-atlassian: failed to %s: %w", operation, err)
}

func enqueueTokens(tokens []string) annotations.Annotations {
	if len(tokens) == 0 {
		return nil
	}
	return annotations.New(v2.EnqueuePageTokens_builder{PageTokens: tokens}.Build())
}

// groupRoleAssignmentGrants turns a group's role assignments into workspace role grants
// whose principal is the group.
func groupRoleAssignmentGrants(group *v2.Resource, roleAssignments []client.RoleAssignment) []*v2.Grant {
	var grants []*v2.Grant
	for _, roleAssignment := range roleAssignments {
		// We only want to sync the role assignments for the Atlassian Products (Workspaces).
		// The ignored scopes refers to:
		//     - platform: Organization-level roles (org-admin, site-admin) are already synced directly from user role-assignments.
		//     - project: This is a specific context within Jira. Roles with 'project' as the resource owner are typically project roles defined
		// within a particular Jira project (e.g., "Administrators", "Developers", "Users" within a single Jira project).
		//     - goal: This likely refers to roles related to Atlas Goals. If the organization uses Atlas for setting and tracking goals,
		// roles assigned within that product might have 'goal' as the resource owner.
		if roleAssignment.ResourceOwner == resourceOwnerPlatform ||
			roleAssignment.ResourceOwner == resourceOwnerProject ||
			roleAssignment.ResourceOwner == resourceOwnerGoal {
			continue
		}

		workspaceResource := &v2.Resource{
			Id: &v2.ResourceId{
				ResourceType: workspaceResourceType.Id,
				Resource:     roleAssignment.ResourceId,
			},
		}

		for _, role := range roleAssignment.Roles {
			entitlementID := fmt.Sprintf("role:%s", role)
			grants = append(grants, grant.NewGrant(workspaceResource, entitlementID, group))
		}
	}
	return grants
}

func (b *groupBuilder) Grant(ctx context.Context, principal *v2.Resource, entitlement *v2.Entitlement) (annotations.Annotations, error) {
	if principal.Id.ResourceType != userResourceType.Id {
		return nil, fmt.Errorf("baton-atlassian: only users can be granted group membership")
	}

	groupID := entitlement.Resource.Id.Resource
	accountID := principal.Id.Resource

	directoryID, err := b.client.GetGroupDirectoryID(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("baton-atlassian: failed to resolve group directory: %w", err)
	}

	err = b.client.AddUserToGroup(ctx, directoryID, groupID, accountID)
	if err != nil {
		if client.IsAlreadyMember(err) {
			return annotations.New(&v2.GrantAlreadyExists{}), nil
		}
		return nil, fmt.Errorf("baton-atlassian: failed to add user to group (code %q): %w", client.ErrorCode(err), err)
	}

	return nil, nil
}

func (b *groupBuilder) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	if grant.Principal.Id.ResourceType != userResourceType.Id {
		return nil, fmt.Errorf("baton-atlassian: only users can be revoked from group membership")
	}

	groupID := grant.Entitlement.Resource.Id.Resource
	accountID := grant.Principal.Id.Resource

	directoryID, err := b.client.GetGroupDirectoryID(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("baton-atlassian: failed to resolve group directory: %w", err)
	}

	// Atlassian returns 204 for an already-absent membership (live-measured, CXH-2373), so a
	// repeat revoke is already idempotent here and needs no GrantAlreadyRevoked branch.
	err = b.client.RemoveUserFromGroup(ctx, directoryID, groupID, accountID)
	if err != nil {
		return nil, fmt.Errorf("baton-atlassian: failed to remove user from group: %w", err)
	}

	return nil, nil
}

func parseIntoGroupResource(group client.Group) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"name":        group.Name,
		"description": group.Description,
	}

	groupTraits := []resource.GroupTraitOption{}
	return resource.NewGroupResource(
		group.Name,
		groupResourceType,
		group.ID,
		groupTraits,
		resource.WithResourceProfile(profile),
	)
}

func newGroupBuilder(c *client.AtlassianClient) *groupBuilder {
	return &groupBuilder{
		resourceType: groupResourceType,
		client:       c,
	}
}
