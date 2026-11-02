package quint

import (
	"encoding/json"
	"math/big"
	"testing"
)

func TestLosslessIntegersAndIDs(t *testing.T) {
	const huge = "12345678901234567890123456789012345678901234567890"
	for _, s := range []string{"0", "-1", huge, "-" + huge} {
		var n Integer
		if err := json.Unmarshal([]byte(s), &n); err != nil {
			t.Fatal(err)
		}
		n.Big().SetInt64(42)
		b, err := json.Marshal(n)
		if err != nil || string(b) != s {
			t.Fatalf("round trip %s: %s %v", s, b, err)
		}
	}
	id, err := ParseID(huge)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(map[ID]string{id: "definition"})
	if err != nil {
		t.Fatal(err)
	}
	var got map[ID]string
	if err := json.Unmarshal(b, &got); err != nil || got[id] != "definition" {
		t.Fatalf("map round trip: %v %v", got, err)
	}
	zero, _ := ParseID("0")
	if zero != (ID{}) {
		t.Fatal("noncanonical zero ID")
	}
	source := big.NewInt(7)
	n := IntegerFromBig(source)
	source.SetInt64(9)
	if n.String() != "7" {
		t.Fatal("aliased input")
	}
}
func TestInvalidIntegerJSON(t *testing.T) {
	for _, s := range []string{`null`, `"12"`, `1.5`, `1e3`, `01`, `+1`, `-`} {
		var n Integer
		if json.Unmarshal([]byte(s), &n) == nil {
			t.Fatalf("accepted %s", s)
		}
	}
	var id ID
	if json.Unmarshal([]byte(`-1`), &id) == nil {
		t.Fatal("negative ID")
	}
}
func TestIDGenerator(t *testing.T) {
	g := NewIDGenerator()
	for _, want := range []string{"1", "2", "3"} {
		if got := g.Next().String(); got != want {
			t.Fatalf("got %s want %s", got, want)
		}
	}
}
