package conv

import "golang.org/x/exp/constraints"

const (
	DefaultPageSize = 20
	MaxPageSize     = 500
)

// Page calculates pagination values based on total items and page size.
// It returns the number of pages needed and the effective page size to use.
// Non-positive page sizes fall back to DefaultPageSize, and values above
// MaxPageSize are clamped to MaxPageSize.
func Page[P constraints.Integer](total, pageSize P) (page P, size P) {
	if pageSize <= 0 {
		pageSize = max(1, DefaultPageSize)
	}
	if maxLimit := int64(MaxPageSize); uint64(pageSize) > uint64(maxLimit) {
		pageSize = P(maxLimit)
	}
	if total <= 0 {
		return 0, pageSize
	}
	page = total / pageSize
	if total%pageSize != 0 {
		page++
	}
	return page, pageSize
}
