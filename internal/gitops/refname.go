package gitops

import (
	"fmt"
	"strings"
)

// rejectFlagLikeRef guards an agent-supplied ref that git would otherwise
// parse as an option. A "--" separator can't do this uniformly: rev-parse and
// `checkout -b` read it as a pathspec boundary, not end-of-options. Git
// refuses to create a branch named with a leading "-", so one is never a
// real ref.
func rejectFlagLikeRef(role, ref string) error {
	if strings.HasPrefix(ref, "-") {
		return fmt.Errorf("gitops: %s %q must not start with '-'", role, ref)
	}
	return nil
}
