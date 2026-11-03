package tlc

import native "github.com/glycerine/tlago/tlc"

// ExitStatusForErrorCode groups these errors with violations. Their specific
// codes retain the distinction between a false predicate and an evaluation
// that failed before obtaining a Boolean result.
func evaluationFailure(code int) bool {
	switch code {
	case native.ECTLCAssumptionEvaluationError,
		native.ECTLCPostconditionEvaluationError,
		native.ECTLCActionPropertyEvaluationFailed:
		return true
	default:
		return false
	}
}

func initialViolation(code int) bool {
	return code == native.ECTLCInvariantViolatedInitial || code == native.ECTLCPropertyViolatedInitial
}
