package model

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestOpsRoundTrip(t *testing.T) {
	fixtures := []string{
		`{"op":"edit","id":"f_1","value":"text"}`,
		`{"op":"editUrl","id":"f_1","url":"https://example.com"}`,
		`{"op":"add","listId":"l_1","after":null,"value":"text"}`,
		`{"op":"delete","id":"b_1"}`,
		`{"op":"reorder","parentId":"l_1","order":["b_2","b_1"]}`,
		`{"op":"move","id":"b_1","toEntryId":"e_1","after":"b_2"}`,
		`{"op":"hide","id":"b_1"}`,
		`{"op":"show","id":"b_1"}`,
		`{"op":"replaceImage","id":"a_1","assetId":"a_2"}`,
		`{"op":"setStyle","id":"st_1","value":"11pt"}`,
	}
	for _, fixture := range fixtures {
		var op Op
		if err := json.Unmarshal([]byte(fixture), &op); err != nil {
			t.Fatalf("%s: %v", fixture, err)
		}
		data, err := json.Marshal(op)
		if err != nil {
			t.Fatal(err)
		}
		var before, after any
		_ = json.Unmarshal([]byte(fixture), &before)
		_ = json.Unmarshal(data, &after)
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("op changed: %s -> %s", fixture, data)
		}
	}
	for _, invalid := range []string{`{"op":"eval","id":"b_1"}`, `{"op":"edit"}`, `{"op":"edit","id":"f_1","value":"x","surprise":true}`} {
		var op Op
		if err := json.Unmarshal([]byte(invalid), &op); err == nil {
			t.Fatalf("accepted %s", invalid)
		}
	}
}
