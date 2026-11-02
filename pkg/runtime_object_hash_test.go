package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamRuntimeObjectHashes(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name           string
		InitialCounter uint32
		Steps          []struct {
			Kind    string
			Key     int
			Counter uint32
			Order   []int
		}
	}
	data, err := os.ReadFile("testdata/parser/runtime_object_hash.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			state := &RuntimeObjectHashState{}
			if err := state.Restore(fixture.InitialCounter); err != nil {
				t.Fatal(err)
			}
			objects := make([]runtimeObjectIdentity, 100)
			var set runtimeObjectSet[int]
			for i, step := range fixture.Steps {
				switch step.Kind {
				case "add":
					set.Add(state, &objects[step.Key], step.Key)
				case "hash":
					objects[step.Key].hashWith(state)
				case "restore":
					if err := state.Restore(step.Counter); err != nil {
						t.Fatal(err)
					}
				default:
					t.Fatal("unknown fixture operation", step.Kind)
				}
				order := []int{}
				for _, entry := range set.Entries() {
					order = append(order, entry.Value)
				}
				if state.Counter() != step.Counter || !reflect.DeepEqual(order, step.Order) {
					t.Fatal(i, "counter/order", state.Counter(), order, "want", step.Counter, step.Order)
				}
			}
		})
	}
}

func TestRuntimeObjectHashOwnershipAndRollover(t *testing.T) {
	state := &RuntimeObjectHashState{}
	if err := state.Restore(0x3fffffff); err != nil {
		t.Fatal(err)
	}
	if got := state.NextHash(); got != 0x40000000 || state.Counter() != 0 {
		t.Fatal("last UID was coerced or rollover was delayed", got, state.Counter())
	}
	object := &runtimeObjectIdentity{}
	if object.hashWith(state) != 1 || object.hashWith(state) != 1 || state.Counter() != 1 {
		t.Fatal("object identity did not cache its UID")
	}
	for _, invalid := range []uint32{0x40000000, 0xffffffff} {
		if err := state.Restore(invalid); err == nil || state.Counter() != 1 {
			t.Fatal("invalid counter accepted or changed state", invalid, state.Counter())
		}
	}
	var set runtimeObjectSet[int]
	set.Add(state, object, 7)
	view := set.Entries()
	view[0].Value = 9
	set.Add(state, object, 11)
	if set.Entries()[0].Value != 7 || state.Counter() != 1 {
		t.Fatal("set exposed storage or replaced duplicate identity")
	}
}
