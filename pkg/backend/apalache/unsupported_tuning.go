package apalache

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
)

// UnsupportedCheckOptionError identifies a setting accepted by the Go service's
// configuration parser but not applied by its current incremental CHECK path.
// Rejecting it avoids presenting an ignored search/resource setting as active.
type UnsupportedCheckOptionError struct {
	Option string
	Reason string
}

func (e *UnsupportedCheckOptionError) Error() string {
	return fmt.Sprintf("unsupported Go Apalache CHECK option %q: %s", e.Option, e.Reason)
}

// CheckConfig retains upstream merging behavior; this guard runs on its final
// effective configuration immediately before command execution. Malformed
// configuration shapes/values remain the strict service parser's responsibility.
func validateCheckTuning(config json.RawMessage) error {
	// Mirror the service's struct/map decoding: field names are folded and
	// repeated checker/tuning objects merge rather than replacing the object.
	// An exact-key JSON dictionary here would miss effective settings that the
	// service accepts through those spellings or repeated fields.
	var effective struct {
		Checker struct {
			Tuning map[string]*string `json:"tuning"`
		} `json:"checker"`
	}
	if err := json.Unmarshal(config, &effective); err != nil {
		return nil
	}
	// Native ParseCheckerTuning validates keys in lexical order too.
	keys := make([]string, 0, len(effective.Checker.Tuning))
	for key := range effective.Checker.Tuning {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		decoded := effective.Checker.Tuning[key]
		if decoded == nil {
			continue
		}
		value := *decoded
		reason := ""
		switch key {
		case "search.simulation":
			if value == "true" {
				reason = "random transition simulation (--random-transitions) is not implemented by the current incremental checker"
			}
		case "search.simulation.maxRun":
			if runs, err := strconv.Atoi(value); err == nil && runs > 0 {
				reason = "simulation run limits are not applied by the current incremental checker"
			}
		case "search.split.timeout.minimum":
			if _, err := strconv.ParseInt(value, 10, 64); err == nil {
				reason = "split/jail timeouts are not applied by the current incremental checker; use checker.timeout-smt for per-query deadlines"
			}
		case "search.split.timeout.factor":
			if _, err := strconv.ParseInt(value, 10, 32); err == nil {
				reason = "split/jail timeouts are not applied by the current incremental checker; use checker.timeout-smt for per-query deadlines"
			}
		case "search.idle.timeout":
			if _, err := strconv.ParseInt(value, 10, 64); err == nil {
				reason = "idle timeouts are not applied by the current incremental checker; use caller cancellation or checker.timeout-smt"
			}
		case "z3.rlimit":
			// Zero disables a limit and is safe to retain. g3's ordinary
			// CheckSat path does not push a positive resource-count budget.
			if limit, err := strconv.ParseUint(value, 10, 32); err == nil && limit > 0 && limit <= math.MaxInt32 {
				reason = "the linked g3 incremental CheckSat path does not apply a resource-count limit"
			}
		case "z3.reproducible_resource_limit", "z3.reproducible-resource-limit", "z3.smt.rlimit":
			// Generic text options try Boolean parsing before unsigned parsing;
			// Boolean spellings (including "1") are invalid for these UInt keys.
			if _, boolError := strconv.ParseBool(value); boolError != nil {
				if limit, err := strconv.ParseUint(value, 10, 32); err == nil && limit > 0 {
					reason = "the linked g3 incremental CheckSat path does not apply a resource-count limit"
				}
			}
		case "z3.sat.cardinality.encoding":
			switch value {
			case "grouped", "bimander", "ordered", "unate", "circuit":
				reason = "the linked g3 backend retains this compatibility setting without selecting an encoding"
			}
		}
		if reason != "" {
			return &UnsupportedCheckOptionError{Option: "checker.tuning." + key, Reason: reason}
		}
	}
	return nil
}
