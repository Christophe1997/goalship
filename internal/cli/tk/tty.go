package tk

import "os"

// isTerminal is a seam over the char-device check std Go CLIs use for tty
// detection (no golang.org/x/term dependency needed for this). Reassigned
// in tests together with stdinIsTTY/stdinAndStdoutAreTTY below, since the
// real process stdin/stdout are whatever `go test` happens to be attached
// to and can't be relied on to exercise both branches.
var isTerminal = func(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// stdinIsTTY gates add-note's stdin-pipe fallback (bash tk: `[[ ! -t 0 ]]`).
var stdinIsTTY = func() bool { return isTerminal(os.Stdin) }

// stdinAndStdoutAreTTY gates edit's $EDITOR launch (bash tk: `[ -t 0 ] &&
// [ -t 1 ]`) — both must be interactive, not just one.
var stdinAndStdoutAreTTY = func() bool { return isTerminal(os.Stdin) && isTerminal(os.Stdout) }
