// Package loop implements goalship's execution-loop mechanics — a Go port
// of loop_runner.py invoked as `goalship loop <subcommand>`.
package loop

import "github.com/spf13/cobra"

// NewCmd returns the "loop" parent command with every loop-mechanics
// subcommand attached. The caller (internal/cli/root.go) assigns its
// cobra.Group.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "loop",
		Short: "Execution-loop mechanics: ledger, git/gh, and PR operations",
	}

	cmd.AddCommand(
		NewPreflightCmd(),
		NewReconcileCmd(),
		NewLedgerCmd(),
		NewResumeCandidatesCmd(),
		NewDirtyCmd(),
		NewBranchNameCmd(),
		NewResolveBaseCmd(),
		NewCommitLandedCmd(),
		NewRunBranchCmd(),
		NewFindPRCmd(),
		NewClaimCmd(),
		NewCommitCmd(),
		NewHeadSHACmd(),
		NewPushCmd(),
		NewCreatePRCmd(),
		NewRetargetPRCmd(),
		NewShipCmd(),
		NewResetCmd(),
	)

	return cmd
}
