package shared

import (
	"net/http"
	"testing"

	"github.com/go-sphere/httpx"
	sharedv1 "github.com/go-sphere/sphere-telegram-layout/api/shared/v1"
)

func TestUploadTokenRejectsEmptyFilename(t *testing.T) {
	_, err := NewService(nil, "user").UploadToken(t.Context(), &sharedv1.UploadTokenRequest{})
	if err == nil {
		t.Fatal("UploadToken(empty filename) error = nil")
	}
	if _, status, message := httpx.ParseError(err); status != http.StatusBadRequest || message != "filename is required" {
		t.Fatalf("UploadToken(empty filename) = %d %q, want 400 \"filename is required\"", status, message)
	}
}
