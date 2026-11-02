package quint

import (
	"encoding/json"
	"github.com/glycerine/insdict"
	"os"
	"testing"
)

func TestUpstreamStorageMetadata(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Enabled  bool
		Op, Name string
		Value    *Expr
		Record   Expr
		Action   string
		Picks    []struct {
			Name  string
			Value *Expr
		}
	}
	data, err := os.ReadFile("testdata/parser/runtime_storage_metadata.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	var storage *VariableStorage
	var snapshots *insdict.Dict[string, VariableSnapshot]
	for index, f := range fixtures {
		if storage == nil || storage.metadata != f.Enabled {
			storage = NewVariableStorageWithMetadata(f.Enabled)
			snapshots = insdict.NewDict[string, VariableSnapshot]()
		}
		value := RuntimeValue{}
		if f.Value != nil {
			value, err = NewRuntimeValue(*f.Value)
			if err != nil {
				t.Fatal(err)
			}
		}
		switch f.Op {
		case "register":
			storage.Register(f.Name, f.Name)
		case "pick":
			storage.NondetPicks.Set(f.Name, value)
		case "action":
			storage.ActionTaken = f.Name
		case "next":
			storage.next.Get(f.Name).Value = value
		case "snapshot":
			snapshots.Set(f.Name, storage.Snapshot())
		case "recover":
			storage.Recover(snapshots.Get(f.Name))
		case "shift":
			storage.Shift()
		case "reset":
			storage.Reset()
		case "clear":
			storage.ClearMetadata()
		case "from":
			if err = storage.FromRecord(value); err != nil {
				t.Fatal(err)
			}
		}
		got, err := storage.Record()
		want, e := NewRuntimeValue(f.Record)
		if err != nil || e != nil || !got.Equal(want) || storage.ActionTaken != f.Action {
			t.Fatal(index, got.String(), want.String(), storage.ActionTaken, f.Action, err)
		}
		names := []string{}
		for name, pick := range storage.NondetPicks.All() {
			names = append(names, name)
			found := false
			for _, expected := range f.Picks {
				if expected.Name == name {
					found = true
					if expected.Value == nil {
						if pick.Valid() {
							t.Fatal(index, "expected unset pick")
						}
					} else {
						want, err := NewRuntimeValue(*expected.Value)
						if err != nil || !pick.Equal(want) {
							t.Fatal(index, pick.String(), want.String(), err)
						}
					}
				}
			}
			if !found {
				t.Fatal(index, "unexpected pick", name)
			}
		}
		if len(names) != len(f.Picks) {
			t.Fatal(index, "missing picks")
		}
	}
}

func TestStorageRestoreCachesAndMetadataOwnership(t *testing.T) {
	storage := NewVariableStorageWithMetadata(true)
	current, next := storage.Register("x", "x")
	n, _ := ParseInteger("7")
	value, _ := NewRuntimeValue(Expr{Kind: "int", Int: n})
	storage.NondetPicks.Set("p", value)
	storage.ActionTaken = "step"
	snapshot := storage.Snapshot()
	storage.NondetPicks.Set("p", RuntimeValue{})
	storage.ActionTaken = "changed"
	storage.Recover(snapshot)
	if !storage.NondetPicks.Get("p").Equal(value) || storage.ActionTaken != "step" {
		t.Fatal("metadata recovery failed")
	}
	storage.NondetPicks.Set("p", RuntimeValue{})
	storage.Recover(snapshot)
	if !storage.NondetPicks.Get("p").Equal(value) {
		t.Fatal("snapshot aliased")
	}
	cache := &RuntimeCache{Value: value}
	storage.TrackCache(cache)
	next.Value = value
	record, _ := NewRuntimeValue(Expr{Kind: "app", Opcode: "Rec", Args: []Expr{{Kind: "str", Str: "x"}, value.source}})
	if err := storage.FromRecord(record); err != nil || !current.Value.Equal(value) || next.Value.Valid() || cache.Value.Valid() {
		t.Fatal("restore contract", err)
	}
	if storage.ActionTaken != "" || storage.NondetPicks.Get("p").Valid() {
		t.Fatal("restore metadata reset")
	}
	for _, source := range []string{`1`, `Rec("mbt::actionTaken",1)`, `Rec("mbt::nondetPicks",1)`, `Rec("mbt::nondetPicks",Rec("p",1))`} {
		e, err := ParseExpression(source)
		if err != nil {
			t.Fatal(err)
		}
		bad, err := NewRuntimeValue(e)
		if err != nil {
			t.Fatal(err)
		}
		if err = storage.FromRecord(bad); err == nil {
			t.Fatal("invalid record accepted", source)
		}
	}
	// A registered variable wins over an identically named metadata field on import.
	collision := NewVariableStorageWithMetadata(true)
	cell, _ := collision.Register("id", ITFActionTaken)
	e, _ := ParseExpression(`Rec("mbt::actionTaken",17)`)
	record, _ = NewRuntimeValue(e)
	if err := collision.FromRecord(record); err != nil || cell.Value.String() != "17" || collision.ActionTaken != "" {
		t.Fatal(err, cell.Value.String())
	}
}
