package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
)

// Op is decoded strictly. JSON fields are preserved as the documented union.
type Op struct {
	Kind      string   `json:"op"`
	ID        string   `json:"id,omitempty"`
	Value     string   `json:"value,omitempty"`
	URL       string   `json:"url,omitempty"`
	ListID    string   `json:"listId,omitempty"`
	After     *string  `json:"after"`
	ParentID  string   `json:"parentId,omitempty"`
	Order     []string `json:"order,omitempty"`
	ToEntryID string   `json:"toEntryId,omitempty"`
	AssetID   string   `json:"assetId,omitempty"`
}

func (o Op) MarshalJSON() ([]byte, error) {
	type plain Op
	if o.Kind == "add" || o.Kind == "move" {
		return json.Marshal(plain(o))
	}
	// Other operations have no nullable after key.
	var fields map[string]any
	data, err := json.Marshal(plain(o))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	delete(fields, "after")
	return json.Marshal(fields)
}

func (o *Op) UnmarshalJSON(data []byte) error {
	type plain Op
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	var result plain
	if err := d.Decode(&result); err != nil {
		return err
	}
	*o = Op(result)
	return o.Validate()
}
func (o Op) Validate() error {
	need := func(s string) bool { return strings.TrimSpace(s) != "" }
	switch o.Kind {
	case "edit", "setStyle":
		if need(o.ID) && need(o.Value) {
			return nil
		}
	case "editUrl":
		if need(o.ID) && need(o.URL) {
			return nil
		}
	case "add":
		if need(o.ListID) && need(o.Value) {
			return nil
		}
	case "delete", "hide", "show":
		if need(o.ID) {
			return nil
		}
	case "reorder":
		if need(o.ParentID) && len(o.Order) > 0 {
			for _, id := range o.Order {
				if !need(id) {
					return errors.New("invalid order id")
				}
			}
			return nil
		}
	case "move":
		if need(o.ID) && need(o.ToEntryID) {
			return nil
		}
	case "replaceImage":
		if need(o.ID) && need(o.AssetID) {
			return nil
		}
	}
	return errors.New("invalid operation")
}
