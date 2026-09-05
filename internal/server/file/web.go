package file

import (
	"strings"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/httpsrv"
	"github.com/go-sphere/sphere/server/httpz"
	"github.com/go-sphere/sphere/server/service/file"
	"github.com/go-sphere/sphere/storage"
	"github.com/go-sphere/sphere/storage/fileserver"
)

type Config struct {
	Address string   `json:"address" yaml:"address"`
	Cors    []string `json:"cors" yaml:"cors"`
	Debug   bool     `json:"debug" yaml:"debug"`
}

type UploadResponse struct {
	Key string `json:"key"`
	URL string `json:"url"`
}

// @Summary Upload file
// @Description Upload binary file content using the one-time `key` from the upload authorization URL, then return file key and accessible URL.
// @Tags shared.v1,file
// @Accept application/octet-stream
// @Produce json
// @Param key path string true "One-time upload authorization key"
// @Param body body string true "Binary file content"
// @Success 200 {object} UploadResponse
// @Failure 400 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /{key} [put]
func bindUploadRoute() { //
	panic("implement by others")
}

// @Summary Download file
// @Description Download an uploaded file by file path and return raw binary content.
// @Tags shared.v1,file
// @Produce application/octet-stream
// @Param filename path string true "File path (supports subpaths)"
// @Success 200 {file} file
// @Failure 404 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /{filename} [get]
func bindDownloadRoute() {
	panic("implement by others")
}

var _ = bindUploadRoute
var _ = bindDownloadRoute

// @Summary Generate debug upload auth
// @Description Generate one-time upload authorization for testing and return storage.UploadAuthResult-like data (authorization + file).
// @Tags shared.v1,file
// @Produce json
// @Param filename path string true "Original filename used to build upload target"
// @Success 200 {object} httpz.DataResponse[storage.UploadAuthResult]
// @Failure 400 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /debug/{filename} [post]
func bindDebugRoute(engine httpx.Engine, fileServer *fileserver.FileServer) {
	engine.Group("/").POST("/debug/:filename", func(context httpx.Context) error {
		filename := strings.TrimSpace(context.Param("filename"))
		if filename == "" {
			return httpx.NewBadRequestError("filename is required")
		}
		auth, err := fileServer.GenerateUploadAuth(context.Context(), storage.UploadAuthRequest{
			FileName: filename,
		})
		if err != nil {
			return httpx.InternalServerError(err)
		}
		return context.JSON(200, httpz.DataResponse[storage.UploadAuthResult]{
			Success: true,
			Data:    auth,
		})
	})
}

func NewWebServer(conf Config, storage *fileserver.FileServer) (*file.Web, error) {
	engine := httpsrv.NewGinServer("file", conf.Address)
	if err := httpsrv.UseCORS(engine, conf.Cors); err != nil {
		return nil, err
	}
	if conf.Debug {
		bindDebugRoute(engine, storage)
	}
	return file.NewWebServer(
		engine,
		storage,
	), nil
}
