package apalache

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCheckConfigSimulationSeed(t *testing.T) {
	compiled := compileSource(t, "module M { action init=true action step=true val safe=true }")
	seedOf := func(t *testing.T, options CheckOptions) string {
		t.Helper()
		config, err := CheckConfig(compiled, nil, options)
		if err != nil {
			t.Fatal(err)
		}
		var decoded struct {
			Checker struct {
				Tuning map[string]any `json:"tuning"`
			} `json:"checker"`
		}
		if err = json.Unmarshal(config, &decoded); err != nil {
			t.Fatal(err)
		}
		value, ok := decoded.Checker.Tuning["search.simulation.seed"]
		if !ok {
			return ""
		}
		return value.(string)
	}
	if got := seedOf(t, CheckOptions{MaxSteps: 2, RandomTransitions: true, RandomSeed: 42}); got != "42" {
		t.Fatalf("seed = %q, want 42", got)
	}
	if got := seedOf(t, CheckOptions{MaxSteps: 2, RandomTransitions: true}); got != "" {
		t.Fatalf("zero seed emitted as %q", got)
	}
	if got := seedOf(t, CheckOptions{MaxSteps: 2, RandomSeed: 42}); got != "" {
		t.Fatalf("seed emitted without random transitions: %q", got)
	}
	// A seed loaded from --apalache-config survives when no explicit seed is set.
	loaded := json.RawMessage(`{"checker":{"tuning":{"search.simulation.seed":"7"}}}`)
	config, err := CheckConfig(compiled, loaded, CheckOptions{RandomTransitions: true})
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Checker struct {
			Tuning map[string]any `json:"tuning"`
		} `json:"checker"`
	}
	if err = json.Unmarshal(config, &decoded); err != nil || decoded.Checker.Tuning["search.simulation.seed"] != "7" {
		t.Fatal("loaded seed lost", string(config), err)
	}
	if _, err := CheckConfig(compiled, nil, CheckOptions{RandomTransitions: true, RandomSeed: -1}); err == nil || !strings.Contains(err.Error(), "negative") {
		t.Fatal("negative seed accepted", err)
	}
}
