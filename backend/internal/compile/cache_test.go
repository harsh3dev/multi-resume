package compile

import "testing"

func TestCacheKeyDeterministic(t *testing.T) {
	a := CacheKey("tex", map[string]string{"img/a.png": "h1", "img/b.png": "h2"}, "pdflatex")
	b := CacheKey("tex", map[string]string{"img/b.png": "h2", "img/a.png": "h1"}, "pdflatex")
	if a != b {
		t.Error("asset order must not change the key")
	}
	if len(a) != 64 {
		t.Errorf("key should be a sha256 hex string, got %q", a)
	}
}

func TestCacheKeySensitivity(t *testing.T) {
	base := map[string]string{"a.png": "h1"}
	cases := []struct {
		name     string
		tex      string
		assets   map[string]string
		compiler string
	}{
		{"tex change", "tex2", base, "pdflatex"},
		{"asset hash change", "tex", map[string]string{"a.png": "h2"}, "pdflatex"},
		{"asset added", "tex", map[string]string{"a.png": "h1", "b.png": "h3"}, "pdflatex"},
		{"asset path change", "tex", map[string]string{"b.png": "h1"}, "pdflatex"},
		{"compiler change", "tex", base, "xelatex"},
	}
	want := CacheKey("tex", base, "pdflatex")
	for _, tc := range cases {
		if got := CacheKey(tc.tex, tc.assets, tc.compiler); got == want {
			t.Errorf("%s must change the key", tc.name)
		}
	}
}
