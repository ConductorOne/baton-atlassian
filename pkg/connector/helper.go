package connector

import (
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/pagination"
)

func getToken(pToken *pagination.Token, resourceType *v2.ResourceType) (*pagination.Bag, string, error) {
	var pageToken string
	_, bag, err := unmarshalSkipToken(pToken)
	if err != nil {
		return bag, "", err
	}

	if bag.Current() == nil {
		bag.Push(pagination.PageState{
			ResourceTypeID: resourceType.Id,
		})
	}

	if bag.Current().Token != "" {
		pageToken = bag.Current().Token
	}

	return bag, pageToken, nil
}

func unmarshalSkipToken(token *pagination.Token) (string, *pagination.Bag, error) {
	b := &pagination.Bag{}
	err := b.Unmarshal(token.Token)
	if err != nil {
		return "", nil, err
	}
	current := b.Current()
	var skip string
	if current != nil && current.Token != "" {
		skip = current.Token
	}
	return skip, b, nil
}
