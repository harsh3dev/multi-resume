package parser

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/harsh3dev/multi-resume/backend/internal/model"
)

// learnRoles classifies every custom command by the features of its fully
// expanded body (calls of other custom commands are substituted, several
// levels deep). A local definition always wins; the template dictionary
// only fills in commands the document does not define (their definitions
// live in a .cls file) or defines in a way the parser cannot read.
func learnRoles(defs []definition) {
	byName := make(map[string]*definition, len(defs))
	for i := range defs {
		byName[defs[i].Name] = &defs[i]
	}
	opt := latexOptions(defs)
	for i := range defs {
		d := &defs[i]
		body := expandBody(d.Body, byName, opt, maxExpandDepth)
		d.Role, d.Args = learnRole(d.Name, d.ArgCount, body)
		if d.Role == "other" {
			if t, ok := dictionary.Commands[d.Name]; ok && len(t.Slots) == d.ArgCount {
				d.Role, d.Args = t.Role, t.args()
			}
		}
	}
}

var beginEnvPattern = regexp.MustCompile(`\\(begin|end)\{([A-Za-z*]+)\}`)

// listBalance counts list-environment openings and closings in a body.
func listBalance(body string) (opens, closes int) {
	for _, m := range beginEnvPattern.FindAllStringSubmatch(body, -1) {
		if !isListEnv(m[2]) {
			continue
		}
		if m[1] == "begin" {
			opens++
		} else {
			closes++
		}
	}
	return
}

// unlabeledList reports a list whose items draw no bullet (label={}): in
// resume templates this is a layout container for entries, not a bullet list.
var unlabeledList = regexp.MustCompile(`label\s*=\s*(\{\s*\}|\\null|,|\])`)

func learnRole(name string, count int, body string) (string, []model.CommandArg) {
	if count < 0 || count > 9 {
		return "other", []model.CommandArg{}
	}
	args := make([]model.CommandArg, 0, count)
	for i := 1; i <= count; i++ {
		placeholder := "#" + strconv.Itoa(i)
		arg := model.CommandArg{Index: i, Use: "unknown"}
		if !strings.Contains(body, placeholder) {
			arg.Use = "unused"
		} else if insideCommand(body, `\href`, placeholder, 0) {
			arg.Use = "url"
		} else if insideCommand(body, `\vspace`, placeholder, 0) || insideCommand(body, `\hspace`, placeholder, 0) {
			arg.Use = "length"
		} else {
			arg.Use = "text"
		}
		switch {
		case insideCommand(body, `\textbf`, placeholder, -1):
			arg.Slot = model.Bold
		case insideCommand(body, `\textit`, placeholder, -1), insideCommand(body, `\emph`, placeholder, -1):
			arg.Slot = model.Italic
		case afterMarker(body, placeholder, `\hfill`) || afterMarker(body, placeholder, "&"):
			arg.Slot = model.Right
		case i == 1:
			arg.Slot = model.Left
		default:
			arg.Slot = model.Plain
		}
		args = append(args, arg)
	}
	role := "other"
	nameLower := strings.ToLower(name)
	opens, closes := listBalance(body)
	switch {
	case hasLogic(body):
		role = "other" // conditionals are never evaluated or guessed
	case strings.Contains(body, `\section`):
		role = "section"
	case opens > closes:
		role = "listOpen"
		if unlabeledList.MatchString(body) {
			role = "structure"
		}
	case closes > opens:
		role = "listClose"
	case strings.Contains(body, `\item`) && count == 1 && args[0].Use == "text":
		role = "bullet"
	case count >= 2 && (strings.Contains(body, `\hfill`) || strings.Contains(body, "&") || strings.Contains(body, "tabular")):
		role = "entry"
	case (strings.Contains(body, `\textbf`) && strings.Contains(body, ":") && count >= 2) || strings.Contains(nameLower, "skill"):
		role = "skill"
	case strings.Contains(body, `\href`):
		role = "link"
	}
	return role, args
}

// Require a command argument to contain the placeholder; position 0 selects
// only the first argument (the URL of href). -1 checks any argument.
func insideCommand(body, command, placeholder string, position int) bool {
	for offset := 0; offset < len(body); {
		idx := strings.Index(body[offset:], command+"{")
		if idx < 0 {
			return false
		}
		start := offset + idx + len(command)
		depth, arg := 0, 0
		for j := start; j < len(body); j++ {
			switch body[j] {
			case '{':
				depth++
				if depth == 1 {
					start = j + 1
				}
			case '}':
				depth--
				if depth == 0 {
					if (position < 0 || arg == position) && strings.Contains(body[start:j], placeholder) {
						return true
					}
					arg++
					if position >= 0 && arg > position {
						break
					}
				}
			}
			if depth == 0 && j > start && body[j] != '{' && body[j] != '}' && body[j] != ' ' && body[j] != '\t' {
				break
			}
		}
		offset = start
	}
	return false
}

var whitespace = regexp.MustCompile(`\s+`)

func afterMarker(body, placeholder, marker string) bool {
	index := strings.Index(body, placeholder)
	if index < 0 {
		return false
	}
	left := whitespace.ReplaceAllString(body[:index], "")
	markerIndex := strings.LastIndex(left, marker)
	if markerIndex < 0 {
		return false
	}
	// A marker on the same row, rather than one on a previous row.
	return !strings.ContainsAny(body[markerIndex:index], "\n\\") || strings.HasSuffix(left, marker)
}
