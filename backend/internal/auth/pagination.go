package auth

const (
	defaultPageSize = 50
	maxPageSize     = 100
)

// Page contains one bounded page of authentication metadata and the total matching count.
type Page[T any] struct {
	Items    []T `json:"items"`
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
	Total    int `json:"total"`
}

// paginate receives ordered items and raw page values and returns a bounded page.
func paginate[T any](items []T, page int, pageSize int) Page[T] {
	page, pageSize = normalizePage(page, pageSize)
	total := len(items)
	start := (page - 1) * pageSize
	if start >= total {
		return Page[T]{
			Items:    []T{},
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		}
	}

	end := min(start+pageSize, total)
	return Page[T]{
		Items:    items[start:end],
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}
}

// normalizePage receives raw pagination values and returns bounded page and page-size values.
func normalizePage(page int, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	return page, pageSize
}
