package client

import (
	"fmt"
	"strconv"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// maxItemsPerPage that the API allows is 100. The default value is 20.
const maxItemsPerPage = 100

func WithPageSize(pageSize int) RequestOpt {
	if pageSize < 0 {
		pageSize = 0
	}
	if pageSize > maxItemsPerPage {
		pageSize = maxItemsPerPage
	}

	return WithQueryParam("limit", strconv.Itoa(pageSize))
}

func WithPageToken(pageToken string) RequestOpt {
	return WithQueryParam("cursor", pageToken)
}

func WithQueryParam(key string, value string) RequestOpt {
	return func(rc *requestConfig) {
		q := rc.url.Query()
		q.Set(key, value)
		rc.url.RawQuery = q.Encode()
	}
}

type RoleAssignmentBody struct {
	Role     string `json:"role"`
	Resource string `json:"resource"`
}

func (er *APIError) Message() string {
	if len(er.Errors) > 0 {
		return fmt.Sprintf("API error response detail: %s", er.Errors[0].Detail)
	}
	return "Error response empty"
}

func IsNotFound(err error) bool {
	return status.Code(err) == codes.NotFound
}
