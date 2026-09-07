package bot

import (
	"context"
	"testing"

	botv1 "github.com/go-sphere/sphere-telegram-layout/api/bot/v1"
)

func TestCatalogPages(t *testing.T) {
	srv := &Service{}
	totalPages := (int64(len(CatalogEntries)) + catalogPageSize - 1) / catalogPageSize

	t.Run("page zero returns the first page", func(t *testing.T) {
		resp, err := srv.Catalog(t.Context(), &botv1.CatalogRequest{Page: 0})
		if err != nil {
			t.Fatalf("Catalog() error = %v", err)
		}
		if resp.Page != 0 || resp.TotalPages != totalPages || len(resp.Items) != 3 {
			t.Fatalf("unexpected response: %+v", resp)
		}
		if resp.Items[0].Id != 1 {
			t.Fatalf("first item = %d, want 1", resp.Items[0].Id)
		}
	})

	t.Run("negative page clamps to zero", func(t *testing.T) {
		resp, err := srv.Catalog(t.Context(), &botv1.CatalogRequest{Page: -1})
		if err != nil {
			t.Fatalf("Catalog() error = %v", err)
		}
		if resp.Page != 0 {
			t.Fatalf("page = %d, want 0", resp.Page)
		}
	})

	t.Run("oversized page clamps to the last page", func(t *testing.T) {
		resp, err := srv.Catalog(t.Context(), &botv1.CatalogRequest{Page: 99})
		if err != nil {
			t.Fatalf("Catalog() error = %v", err)
		}
		if resp.Page != totalPages-1 {
			t.Fatalf("page = %d, want %d", resp.Page, totalPages-1)
		}
	})
}

func TestCatalogItemBounds(t *testing.T) {
	srv := &Service{}

	t.Run("count clamps into bounds", func(t *testing.T) {
		for in, want := range map[int64]int64{0: 1, 1: 1, 3: 3, 99: 5} {
			resp, err := srv.CatalogItem(context.Background(), &botv1.CatalogItemRequest{Id: 1, Page: 0, Count: in})
			if err != nil {
				t.Fatalf("CatalogItem(%d) error = %v", in, err)
			}
			if resp.Count != want {
				t.Fatalf("count = %d, want %d (input %d)", resp.Count, want, in)
			}
		}
	})

	t.Run("unknown id errors", func(t *testing.T) {
		_, err := srv.CatalogItem(t.Context(), &botv1.CatalogItemRequest{Id: 99})
		if err == nil {
			t.Fatal("expected an error for an unknown id")
		}
	})
}
