// Package fonts is the single source of truth for the fonts a resume's
// font-family style setting may be switched to. The frontend renders its
// select from GET /api/meta/fonts, the parser recognises these packages as the
// fontFamily setting, and setStyle rejects anything else for such a setting.
package fonts

// Font is one selectable \usepackage-style font.
type Font struct {
	// Value is the LaTeX package name written into \usepackage{...}.
	Value string `json:"value"`
	// Label is the human-readable name shown in the UI.
	Label string `json:"label"`
}

// Package lists fonts loadable with \usepackage under pdfLaTeX. Every entry
// ships with the TeX Live packages the production image installs
// (texlive-latex-base for times/helvet, texlive-fonts-recommended for
// charter/mathpazo/lmodern, texlive-fonts-extra for libertine).
//
// fontspec (\setmainfont, XeLaTeX/LuaLaTeX) names depend on system fonts and
// are deliberately not listed: they are not verified against the image.
var Package = []Font{
	{Value: "charter", Label: "Charter"},
	{Value: "mathpazo", Label: "Palatino"},
	{Value: "times", Label: "Times"},
	{Value: "lmodern", Label: "Latin Modern"},
	{Value: "libertine", Label: "Libertine"},
	{Value: "helvet", Label: "Helvetica (sans only)"},
}

// IsPackage reports whether name is a listed package font.
func IsPackage(name string) bool {
	for _, f := range Package {
		if f.Value == name {
			return true
		}
	}
	return false
}
