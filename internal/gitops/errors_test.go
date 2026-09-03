package gitops

import (
	"strings"
	"testing"
)

func TestExitError_Error_IncludesArgvExitCodeAndStderr(t *testing.T) {
	err := &ExitError{
		Argv:     []string{"git", "rev-parse", "no-such-ref"},
		ExitCode: 128,
		Stderr:   "fatal: ambiguous argument 'no-such-ref'\n",
	}
	msg := err.Error()
	for _, want := range []string{"git rev-parse no-such-ref", "128", "ambiguous argument"} {
		if !strings.Contains(msg, want) {
			t.Errorf("Error() = %q, missing %q", msg, want)
		}
	}
}
