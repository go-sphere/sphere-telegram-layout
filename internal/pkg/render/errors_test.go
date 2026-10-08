package render

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/server/httpz"
	"github.com/go-sphere/sphere/storage/storageerr"
)

// TestParseErrorStatuses pins the status the parser installed in init() reports
// for the errors a handler returns, which is the status the client receives.
func TestParseErrorStatuses(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			// A binder wraps the read error as a 400; the cap must still win.
			name: "body cap",
			err:  httpx.WrapBindError(&http.MaxBytesError{Limit: 64}),
			want: http.StatusRequestEntityTooLarge,
		},
		{
			name: "other bind error",
			err:  httpx.WrapBindError(fmt.Errorf("bad json")),
			want: http.StatusBadRequest,
		},
		{
			// A storage sentinel carries no status of its own: only
			// httpz.ParseError classifies it, httpx.ParseError answers 500.
			name: "storage not found",
			err:  storageerr.ErrNotFound,
			want: http.StatusNotFound,
		},
		{
			name: "wrapped storage destination exists",
			err:  fmt.Errorf("move upload: %w", storageerr.ErrDestExists),
			want: http.StatusBadRequest,
		},
		{
			name: "storage invalid filename",
			err:  storageerr.ErrFileNameInvalid,
			want: http.StatusBadRequest,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := httpz.ErrorStatus(tt.err); got != tt.want {
				t.Fatalf("ErrorStatus(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}
