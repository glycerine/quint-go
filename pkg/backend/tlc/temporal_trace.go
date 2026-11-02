package tlc

import (
	"fmt"
	"strconv"

	native "github.com/glycerine/tlago/tlc"
)

// temporalLoop decodes TLC's closing trace marker into the zero-based index of
// the state reached after the last concrete state. Safety traces have no marker.
// A stuttering marker repeats the last concrete state; its printed ordinal is a
// display position, rather than a back-edge target.
func temporalLoop(messages []native.Message, traceLength int) (*int, error) {
	if traceLength < 0 {
		return nil, fmt.Errorf("TLC trace has negative length")
	}
	var loop *int
	markerCode := 0
	for _, message := range messages {
		if message.FormattingOnly {
			continue
		}
		var target int
		switch message.Code {
		case native.ECTLCStatePrint3:
			if traceLength == 0 {
				return nil, fmt.Errorf("TLC stuttering marker has no concrete state")
			}
			target = traceLength - 1
		case native.ECTLCBackToState:
			ordinal, present := message.StateNumber, message.StateInfo != nil || message.StateNumber != 0
			if len(message.Params) > 0 {
				parsed, err := strconv.Atoi(message.Params[0])
				if err != nil {
					return nil, fmt.Errorf("TLC back-to-state ordinal %q is invalid", message.Params[0])
				}
				if present && parsed != ordinal {
					return nil, fmt.Errorf("TLC back-to-state ordinals disagree: %d and %d", ordinal, parsed)
				}
				ordinal, present = parsed, true
			}
			if !present || ordinal < 1 || ordinal > traceLength {
				return nil, fmt.Errorf("TLC back-to-state ordinal %d is outside trace of length %d", ordinal, traceLength)
			}
			target = ordinal - 1
		default:
			continue
		}
		if loop != nil && (*loop != target || markerCode != message.Code) {
			return nil, fmt.Errorf("TLC trace has conflicting closing markers")
		}
		loop = &target
		markerCode = message.Code
	}
	return loop, nil
}
