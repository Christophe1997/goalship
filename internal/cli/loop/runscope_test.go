package loop

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/ledger"
)

type runScopedCase struct {
	name string
	cmd  func() *cobra.Command
	args []string
}

// runScopedCmds lists every run-scoped loop command (KTD8) with otherwise
// valid args, so only the --run-id handling can make it fail.
func runScopedCmds(repoRoot string) []runScopedCase {
	return []runScopedCase{
		{"claim", NewClaimCmd, []string{repoRoot, "T-1", "feat/x", "main", "main"}},
		{"ship", NewShipCmd, []string{repoRoot, "T-1", "feat/x", "https://example.com/pull/1", "deadbeef"}},
		{"reconcile", NewReconcileCmd, []string{repoRoot}},
		{"resolve-base", NewResolveBaseCmd, []string{repoRoot, "T-1", "main"}},
		{"run-branch", NewRunBranchCmd, []string{repoRoot, "T-1"}},
	}
}

func TestRunScopedCmds_MissingRunID_ErrorNamesFlag(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	for _, c := range runScopedCmds(repoRoot) {
		t.Run(c.name, func(t *testing.T) {
			err := execExpectError(c.cmd(), c.args)
			if err == nil {
				t.Fatal("execute: expected an error without --run-id, got nil")
			}
			if !strings.Contains(err.Error(), "--run-id") {
				t.Errorf("error = %q, want it to name --run-id", err)
			}
		})
	}
}

func TestRunScopedCmds_UnknownRun_ErrorsAndCreatesNoLedger(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	for _, c := range runScopedCmds(repoRoot) {
		t.Run(c.name, func(t *testing.T) {
			err := execExpectError(c.cmd(), append(c.args, "--run-id", "no-such-run"))
			if err == nil {
				t.Fatal("execute: expected an error for an unknown run, got nil")
			}
			if _, statErr := os.Stat(ledger.ResolveLedgerPath(repoRoot, "no-such-run")); !errors.Is(statErr, os.ErrNotExist) {
				t.Errorf("ledger for the unknown run exists after the call (stat err = %v)", statErr)
			}
		})
	}
}
