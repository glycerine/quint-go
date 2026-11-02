package quint

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
)

// DebugMessage is a labelled concrete value emitted by q::debug in a trace.
type DebugMessage struct {
	Label string `json:"label"`
	Value Expr   `json:"value"`
}

// ITFDiagnostics returns diagnostic rows only for states whose metadata contains
// diagnostics, matching upstream diagnosticsOfItf. It does not insert empty rows
// for states without the field. All diagnostic expression IDs are zero.
func ITFDiagnostics(trace ITFTrace) ([][]DebugMessage, error) {
	rows := [][]DebugMessage{}
	for index, metadata := range trace.StateMetadata {
		value, present, err := itfMetadataField(metadata, "diagnostics")
		if err != nil {
			return nil, fmt.Errorf("ITF state %d diagnostics: %w", index, err)
		}
		if !present {
			continue
		}
		messages, err := decodeITFDebugMessages(value)
		if err != nil {
			return nil, fmt.Errorf("ITF state %d diagnostics: %w", index, err)
		}
		rows = append(rows, messages)
	}
	return rows, nil
}

// PendingITFDiagnostics extracts diagnostics from a failed step that produced no
// state. Missing or falsey pending metadata yields an empty list, as upstream.
func PendingITFDiagnostics(trace ITFTrace) ([]DebugMessage, error) {
	value, present, err := itfMetadataField(trace.Metadata, "pending_diagnostics")
	if err != nil {
		return nil, fmt.Errorf("ITF pending diagnostics: %w", err)
	}
	trimmed := bytes.TrimSpace(value)
	falsey := !present || bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte("false")) || bytes.Equal(trimmed, []byte(`""`))
	if len(trimmed) > 0 && (trimmed[0] == '-' || (trimmed[0] >= '0' && trimmed[0] <= '9')) {
		if number, ok := new(big.Rat).SetString(string(trimmed)); ok && number.Sign() == 0 {
			falsey = true
		}
	}
	if falsey {
		return []DebugMessage{}, nil
	}
	messages, err := decodeITFDebugMessages(value)
	if err != nil {
		return nil, fmt.Errorf("ITF pending diagnostics: %w", err)
	}
	return messages, nil
}

func itfMetadataField(metadata json.RawMessage, key string) (json.RawMessage, bool, error) {
	trimmed := bytes.TrimSpace(metadata)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, false, nil
	}
	if trimmed[0] != '{' || !json.Valid(trimmed) {
		return nil, false, fmt.Errorf("metadata must be a JSON object")
	}
	fields, err := itfObject(trimmed)
	if err != nil {
		return nil, false, err
	}
	value, ok := fields.Get2(key)
	return value, ok, nil
}

func decodeITFDebugMessages(value json.RawMessage) ([]DebugMessage, error) {
	var text *string
	if err := json.Unmarshal(value, &text); err != nil || text == nil {
		return nil, fmt.Errorf("diagnostics must be a JSON-encoded string")
	}
	var entries []json.RawMessage
	if err := json.Unmarshal([]byte(*text), &entries); err != nil {
		return nil, fmt.Errorf("invalid diagnostics JSON: %w", err)
	}
	if entries == nil {
		return nil, fmt.Errorf("diagnostics must encode an array")
	}
	output := []DebugMessage{}
	for index, entry := range entries {
		var message struct {
			Label *string         `json:"label"`
			Value json.RawMessage `json:"value"`
		}
		if err := json.Unmarshal(entry, &message); err != nil || message.Label == nil {
			return nil, fmt.Errorf("diagnostic %d requires a string label", index)
		}
		expression, err := decodeITFValue(message.Value, func() ID { return ID{} })
		if err != nil {
			return nil, fmt.Errorf("diagnostic %d: %w", index, err)
		}
		output = append(output, DebugMessage{Label: *message.Label, Value: expression})
	}
	return output, nil
}
