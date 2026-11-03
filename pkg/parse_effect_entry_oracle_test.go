package quint

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamStandaloneEffectEntry(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name, Source, Policy string
		Upstream             struct {
			Effect json.RawMessage
			Errors []struct {
				Explanation string
				Locs        []UpstreamLocation
			}
		}
		Remaining *struct {
			Text             string
			Line, Col, Index int
		}
		Issues []SyntaxIssue
	}
	data, err := os.ReadFile("testdata/parser/parser_effect_entry.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 58 {
		t.Fatalf("standalone effect fixture inventory: got %d, want 58", len(fixtures))
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseEffect(fixture.Source)
			if len(fixture.Issues) > 0 {
				var syntax *SyntaxErrors
				if !errors.As(err, &syntax) {
					t.Fatalf("got %T %v, want syntax errors", err, err)
				}
				if syntax.Context != "effect" || syntax.Source != "" || !reflect.DeepEqual(syntax.Issues, fixture.Issues) {
					t.Fatalf("effect syntax errors:\n got %+v\nwant %+v", syntax, fixture.Issues)
				}
				if !reflect.DeepEqual(got, Effect{}) {
					t.Fatalf("failed parsing returned an effect: %+v", got)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				encoded, err := json.Marshal(got)
				if err != nil {
					t.Fatal(err)
				}
				decode := func(encoded []byte) any {
					decoder := json.NewDecoder(bytes.NewReader(encoded))
					decoder.UseNumber()
					var value any
					if err := decoder.Decode(&value); err != nil {
						t.Fatal(err)
					}
					return value
				}
				// Compare the complete transport effect, including duplicate state
				// occurrences and their unmodified per-parse references.
				if !reflect.DeepEqual(decode(encoded), decode(fixture.Upstream.Effect)) {
					t.Errorf("raw effect:\n got %s\nwant %s", encoded, fixture.Upstream.Effect)
				}
			}
			if fixture.Policy == "complete-input-boundary" {
				// Upstream's standalone entry may return a prefix effect. Go's
				// documented complete-input contract deliberately rejects its
				// first unconsumed token, retaining any earlier syntax issues.
				if fixture.Remaining == nil || len(fixture.Issues) == 0 || fixture.Issues[len(fixture.Issues)-1].Message != "unexpected trailing input" {
					t.Fatal("incomplete boundary fixture")
				}
			} else if fixture.Policy != "shared" || fixture.Remaining != nil || len(fixture.Issues) != len(fixture.Upstream.Errors) {
				t.Fatal("invalid shared-parser fixture")
			}
		})
	}
}
