package loop

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/ledger"
	"github.com/Christophe1997/goalship/internal/ticket"
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
			_, statErr := os.Stat(ledger.ResolveLedgerPath(repoRoot, effectiveRunID))
			isNewRun := errors.Is(statErr, os.ErrNotExist)
			state, err := ledger.LoadRunState(repoRoot, effectiveRunID)
			if err != nil {
				return err
			}
			if isNewRun && os.Getenv(ticket.TicketsDirEnv) == "" {
				if err := assignRunFolder(repoRoot, state); err != nil {
					return err
				}
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
			if err := writeRunSummary(repoRoot, state); err != nil {
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

// assignRunFolder records a brand-new run's ticket folder in its ledger
// (KTD3). Only `loop ledger`'s own run creation sets it, so a run's folder
// can never be repointed after tickets land in it. A folder that already
// holds run.json (a committed run whose git-excluded ledger is gone) seeds
// the recreated ledger's goal and outcome, so recovery never blanks them.
func assignRunFolder(repoRoot string, state *ledger.RunState) error {
	runDir, err := ticket.RunDir(ticket.ResolveTicketsDir(repoRoot), state.RunID)
	if err != nil {
		return fmt.Errorf("loop ledger: %w", err)
	}
	rel, err := filepath.Rel(repoRoot, runDir)
	if err != nil {
		return fmt.Errorf("loop ledger: %w", err)
	}
	rel = filepath.ToSlash(rel)
	state.TicketsDir = &rel

	existing, err := ticket.ReadRunSummary(runDir)
	switch {
	case err == nil:
		state.Goal = existing.Goal
		state.TerminalState = existing.Outcome
	case !errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("loop ledger: %w", err)
	}
	return nil
}

// writeRunSummary mirrors the ledger's goal and outcome into the run
// folder's run.json, creating the folder on first write. Goalship never
// stages or commits it (R12); the orchestrating skill decides that.
func writeRunSummary(repoRoot string, state *ledger.RunState) error {
	if state.TicketsDir == nil {
		return nil
	}
	runDir := filepath.Join(repoRoot, filepath.FromSlash(*state.TicketsDir))
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return fmt.Errorf("loop ledger: create %s: %w", runDir, err)
	}
	return ticket.WriteRunSummary(runDir, ticket.RunSummary{
		RunID:     state.RunID,
		Goal:      state.Goal,
		StartedAt: time.Now().UTC().Format(time.RFC3339),
		Outcome:   state.TerminalState,
	})
}
