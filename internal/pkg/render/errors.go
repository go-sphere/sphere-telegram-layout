package render

import (
	"errors"
	"net/http"
	"strings"

	"buf.build/go/protovalidate"
	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/conv"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent"
	"github.com/go-sphere/sphere/server/httpz"
)

func init() {
	httpz.SetDefaultErrorParser(func(err error) (int32, int32, string) {
		var ve *protovalidate.ValidationError
		if errors.As(err, &ve) {
			return ValidationError(ve)
		}
		var ne *ent.NotFoundError
		if errors.As(err, &ne) {
			return EntNotFoundError(ne)
		}
		var ce *ent.ConstraintError
		if errors.As(err, &ce) {
			return EntConstraintError(ce)
		}
		return httpx.ParseError(err)
	})
}

func ValidationError(err *protovalidate.ValidationError) (int32, int32, string) {
	return 0, http.StatusBadRequest, strings.Join(conv.Map(err.Violations, func(s *protovalidate.Violation) string {
		return s.Proto.GetMessage()
	}), ",")
}

func EntNotFoundError(*ent.NotFoundError) (int32, int32, string) {
	return 0, http.StatusNotFound, http.StatusText(http.StatusNotFound)
}

func EntConstraintError(*ent.ConstraintError) (int32, int32, string) {
	return 0, http.StatusBadRequest, http.StatusText(http.StatusBadRequest)
}
