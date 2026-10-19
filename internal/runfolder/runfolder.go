// Package runfolder decides which tickets folder a run's commands use: a
// TICKETS_DIR override, the folder its ledger records, the shared tickets
// area for legacy runs, or (for tk only) a ledger-less run folder.
package runfolder

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Christophe1997/goalship/internal/ledger"
	"github.com/Christophe1997/goalship/internal/ticket"
)

// Resolve returns the tickets folder runID's commands read and write in
// repoRoot. allowLedgerless accepts a run folder holding run.json but no
// ledger, as in a fresh clone where the git-excluded ledger is absent.
// An unknown run is refused rather than minted, so a typo never creates a
// ledger.
func Resolve(repoRoot, runID string, allowLedgerless bool) (string, error) {
	if v := os.Getenv(ticket.TicketsDirEnv); v != "" {
		return v, nil
	}
	area := ticket.ResolveTicketsDir(repoRoot)
	runDir, err := ticket.RunDir(area, runID)
	if err != nil {
		return "", fmt.Errorf("runfolder: %w", err)
	}

	ledgerPath := ledger.ResolveLedgerPath(repoRoot, runID)
	_, err = os.Stat(ledgerPath)
	switch {
	case err == nil:
		state, err := ledger.LoadRunState(repoRoot, runID)
		if err != nil {
			return "", fmt.Errorf("runfolder: %w", err)
		}
		if state.TicketsDir == nil {
			return area, nil
		}
		return filepath.Join(repoRoot, filepath.FromSlash(*state.TicketsDir)), nil
	case !errors.Is(err, os.ErrNotExist):
		return "", fmt.Errorf("runfolder: %w", err)
	}

	if allowLedgerless {
		if _, err := os.Stat(filepath.Join(runDir, ticket.RunSummaryFile)); err == nil {
			return runDir, nil
		}
	}
	return "", fmt.Errorf("runfolder: unknown run %q (no ledger at %s)", runID, ledgerPath)
}

// All returns the tickets area followed by every run folder inside it.
func All(repoRoot string) ([]string, error) {
	area := ticket.ResolveTicketsDir(repoRoot)
	runDirs, err := ticket.ListRunDirs(area)
	if err != nil {
		return nil, fmt.Errorf("runfolder: %w", err)
	}
	return append([]string{area}, runDirs...), nil
}
