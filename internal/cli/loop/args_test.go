package loop

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// execExpectError runs cmd like execCmd but returns its error instead of
// failing the test, for tests whose subject is a rejected invocation.
func execExpectError(cmd *cobra.Command, args []string) error {
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	return cmd.Execute()
}

// TestFixedArityCmds_ExtraArgs_Errors: with MinimumNArgs an under-quoted
// argument (`branch-name <repo> feat Fix the thing`) is accepted and the
// command silently acts on the truncated prefix. Args are valid against a
// real repo so only the arity check can produce the error.
func TestFixedArityCmds_ExtraArgs_Errors(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	head := strings.TrimSpace(runLoopGit(t, repoRoot, "rev-parse", "HEAD"))

	cases := []struct {
		name string
		cmd  *cobra.Command
		args []string
	}{
		{"branch-name under-quoted title", NewBranchNameCmd(), []string{repoRoot, "feat", "Fix", "the", "thing"}},
		{"commit-landed", NewCommitLandedCmd(), []string{repoRoot, "main", head, "extra"}},
		{"reset", NewResetCmd(), []string{repoRoot, "main", "extra"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := execExpectError(c.cmd, c.args); err == nil {
				t.Fatal("execute: expected an error for extra args, got nil")
			}
		})
	}
}
