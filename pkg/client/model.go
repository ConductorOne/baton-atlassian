package client

import "time"

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
