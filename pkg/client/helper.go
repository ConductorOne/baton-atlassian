package client

import (
	"errors"
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
	if er.Msg != "" {
		return fmt.Sprintf("API error response detail: %s", er.Msg)
	}
	return "Error response empty"
}

func (er *APIError) FirstCode() string {
	if len(er.Errors) > 0 {
		return er.Errors[0].Code
	}
	return ""
}

// codeAlreadyMember is Atlassian's errors[].code for a genuine group-membership conflict (user
// already a member); a licence-limit 409 uses a different code and is a real failure. Live-measured
// 2026-08-31 (CXH-2373); Atlassian publishes no error-code schema.
const codeAlreadyMember = "ADMIN-UAM-409-2"

// requestError couples the uhttp status error (which carries the gRPC code mapped from the HTTP
// status) with the parsed Atlassian error body, so callers can disambiguate a 409/404 by the vendor
// errors[].code, not by HTTP status alone.
type requestError struct {
	grpcErr error
	body    APIError
}

func (e *requestError) Error() string { return e.grpcErr.Error() }
func (e *requestError) Unwrap() error { return e.grpcErr }

func apiErrorFrom(err error) (*APIError, bool) {
	var re *requestError
	if errors.As(err, &re) {
		return &re.body, true
	}
	return nil, false
}

// IsAlreadyMember reports whether err is the benign 409 where the user is already a group member.
// Atlassian returns 204 for a genuine re-add, so only the resource-conflict code is idempotent; a
// licence-limit 409 returns false and propagates as a failure.
func IsAlreadyMember(err error) bool {
	if status.Code(err) != codes.AlreadyExists {
		return false
	}
	body, ok := apiErrorFrom(err)
	return ok && body.FirstCode() == codeAlreadyMember
}

func IsConflict(err error) bool {
	return status.Code(err) == codes.AlreadyExists
}

func ErrorCode(err error) string {
	if body, ok := apiErrorFrom(err); ok {
		return body.FirstCode()
	}
	return ""
}
