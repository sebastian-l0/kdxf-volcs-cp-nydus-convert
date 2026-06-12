package output

import (
	"encoding/json"
	"io"

	"github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/app"
)

func WriteJSON(w io.Writer, result *app.Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
