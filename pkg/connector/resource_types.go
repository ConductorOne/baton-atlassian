package connector

import (
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
)

var userResourceType = &v2.ResourceType{
	Id:          "user",
	DisplayName: "User",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_USER},
}

var workspaceResourceType = &v2.ResourceType{
	Id:          "workspace",
	DisplayName: "Workspace",
}

// Group grants are synced type-scoped; see GrantsForResourceType in groups.go.
var groupResourceType = &v2.ResourceType{
	Id:          "group",
	DisplayName: "Group",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_GROUP},
	Annotations: annotations.New(&v2.TypeScopedGrants{}),
}

var organizationResourceType = &v2.ResourceType{
	Id:          "organization",
	DisplayName: "Organization",
}

var apiTokenResourceType = &v2.ResourceType{
	Id:          "api_token",
	DisplayName: "API Token",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_SECRET},
}
