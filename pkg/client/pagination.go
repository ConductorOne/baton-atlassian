package client

const ItemsPerPage = 100

type PageOptions struct {
	PageSize  int
	PageToken string
}

func getPageSize(pageSize int) int {
	if pageSize <= 0 || pageSize > ItemsPerPage {
		pageSize = ItemsPerPage
	}
	return pageSize
}
