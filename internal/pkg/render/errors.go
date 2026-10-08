package render

import (
	"errors"
	"net/http"
	"strings"

	"buf.build/go/protovalidate"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/conv"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent"
	"github.com/go-sphere/sphere/server/httpz"
)

func init() {
	httpz.SetDefaultErrorParser(parseError)
}

// parseError is the process-wide httpz error parser: it maps validation, ent
// and request-body-cap errors, and leaves the rest to httpz.ParseError. That
// fallback matters: httpx.ParseError does not classify the storage sentinels
// (ErrNotFound, ErrDestExists, ErrFileNameInvalid), so a missing storage key
// would render as 500.
func parseError(err error) (int32, int32, string) {
	if ve, ok := errors.AsType[*protovalidate.ValidationError](err); ok {
		return ValidationError(ve)
	}
	if ne, ok := errors.AsType[*ent.NotFoundError](err); ok {
		return EntNotFoundError(ne)
	}
	if ce, ok := errors.AsType[*ent.ConstraintError](err); ok {
		return EntConstraintError(ce)
	}
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		return 0, http.StatusRequestEntityTooLarge, http.StatusText(http.StatusRequestEntityTooLarge)
	}
	return httpz.ParseError(err)
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
