package api

import (
	"net/http"

	"github.com/harsh3dev/multi-resume/backend/internal/fonts"
)

// listFonts handles GET /api/meta/fonts: the allowlist behind the Style tab's
// font-family select. Static data, so no store access.
func (d Deps) listFonts(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"fonts": fonts.Package})
}
