package tailor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/harsh3dev/multi-resume/backend/internal/checks"
	"github.com/harsh3dev/multi-resume/backend/internal/model"
)

// Change is one proposed, validated change streamed to the browser.
type Change struct {
	Op    string   `json:"op"`
	ID    string   `json:"id"`
	Value string   `json:"value,omitempty"`
	Why   string   `json:"why"`
	Flags []string `json:"flags"`
}

// Deps are the run's external needs.
type Deps struct {
	// StreamText calls the model and invokes onLine for each completed JSONL
	// line of model output. Tests inject a fake.
	StreamText func(ctx context.Context, system, user string, onLine func(string)) error
	// Corpus is the whole-resume text corpus for the new-claims check.
	Corpus string
	// Now is injectable for tests.
	Now func() time.Time
}

// Run executes one tailoring run, streaming validated changes to w as one
// growing JSON document (BUILD-PLAN §9.3): {"changes":[…] opens immediately,
// each change is appended as its own JSON object, and the terminal tail
// closes with ],"done":true,"runId":"…"}. The stream is always a valid
// prefix of the final object. Run returns the full change list for
// persistence; runID is embedded in the terminal tail so the browser can
// write back accept/reject decisions.
func Run(ctx context.Context, w io.Writer, tree *model.Resume, jd string, runID string, deps Deps) ([]Change, error) {
	start := deps.Now()
	view := BuildAIView(tree)
	system, user := promptFor(view, jd)
	changes := []Change{}

	write := func(s string) {
		_, _ = io.WriteString(w, s)
		if f, ok := w.(interface{ Flush() }); ok {
			f.Flush()
		}
	}
	first := true
	writeChange := func(c Change) {
		if !first {
			write(",")
		}
		first = false
		data, _ := json.Marshal(c)
		write(string(data))
	}

	// One growing JSON document: every write only appends, so the stream is
	// always a valid prefix of the terminal object
	// {"changes":[…],"done":true,"runId":"…"}. Clients that accumulate the
	// text and partial-parse it (AI SDK useObject) watch the list fill in.
	write(`{"changes":[`)

	streamErr := deps.StreamText(ctx, system, user, func(line string) {
		p, err := parseLine(line)
		if err != nil {
			slog.Warn("tailor: dropped malformed line", "version", PromptVersion)
			return
		}
		if !allowedForAI(p, view) {
			slog.Warn("tailor: dropped op on locked/unknown id", "op", p.Op)
			return
		}
		change := Change{Op: p.Op, ID: p.ID, Value: p.Value, Why: p.Why, Flags: []string{}}
		if p.Op == "edit" {
			item, _ := view.Editable(p.ID)
			if !checks.CheckNumbers(item.Text, p.Value) {
				change.Flags = append(change.Flags, string(checks.FlagNumberChanged))
			}
			if claims := checks.CheckNewClaims(deps.Corpus, p.Value); len(claims) > 0 {
				change.Flags = append(change.Flags, string(checks.FlagNewClaim))
			}
			if !checks.CheckMaxLen(p.Value, item.MaxLen) {
				change.Flags = append(change.Flags, string(checks.FlagTooLong))
			}
		}
		changes = append(changes, change)
		writeChange(change)
	})

	// Terminal tail closes the document; the accumulated stream is now one
	// complete, valid JSON object.
	var tail strings.Builder
	tail.WriteString(`],"done":true`)
	if runID != "" {
		idJSON, _ := json.Marshal(runID)
		tail.WriteString(`,"runId":`)
		tail.Write(idJSON)
	}
	if streamErr != nil {
		tail.WriteString(`,"error":true`)
	}
	tail.WriteByte('}')
	write(tail.String())
	_ = start
	if streamErr != nil {
		return changes, fmt.Errorf("model stream failed: %w", streamErr)
	}
	return changes, nil
}
