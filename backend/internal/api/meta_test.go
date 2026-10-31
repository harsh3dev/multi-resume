package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/harsh3dev/multi-resume/backend/internal/fonts"
)

func TestListFonts(t *testing.T) {
	rec := httptest.NewRecorder()
	Deps{}.listFonts(rec, httptest.NewRequest(http.MethodGet, "/api/meta/fonts", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body struct {
		Fonts []fonts.Font `json:"fonts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Fonts) != len(fonts.Package) || body.Fonts[0].Value == "" || body.Fonts[0].Label == "" {
		t.Fatalf("unexpected fonts: %+v", body.Fonts)
	}
}
