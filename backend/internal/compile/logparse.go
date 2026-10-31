package compile

import (
	"path"
	"regexp"
	"strings"
)

// maxErrorLen caps the client-facing error excerpt.
const maxErrorLen = 2000

// errorContextLines is how many lines after the "!" line are kept.
const errorContextLines = 6

var absPathRe = regexp.MustCompile(`(?:^|[\s("'` + "`" + `])(/[A-Za-z0-9._\-]+)+`)

// ParseLogError extracts the first LaTeX error ("!" line) plus a few lines of
// context from a latexmk log. Filesystem paths are stripped — the result is
// safe to show clients. Returns "" when the log contains no error line.
func ParseLogError(log string, stripDirs ...string) string {
	log = stripPaths(log, stripDirs)
	lines := strings.Split(log, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, "!") {
			continue
		}
		end := i + 1
		for end < len(lines) && end <= i+errorContextLines && !strings.HasPrefix(lines[end], "!") {
			end++
		}
		excerpt := strings.Join(lines[i:end], "\n")
		if len(excerpt) > maxErrorLen {
			excerpt = excerpt[:maxErrorLen]
		}
		return strings.TrimRight(excerpt, "\n")
	}
	return ""
}

// stripPaths removes the given directories, then rewrites any remaining
// absolute path token to its base name.
func stripPaths(s string, dirs []string) string {
	for _, d := range dirs {
		if d != "" {
			s = strings.ReplaceAll(s, d, "")
		}
	}
	return absPathRe.ReplaceAllStringFunc(s, func(m string) string {
		i := strings.Index(m, "/")
		return m[:i] + path.Base(m[i:])
	})
}
