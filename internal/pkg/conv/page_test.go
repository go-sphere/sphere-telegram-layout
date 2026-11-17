package conv

import "testing"

func TestPage(t *testing.T) {
	tests := []struct {
		name     string
		total    int
		pageSize int
		wantPage int
		wantSize int
	}{
		{name: "zero total zero size", total: 0, pageSize: 0, wantPage: 0, wantSize: DefaultPageSize},
		{name: "negative size falls back to default", total: 25, pageSize: -5, wantPage: 2, wantSize: DefaultPageSize},
		{name: "exact division", total: 40, pageSize: 20, wantPage: 2, wantSize: 20},
		{name: "remainder adds page", total: 41, pageSize: 20, wantPage: 3, wantSize: 20},
		{name: "clamps above max page size", total: 1001, pageSize: 9999, wantPage: 3, wantSize: MaxPageSize},
		{name: "negative total returns zero pages", total: -3, pageSize: 10, wantPage: 0, wantSize: 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPage, gotSize := Page(tt.total, tt.pageSize)
			if gotPage != tt.wantPage || gotSize != tt.wantSize {
				t.Fatalf("Page(%d, %d) = (%d, %d), want (%d, %d)",
					tt.total, tt.pageSize, gotPage, gotSize, tt.wantPage, tt.wantSize)
			}
		})
	}
	t.Run("narrow signed and wide unsigned types", func(t *testing.T) {
		if p, s := Page[int8](100, 50); p != 2 || s != 50 {
			t.Fatalf("Page[int8](100, 50) = (%d, %d), want (2, 50)", p, s)
		}
		if p, s := Page[uint64](1001, ^uint64(0)); p != 3 || s != MaxPageSize {
			t.Fatalf("Page[uint64](1001, max) = (%d, %d), want (3, %d)", p, s, MaxPageSize)
		}
	})
}
