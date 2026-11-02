package quint

import (
	"encoding/json"
	"errors"
	"github.com/glycerine/insdict"
	"os"
	"testing"
)

func TestUpstreamVariableStorage(t *testing.T) {
	t.Parallel()
	type cell struct {
		Key   string
		Value *Expr
		Error *Diagnostic
	}
	var fixtures []struct {
		Operation     []string
		Current, Next []cell
		Record        Expr
	}
	data, err := os.ReadFile("testdata/parser/runtime_storage.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	storage := NewVariableStorage()
	snapshots := insdict.NewDict[string, VariableSnapshot]()
	for index, f := range fixtures {
		op := f.Operation
		switch op[0] {
		case "register":
			storage.Register(op[1], op[1])
		case "next":
			_, next := storage.Register(op[1], op[1])
			n, _ := ParseInteger(op[2])
			next.Value, _ = NewRuntimeValue(Expr{Kind: "int", Int: n})
		case "snapshot":
			snapshots.Set(op[1], storage.Snapshot())
		case "recover":
			storage.Recover(snapshots.Get(op[1]))
		case "shift":
			storage.Shift()
		case "reset":
			storage.Reset()
		}
		check := func(expected []cell, registers *insdict.Dict[string, *RuntimeRegister]) {
			t.Helper()
			for _, entry := range expected {
				value, err := registers.Get(entry.Key).Read()
				if entry.Error != nil {
					var d Diagnostic
					if !errors.As(err, &d) || d.Code != entry.Error.Code || d.Message != entry.Error.Message {
						t.Fatal(index, err, entry.Error)
					}
				} else {
					want, e := NewRuntimeValue(*entry.Value)
					if err != nil || e != nil || !value.Equal(want) {
						t.Fatal(index, value.String(), want.String(), err)
					}
				}
			}
		}
		check(f.Current, storage.current)
		check(f.Next, storage.next)
		got, err := storage.Record()
		want, e := NewRuntimeValue(f.Record)
		if err != nil || e != nil || !got.Equal(want) {
			t.Fatal(index, got.String(), want.String(), err)
		}
	}
}
func TestVariableStoragePointersCachesAndSnapshots(t *testing.T) {
	storage := NewVariableStorage()
	current, next := storage.Register("id", "display")
	again, againNext := storage.Register("id", "ignored")
	if again != current || againNext != next || again.Name != "display" {
		t.Fatal("register identity changed")
	}
	value, _ := NewRuntimeValue(Expr{Kind: "bool", Bool: true})
	next.Value = value
	cache := &RuntimeCache{Value: value, Err: Diagnostic{Code: "QNT501", Message: "cached"}}
	storage.TrackCache(cache)
	snapshot := storage.Snapshot()
	storage.Shift()
	if !current.Value.Equal(value) || next.Value.Valid() || cache.Value.Valid() || cache.Err != nil {
		t.Fatal("shift failed")
	}
	storage.Recover(snapshot)
	if !next.Value.Equal(value) || !current.Value.Equal(value) {
		t.Fatal("recover touched current")
	}
	next.Value = RuntimeValue{}
	storage.Recover(snapshot)
	if !next.Value.Equal(value) {
		t.Fatal("snapshot mutated")
	}
	cache.Value = value
	storage.Reset()
	if current.Value.Valid() || next.Value.Valid() || !cache.Value.Valid() {
		t.Fatal("reset contract")
	}
	if _, err := current.Read(); err == nil || err.Error() != "[QNT502] Variable display not set" {
		t.Fatal(err)
	}
}
