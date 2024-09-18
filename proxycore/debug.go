package proxycore

type DebugLevel int

const (
	DebugInfo = DebugLevel(iota)
	DebugWarning
	DebugError
	DebugAll
	DebugTrace
)
