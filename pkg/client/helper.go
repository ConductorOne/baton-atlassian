package client

import (
	"net/url"
	"strconv"
)

// maxItemsPerPage that the API allows is 100. The default value is 20.
const maxItemsPerPage = 100

type ReqOpt func(reqURL *url.URL)

func WithPageSize(pageSize int) ReqOpt {
	if pageSize < 0 {
		pageSize = 0
	}
	if pageSize > maxItemsPerPage {
		pageSize = maxItemsPerPage
	}

	return WithQueryParam("limit", strconv.Itoa(pageSize))
}

func WithPageToken(pageToken string) ReqOpt {
	return WithQueryParam("cursor", pageToken)
}

func WithQueryParam(key string, value string) ReqOpt {
	return func(reqURL *url.URL) {
		q := reqURL.Query()
		q.Set(key, value)
		reqURL.RawQuery = q.Encode()
	}
}
