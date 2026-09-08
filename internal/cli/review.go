package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/ledger"
	"github.com/Christophe1997/goalship/internal/reviewserver"
)

// NewReviewCmd starts the browser-based ticket-graph review checkpoint for
// a run. Unlike tk/loop, it has no subcommands of its own — <run-id> is a
// direct argument to a single action. This command owns the
// existence/already-approved refusal (R9) — it resolves and validates the
// run before reviewserver.Run ever acquires the lock or binds a port;
// everything from lock acquisition onward is reviewserver's job.
func NewReviewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "review <run-id>",
		Short: "Open the ticket-graph review checkpoint for a run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runID := args[0]

			repoRoot, err := findRepoRoot()
			if err != nil {
				return fmt.Errorf("review: %w", err)
			}

			state, err := loadExistingRunState(repoRoot, runID)
			if err != nil {
				return fmt.Errorf("review: %w", err)
			}
			if state.ReviewState == ledger.ReviewStateApproved {
				return fmt.Errorf("review: run %q is already approved, nothing to review", runID)
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return reviewserver.Run(ctx, reviewserver.Options{
				RepoRoot: repoRoot,
				RunID:    runID,
				Stdout:   cmd.OutOrStdout(),
			})
		},
	}
}

// reviewStatusResult is review-status's JSON output. review_state is never
// empty for a run with an existing ledger — ParseRunState defaults an
// absent key to ledger.ReviewStatePending — so printing it as-is already
// reports "approved" explicitly, with no special-case branch needed.
type reviewStatusResult struct {
	ReviewState     string `json:"review_state"`
	ReviewNotes     string `json:"review_notes"`
	ReviewUpdatedAt string `json:"review_updated_at"`
}

// printJSON writes v as indented JSON, HTML-escaping disabled — a small
// unexported copy of internal/cli/loop's own printJSON (jsonio.go), kept
// local rather than exported cross-package since that helper isn't part of
// this ticket's scope to change.
func printJSON(cmd *cobra.Command, v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("marshal JSON output: %w", err)
	}
	fmt.Fprint(cmd.OutOrStdout(), buf.String())
	return nil
}

// findRepoRoot locates the git repository root containing the current
// working directory by walking up from cwd, checking each candidate for a
// ".git" entry (file or dir, so a worktree's own root counts too), and
// checking the filesystem root last — mirroring
// internal/cli/tk/ticketsdir.go's locateTicketsDir shape. Every command
// under internal/cli/loop instead takes a <repo-root> argument explicitly;
// review-status (and, once U8 lands, `goalship review`) has none, so it
// must resolve one itself rather than assume os.Getwd() is already it.
func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := cwd; ; {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no git repository found in %s or its parent directories", cwd)
		}
		dir = parent
	}
}

// loadExistingRunState loads runID's ledger from repoRoot, erroring clearly
// when no ledger file exists for it. Unlike ledger.LoadRunState — which
// deliberately auto-creates a fresh zeroed state for callers like `loop
// ledger` that mint a brand-new run on first call — review-status (and,
// once U8 lands, `goalship review`) must tell a genuinely nonexistent run
// apart from a fresh one, which LoadRunState's own return value can't do,
// so the ledger file's existence is checked directly first.
func loadExistingRunState(repoRoot, runID string) (*ledger.RunState, error) {
	path := ledger.ResolveLedgerPath(repoRoot, runID)
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("no run %q found (no ledger at %s)", runID, path)
	}
	return ledger.LoadRunState(repoRoot, runID)
}

// NewReviewStatusCmd reports a run's review_state so the orchestrating
// agent can discover a pending rejection without polling the review server.
func NewReviewStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "review-status <run-id>",
		Short: "Report a run's review_state (pending, rejected, or approved)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runID := args[0]

			repoRoot, err := findRepoRoot()
			if err != nil {
				return fmt.Errorf("review-status: %w", err)
			}

			state, err := loadExistingRunState(repoRoot, runID)
			if err != nil {
				return fmt.Errorf("review-status: %w", err)
			}

			return printJSON(cmd, reviewStatusResult{
				ReviewState:     state.ReviewState,
				ReviewNotes:     state.ReviewNotes,
				ReviewUpdatedAt: state.ReviewUpdatedAt,
			})
		},
	}
}
