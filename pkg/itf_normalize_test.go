package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestUpstreamITFNormalizationRuntimeHistory(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("testdata/parser/itf_normalization_runtime.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures struct {
		Cases []struct {
			Name                 string
			Input                json.RawMessage
			Output               []Expr
			InitialUID, FinalUID uint32
		}
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures.Cases {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			var objects RuntimeObjectHashState
			if err := objects.Restore(fixture.InitialUID); err != nil {
				t.Fatal(err)
			}
			trace, err := DecodeNormalizedITFTraceContext(context.Background(), fixture.Input, &objects)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(trace.States, fixture.Output) || objects.Counter() != fixture.FinalUID {
				got, _ := json.Marshal(trace.States)
				want, _ := json.Marshal(fixture.Output)
				t.Fatalf("Go UID%d %s\nTS UID%d %s", objects.Counter(), got, fixture.FinalUID, want)
			}
			if fixture.InitialUID == 0 {
				ordinary, err := DecodeNormalizedITFTrace(fixture.Input)
				if err != nil || !reflect.DeepEqual(ordinary.States, fixture.Output) {
					t.Fatal("fresh default state differs", err)
				}
			}
			if string(trace.Raw) != string(fixture.Input) || trace.Loop == nil || *trace.Loop != 0 || len(trace.Metadata) == 0 || !reflect.DeepEqual(trace.Params, []string{"N"}) {
				t.Fatal("trace envelope lost")
			}
			for _, metadata := range trace.StateMetadata {
				if !strings.Contains(string(metadata), `"keep"`) {
					t.Fatal("state metadata lost")
				}
			}
			before := append([]byte(nil), fixture.Input...)
			if len(trace.States) > 0 {
				trace.States[0].Args[0].Str = "changed"
			}
			if string(before) != string(fixture.Input) {
				t.Fatal("normalization changed input bytes")
			}
		})
	}
}

type itfNormalizeCancelContext struct {
	context.Context
	cancel        context.CancelFunc
	checks, limit int
}

func (ctx *itfNormalizeCancelContext) Err() error {
	ctx.checks++
	if ctx.limit > 0 && ctx.checks >= ctx.limit {
		ctx.cancel()
	}
	return ctx.Context.Err()
}

func TestITFNormalizationCancellationAndErrorPrecedence(t *testing.T) {
	t.Parallel()
	input := []byte(`{"states":[{"x":{"#set":[1,2]}},{"x":{"#set":[2,3]}}]}`)
	for _, limit := range []int{1, 2, 4, 8} {
		base, cancel := context.WithCancel(context.Background())
		ctx := &itfNormalizeCancelContext{Context: base, cancel: cancel, limit: limit}
		_, err := DecodeNormalizedITFTraceContext(ctx, input, nil)
		cancel()
		if !errors.Is(err, context.Canceled) {
			t.Fatal("cancellation lost", limit, err)
		}
	}
	base, cancel := context.WithCancel(context.Background())
	ctx := &itfNormalizeCancelContext{Context: base, cancel: cancel, limit: 2}
	if _, err := DecodeNormalizedITFTraceContext(ctx, []byte(`{"states":[]}`), nil); !errors.Is(err, context.Canceled) {
		t.Fatal("empty trace cancellation lost", err)
	}
	cancel()
	var objects RuntimeObjectHashState
	if err := objects.Restore(31); err != nil {
		t.Fatal(err)
	}
	bad := []byte(`{"states":[{"x":{"#set":[0,1,2,3,4,5,6,7,8]}},{"x":{"#unserializable":"Int"}}]}`)
	if _, err := DecodeNormalizedITFTraceContext(context.Background(), bad, &objects); err == nil || !strings.Contains(err.Error(), "state 1") || objects.Counter() != 31 {
		t.Fatal("serialized before every state was converted", objects.Counter(), err)
	}
	if _, err := DecodeNormalizedITFTraceContext(context.Background(), []byte(`null`), nil); err == nil {
		t.Fatal("invalid trace accepted")
	}
}
