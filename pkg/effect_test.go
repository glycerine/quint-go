package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestEffectSchemas(t *testing.T) {
	samples := []string{`{"kind":"concrete","components":[]}`, `{"kind":"variable","name":"e"}`, `{"kind":"arrow","params":[],"result":{"kind":"concrete","components":[{"kind":"read","entity":{"kind":"concrete","stateVariables":[{"name":"x","reference":123456789012345678901234567890}]}},{"kind":"update","entity":{"kind":"union","entities":[{"kind":"variable","name":"v","reference":1}]}},{"kind":"temporal","entity":{"kind":"union","entities":[]}}]}}`}
	for _, s := range samples {
		var e Effect
		if err := json.Unmarshal([]byte(s), &e); err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		var got Effect
		if err = json.Unmarshal(data, &got); err != nil || !reflect.DeepEqual(e, got) {
			t.Fatalf("round trip: %s %v", data, err)
		}
	}
	for _, s := range []string{`null`, `{"kind":"arrow","params":[]}`, `{"kind":"concrete"}`, `{"kind":"variable","name":""}`, `{"kind":"concrete","components":[{"kind":"bad","entity":{"kind":"union","entities":[]}}]}`, `{"kind":"concrete","components":[{"kind":"read","entity":{"kind":"variable","name":"x","entities":[]}}]}`} {
		var e Effect
		if json.Unmarshal([]byte(s), &e) == nil {
			t.Errorf("accepted %s", s)
		}
	}
}
func TestUpstreamEffectTable(t *testing.T) {
	data, err := os.ReadFile("../evaluator/fixtures/simple.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Effects map[ID]EffectScheme `json:"effects"`
	}
	if err = json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Effects) == 0 {
		t.Fatal("no effects exercised")
	}
	encoded, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Effects map[ID]EffectScheme `json:"effects"`
	}
	if err = json.Unmarshal(encoded, &got); err != nil || !reflect.DeepEqual(doc, got) {
		t.Fatalf("effects changed: %v", err)
	}
}
