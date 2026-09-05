package shared

import (
	"context"
	"fmt"
	"path"
	"strconv"

	sharedv1 "github.com/go-sphere/sphere-telegram-layout/api/shared/v1"
	"github.com/go-sphere/sphere/storage"
)

var _ sharedv1.StorageServiceHTTPServer = (*Service)(nil)

func (s *Service) UploadToken(ctx context.Context, req *sharedv1.UploadTokenRequest) (*sharedv1.UploadTokenResponse, error) {
	if req.Filename == "" {
		return nil, fmt.Errorf("filename is required")
	}
	id, err := s.GetCurrentID(ctx)
	if err != nil {
		return nil, err
	}
	token, err := s.storage.GenerateUploadAuth(ctx, storage.UploadAuthRequest{
		FileName: req.Filename,
		Dir:      path.Join(s.storageDir, strconv.FormatInt(id, 10)),
	})
	if err != nil {
		return nil, err
	}
	return &sharedv1.UploadTokenResponse{
		Authorization: &sharedv1.UploadAuthorization{
			Type:    string(token.Authorization.Type),
			Value:   token.Authorization.Value,
			Method:  token.Authorization.Method,
			Headers: token.Authorization.Headers,
		},
		File: &sharedv1.UploadFileInfo{
			Key: token.File.Key,
			Url: token.File.URL,
		},
	}, nil
}
