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
	runDir, err := RunDir(area, runID)
	if err != nil {
		return "", err
	}

	ledgerPath := ledger.ResolveLedgerPath(repoRoot, runID)
	_, err = os.Stat(ledgerPath)
	switch {
	case err == nil:
		state, err := ledger.LoadRunState(repoRoot, runID)
		if err != nil {
			return "", fmt.Errorf("runfolder: %w", err)
		}
		dir, ok, err := RecordedDir(repoRoot, state)
		if err != nil {
			return "", err
		}
		if !ok {
			return area, nil
		}
		return dir, nil
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

// RunDir returns runID's folder inside areaDir. The folder name is
// sanitized exactly as the run's ledger file name is, and a name that
// would escape or alias the area itself is refused.
func RunDir(areaDir, runID string) (string, error) {
	safe := ledger.SafeRunID(runID)
	if safe == "" || safe == "." || safe == ".." {
		return "", fmt.Errorf("runfolder: invalid run id %q", runID)
	}
	return filepath.Join(areaDir, safe), nil
}

// RecordedDir returns the run folder state's ledger records, or ok false for
// a legacy run that records none. tk and loop write wherever a ledger
// points, and a ledger can be hand-edited, so a folder outside the default
// tickets area (outside the repo, or where commit and reset would stage or
// clean it) is refused.
func RecordedDir(repoRoot string, state *ledger.RunState) (dir string, ok bool, err error) {
	if state.TicketsDir == nil {
		return "", false, nil
	}
	area := ticket.DefaultTicketsDir(repoRoot)
	dir = filepath.Join(repoRoot, filepath.FromSlash(*state.TicketsDir))
	if rel, err := filepath.Rel(area, dir); err != nil || !filepath.IsLocal(rel) {
		return "", false, fmt.Errorf("runfolder: run %q records tickets_dir %q outside %s",
			state.RunID, *state.TicketsDir, area)
	}
	return dir, true, nil
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
