package output

import (
	"io"

	"github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/app"
	apperrors "github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/errors"
)

func Write(w io.Writer, format string, result *app.Result) error {
	switch format {
	case "json":
		return WriteJSON(w, result)
	case "text":
		return WriteText(w, result)
	default:
		return apperrors.New(apperrors.CodeInvalidConfig, "--output must be text or json")
	}
}
