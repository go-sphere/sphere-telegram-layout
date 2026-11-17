package conv

import (
	"cmp"
	"slices"
)

// UniqueSorted removes duplicates from a slice and returns a sorted copy.
// Zero values are excluded from the result. The original slice is not modified.
// This function is useful for creating clean, deduplicated lists.
func UniqueSorted[T cmp.Ordered](origin []T) []T {
	var zero T
	seen := make(map[T]struct{})
	result := make([]T, 0, len(origin))
	for _, v := range origin {
		if v == zero {
			continue
		}
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			result = append(result, v)
		}
	}
	slices.Sort(result)
	return result
}
