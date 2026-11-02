package quint

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestUpstreamExpressions(t *testing.T) {
	paths, _ := filepath.Glob("../evaluator/fixtures/*.json")
	if os.Getenv("QUINT_CORPUS_TESTS") != "1" {
		paths = []string{"../evaluator/fixtures/simple.json"}
	}
	count := 0
	var visit func(json.RawMessage)
	visit = func(data json.RawMessage) {
		var obj map[string]json.RawMessage
		if json.Unmarshal(data, &obj) == nil && obj != nil {
			if expr, ok := obj["expr"]; ok {
				var e Expr
				if err := json.Unmarshal(expr, &e); err != nil {
					t.Fatal(err)
				}
				encoded, err := json.Marshal(e)
				if err != nil {
					t.Fatal(err)
				}
				var got Expr
				if err = json.Unmarshal(encoded, &got); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(e, got) {
					t.Fatal("expression changed on round trip")
				}
				count++
			}
			for _, v := range obj {
				visit(v)
			}
		} else {
			var items []json.RawMessage
			if json.Unmarshal(data, &items) == nil {
				for _, v := range items {
					visit(v)
				}
			}
		}
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		visit(data)
	}
	if count == 0 {
		t.Fatal("no expressions tested")
	}
	t.Logf("round-tripped %d expression roots", count)
}
func TestExpressionErrorsAndLargeLiteral(t *testing.T) {
	for _, s := range []string{`{"id":1,"kind":"bool"}`, `{"id":1,"kind":"bool","value":null}`, `{"id":1,"kind":"bool","value":false,"name":"bad"}`, `{"id":1,"kind":"unknown"}`} {
		var e Expr
		if json.Unmarshal([]byte(s), &e) == nil {
			t.Fatalf("accepted %s", s)
		}
	}
	var e Expr
	const number = "99999999999999999999999999999999999999"
	if err := json.Unmarshal([]byte(`{"id":1,"kind":"int","value":`+number+`}`), &e); err != nil {
		t.Fatal(err)
	}
	if e.Int.String() != number {
		t.Fatal("integer precision lost")
	}
}
