package render

import (
	"errors"
	"net/http"
	"testing"

	"github.com/go-sphere/httpx"
)

func TestParseErrorMapsBodyCapToRequestEntityTooLarge(t *testing.T) {
	// A binder wraps the read error as a 400; the cap must still win.
	err := httpx.WrapBindError(&http.MaxBytesError{Limit: 64})
	if _, status, _ := parseError(err); status != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", status, http.StatusRequestEntityTooLarge)
	}
	if _, status, _ := parseError(httpx.WrapBindError(errors.New("bad json"))); status != http.StatusBadRequest {
		t.Fatalf("other bind error status = %d, want %d", status, http.StatusBadRequest)
	}
}
