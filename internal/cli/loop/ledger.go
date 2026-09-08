package loop

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/ledger"
)

func NewLedgerCmd() *cobra.Command {
	var runID, claimID, goal, ticketMode, trunkBranch, terminal string
	var ship, fail bool

	cmd := &cobra.Command{
		Use:   "ledger <repo-root>",
		Short: "Read or update the run ledger",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot := args[0]
			flags := cmd.Flags()

			if flags.Changed("ticket-mode") && !ledger.ValidTicketMode(ticketMode) {
				return fmt.Errorf("loop ledger: --ticket-mode must be %q or %q, got %q",
					ledger.TicketModeBranch, ledger.TicketModeCommit, ticketMode)
			}
			if flags.Changed("terminal") && !ledger.ValidTerminalState(terminal) {
				return fmt.Errorf("loop ledger: --terminal must be one of %q, %q, %q, %q, %q, got %q",
					ledger.TerminalExhausted, ledger.TerminalDeadlocked, ledger.TerminalCapped,
					ledger.TerminalUserStop, ledger.TerminalAborted, terminal)
			}

			if err := ledger.EnsureExcluded(repoRoot); err != nil {
				return err
			}

			// "run_id or generate_run_id()": an explicit but empty
			// --run-id is treated the same as not passing it at all.
			effectiveRunID := runID
			if effectiveRunID == "" {
				id, err := ledger.GenerateRunID()
				if err != nil {
					return err
				}
				effectiveRunID = id
			}
			state, err := ledger.LoadRunState(repoRoot, effectiveRunID)
			if err != nil {
				return err
			}

			// Mutation order mirrors loop_runner.py's cmd_ledger exactly:
			// claim, then ship, then fail, then goal/ticket-mode/
			// trunk-branch overwrites, then terminal — --ship and --fail
			// both touch consecutive_failures, so this order matters if
			// they're ever combined with --terminal.
			if claimID != "" { // Python: "if claim_id" (truthy, not "is not None")
				state.ClaimTicket(claimID)
			}
			if ship {
				state.RecordShip()
			}
			if fail {
				state.RecordFailure()
			}
			if flags.Changed("goal") {
				state.Goal = goal
			}
			if flags.Changed("ticket-mode") {
				state.TicketMode = &ticketMode
			}
			if flags.Changed("trunk-branch") {
				state.TrunkBranch = &trunkBranch
			}
			if flags.Changed("terminal") {
				if err := state.MarkTerminal(terminal); err != nil {
					return err
				}
			}

			if err := state.Save(repoRoot); err != nil {
				return err
			}

			data, err := state.BytesWithCapsExceeded()
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	cmd.Flags().StringVar(&runID, "run-id", "", "Existing run ID to load (default: generate a new one)")
	cmd.Flags().StringVar(&claimID, "claim", "", "Ticket ID to claim")
	cmd.Flags().BoolVar(&ship, "ship", false, "Record a successful ship")
	cmd.Flags().BoolVar(&fail, "fail", false, "Record a gate failure")
	cmd.Flags().StringVar(&goal, "goal", "", "Set the run's goal text")
	cmd.Flags().StringVar(&ticketMode, "ticket-mode", "", "Set ticket mode (branch|commit)")
	cmd.Flags().StringVar(&trunkBranch, "trunk-branch", "", "Set the trunk branch name")
	cmd.Flags().StringVar(&terminal, "terminal", "", "Mark the run terminal with this reason")

	return cmd
}
