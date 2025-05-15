package client

import "time"

type PageInfo struct {
	hasNextPage bool
	endCursor   string
}
type TeamQuery struct {
	Team struct {
		TeamSearch TeamSearch `json:"teamSearchV2"`
	} `json:"team"`
}

type TeamSearch struct {
	PageInfo PageInfo   `json:"pageInfo"`
	Edges    []TeamEdge `json:"edges"`
}

type TeamEdge struct {
	Node struct {
		Team Team `json:"team"`
	} `json:"node"`
}

type Team struct {
	ID             string           `json:"id"`
	OrganizationID string           `json:"organizationId"`
	DisplayName    string           `json:"displayName"`
	Description    string           `json:"description"`
	Members        MemberConnection `json:"members"`
}

type MemberConnection struct {
	PageInfo PageInfo     `json:"pageInfo"`
	Edges    []MemberEdge `json:"edges"`
}

type MemberEdge struct {
	Node struct {
		Member Member `json:"member"`
		Role   string `json:"role"`
	} `json:"node"`
}

type Member struct {
	ID        string `json:"id"`
	AccountID string `json:"accountId"`
	Name      string `json:"name"`
}

type UserResponse struct {
	Data  []User `json:"data"`
	Links struct {
		Next string `json:"next"`
		Prev string `json:"prev"`
		Self string `json:"self"`
	} `json:"links"`
}

type User struct {
	AccountId        string    `json:"accountId"`
	AccountType      string    `json:"accountType"`
	Status           string    `json:"status"`
	AccountStatus    string    `json:"accountStatus"`
	MembershipStatus string    `json:"membershipStatus"`
	AddedToOrg       time.Time `json:"addedToOrg"`
	Name             string    `json:"name"`
	Nickname         string    `json:"nickname"`
	Email            string    `json:"email"`
	EmailVerified    bool      `json:"emailVerified"`
	ClaimStatus      string    `json:"claimStatus"`
	PlatformRoles    []string  `json:"platformRoles"`
	Picture          string    `json:"picture"`
	Avatar           string    `json:"avatar"`
	Counts           struct {
		Resources int `json:"resources"`
	} `json:"counts"`
	Links struct {
		Self string `json:"self"`
	} `json:"links"`
}
