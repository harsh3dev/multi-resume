package parser

import (
	_ "embed"
	"encoding/json"

	"github.com/harsh3dev/multi-resume/backend/internal/model"
)

// templates.json is the template dictionary: command meanings for the
// well-known resume classes (Jake's, AwesomeCV, moderncv, AltaCV, Deedy,
// latexcv). It is data, so supporting another template's commands is a JSON
// edit, not a code change.
//
//go:embed templates.json
var templatesJSON []byte

type templateCommand struct {
	Role  string       `json:"role"`
	Slots []model.Slot `json:"slots"`
	Open  string       `json:"open"`
}

type templateDictionary struct {
	ListEnvs []string                   `json:"listEnvs"`
	Commands map[string]templateCommand `json:"commands"`
}

var dictionary = func() templateDictionary {
	var d templateDictionary
	if err := json.Unmarshal(templatesJSON, &d); err != nil {
		panic("templates.json: " + err.Error())
	}
	return d
}()

func isListEnv(name string) bool {
	for _, e := range dictionary.ListEnvs {
		if e == name {
			return true
		}
	}
	return false
}

// dictionaryArgs expands a dictionary entry into per-argument info.
func (t templateCommand) args() []model.CommandArg {
	out := make([]model.CommandArg, len(t.Slots))
	for i, s := range t.Slots {
		out[i] = model.CommandArg{Index: i + 1, Use: "text", Slot: s}
	}
	return out
}
