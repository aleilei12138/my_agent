package agent

import "errors"

var (
	ErrMaxTurns = errors.New(
		"agent maximum turns exceeded",
	)

	ErrDuplicateToolCallID = errors.New(
		"duplicate tool call ID",
	)
)
