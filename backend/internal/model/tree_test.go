package model

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestSectionItemsRoundTrip(t *testing.T) {
	items := []SectionItem{
		{Entry: &Entry{Kind: "entry", ID: "e_1", Fields: []Field{}, Bullets: nil}},
		{Bullets: &BulletList{Kind: "bullets", ID: "l_1", Bullets: []Bullet{}}},
		{Skill: &SkillLine{Kind: "skill", ID: "sk_1"}},
		{Paragraph: &Paragraph{Kind: "paragraph", ID: "p_1"}},
		{Opaque: &Opaque{Kind: "opaque", ID: "o_1", Reason: "unknown"}},
	}
	for _, item := range items {
		data, err := json.Marshal(item)
		if err != nil {
			t.Fatal(err)
		}
		var decoded SectionItem
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(item, decoded) {
			t.Fatalf("item changed: %s", data)
		}
	}
	var bad SectionItem
	if err := json.Unmarshal([]byte(`{"kind":"unrecognized"}`), &bad); err == nil {
		t.Fatal("accepted unknown kind")
	}
}

func TestExampleSectionRoundTrip(t *testing.T) {
	// The complete section object from the data-model §2.2, with its example spans.
	const fixture = `{"id":"s_k2p9","span":[1510,1890],"hidden":false,"title":{"id":"f_a1","value":"Experience","mode":"text","span":[1519,1529]},"items":[{"kind":"entry","id":"e_x7q2","span":[1560,1870],"hidden":false,"command":"\\resumeSubheading","fields":[{"id":"f_b2","value":"Groww","mode":"text","span":[1590,1595],"slot":"bold"},{"id":"f_b3","value":"Bengaluru, India","mode":"text","span":[1597,1613],"slot":"right"},{"id":"f_b4","value":"Software Development Engineer 1","mode":"text","span":[1622,1653],"slot":"italic"},{"id":"f_b5","value":"Sep 2025 – Present","mode":"text","span":[1655,1674],"slot":"right"}],"bullets":{"kind":"bullets","id":"l_m3","span":[1682,1866],"open":"\\resumeItemListStart","close":"\\resumeItemListEnd","bulletCommand":"\\resumeItem","bullets":[{"id":"b_r8t1","span":[1710,1778],"hidden":false,"text":{"value":"Reduced API latency by 34% using **Redis** caching","mode":"text","span":[1721,1776]}},{"id":"b_w4n6","span":[1787,1840],"hidden":true,"text":{"value":"Wrote internal docs for the rates service","mode":"text","span":[1800,1839]}}]}}]}`
	var section Section
	if err := json.Unmarshal([]byte(fixture), &section); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(section)
	if err != nil {
		t.Fatal(err)
	}
	var before, after any
	if err := json.Unmarshal([]byte(fixture), &before); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &after); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("section changed: %s", data)
	}
}
