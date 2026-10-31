package tailor

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PromptVersion is stamped into every tailor_runs row.
const PromptVersion = "v1"

// promptFor builds the system + user prompt per BUILD-PLAN §9.2.
func promptFor(view AIView, jd string) (system, user string) {
	system = `You are a resume tailoring assistant. You rewrite resume bullets to match a job description.

Hard rules — violating any of them makes the output useless:
- NEVER invent numbers, metrics, employers, titles, or dates. Every number you output must appear in the original text of that item.
- NEVER add skills, tools, or technologies that do not already appear somewhere in the resume.
- Respect each item's maxLen (character cap).
- Output ONLY JSON Lines: one object per line, no prose, no markdown fences.

Each line is one change: {"op":"edit","id":"<id>","value":"<new text>","why":"<one sentence>"}
Allowed ops: edit (change text), hide (id only), show (id only). Do not reorder or move unless clearly beneficial.
Items marked readOnly must never appear in your output. Items marked hidden are in the bullet bank — propose show for the relevant ones.`
	viewJSON, _ := json.Marshal(view)
	user = fmt.Sprintf("Resume items (JSON):\n%s\n\nJob description:\n%s", viewJSON, jd)
	return system, user
}

// parsedLine is one JSONL change line from the model.
type parsedLine struct {
	Op    string `json:"op"`
	ID    string `json:"id"`
	Value string `json:"value"`
	Why   string `json:"why"`
}

// parseLine parses one JSONL line into op + why. Unknown ops are errors.
func parseLine(line string) (*parsedLine, error) {
	line = strings.TrimSpace(line)
	if line == "" || !strings.HasPrefix(line, "{") {
		return nil, fmt.Errorf("not a json object")
	}
	dec := json.NewDecoder(strings.NewReader(line))
	dec.DisallowUnknownFields()
	var p parsedLine
	if err := dec.Decode(&p); err != nil {
		return nil, err
	}
	switch p.Op {
	case "edit":
		if p.ID == "" || p.Value == "" {
			return nil, fmt.Errorf("edit needs id and value")
		}
	case "hide", "show":
		if p.ID == "" {
			return nil, fmt.Errorf("%s needs id", p.Op)
		}
	default:
		return nil, fmt.Errorf("op %q not allowed", p.Op)
	}
	return &p, nil
}

// allowedForAI drops edits to read-only or unknown ids. Only edit/hide/show
// survive; add/delete/setStyle/reorder/move never come from the AI.
func allowedForAI(p *parsedLine, view AIView) bool {
	if p.Op == "hide" || p.Op == "show" {
		for _, it := range view.Items {
			if it.ID == p.ID {
				return true
			}
		}
		return false
	}
	// edit
	_, ok := view.Editable(p.ID)
	return ok
}
