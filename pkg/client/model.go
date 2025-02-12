package client

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
