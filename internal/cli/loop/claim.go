package loop

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/gitops"
	"github.com/Christophe1997/goalship/internal/ledger"
	"github.com/Christophe1997/goalship/internal/ticket"
)

// NewClaimCmd claims a ticket for a run: gated on that run's ledger
// carrying an approved review with this ticket in its approved set (R16),
// then create-or-resume its branch and record the claim note. Mirrors
// loop_runner.py's cmd_claim, plus the run-scoped gate this Go port adds
// (the Python original predates review_state/approved_ticket_ids
// entirely, so it never needed a run identifier).
func NewClaimCmd() *cobra.Command {
	var runID string

	cmd := &cobra.Command{
		Use:   "claim <repo-root> <ticket-id> <branch-name> <base-ref> <trunk-branch>",
		Short: "Claim a ticket for the current run",
		Args:  cobra.ExactArgs(5),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, ticketID, branchName, baseRef, trunkBranch := args[0], args[1], args[2], args[3], args[4]

			if runID == "" {
				return fmt.Errorf("loop claim: --run-id is required")
			}

			state, err := ledger.LoadRunState(repoRoot, runID)
			if err != nil {
				return err
			}
			if state.ReviewState != ledger.ReviewStateApproved {
				return fmt.Errorf("loop claim: run %q is not approved (review_state = %q)", runID, state.ReviewState)
			}
			if !slices.Contains(state.ApprovedTicketIDs, ticketID) {
				return fmt.Errorf("loop claim: run %q is approved but ticket %q is not in its approved_ticket_ids", runID, ticketID)
			}

			exists, err := gitops.LocalBranchExists(repoRoot, branchName)
			if err != nil {
				return fmt.Errorf("loop claim: %w", err)
			}
			if exists {
				// Crash recovery: a prior claim already created the branch
				// but crashed before writing the claim note. Check it back
				// out and retry from here instead of failing on "branch
				// already exists".
				if err := gitops.CheckoutBranch(repoRoot, branchName); err != nil {
					return fmt.Errorf("loop claim: %w", err)
				}
			} else if err := gitops.CreateBranch(repoRoot, branchName, baseRef); err != nil {
				return fmt.Errorf("loop claim: %w", err)
			}

			// Captured after checkout-or-create, so it's correct on both
			// paths: a brand-new branch's tip (== baseRef) or a
			// crash-recovery retry's already-advanced tip.
			claimSHA, err := gitops.HeadSHA(repoRoot, "HEAD")
			if err != nil {
				return fmt.Errorf("loop claim: %w", err)
			}

			if err := recordClaimNote(repoRoot, ticketID, branchName, baseRef, trunkBranch, claimSHA); err != nil {
				return fmt.Errorf("loop claim: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&runID, "run-id", "", "Run ID whose ledger gates this claim (required)")
	return cmd
}

// recordClaimNote mirrors reconciliation.py's record_claim_note -> tk_add_note:
// a "branch: "/"base: "/"claim_sha: " line block, base omitted when baseRef
// equals trunkBranch, appended to ticketID's notes.
func recordClaimNote(repoRoot, ticketID, branchName, baseRef, trunkBranch, claimSHA string) error {
	lines := []string{"branch: " + branchName}
	if baseRef != trunkBranch {
		lines = append(lines, "base: "+baseRef)
	}
	lines = append(lines, "claim_sha: "+claimSHA)

	ticketsDir := filepath.Join(repoRoot, ".tickets")
	path, err := ticket.Resolve(ticketsDir, ticketID)
	if err != nil {
		return err
	}
	t, err := ticket.Load(path)
	if err != nil {
		return err
	}
	t.Body = ticket.AppendNote(t.Body, strings.Join(lines, "\n"))
	return t.Save(path)
}
