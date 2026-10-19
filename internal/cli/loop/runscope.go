package loop

import (
	"fmt"

	"github.com/Christophe1997/goalship/internal/runfolder"
)

const runIDFlagUsage = "Run ID whose tickets folder this command uses (required)"

// runTicketsDir resolves the tickets folder a run-scoped command reads and
// writes. An unknown run is refused rather than given a ledger (KTD4).
func runTicketsDir(cmdName, repoRoot, runID string) (string, error) {
	if runID == "" {
		return "", fmt.Errorf("loop %s: --run-id is required", cmdName)
	}
	dir, err := runfolder.Resolve(repoRoot, runID, false)
	if err != nil {
		return "", fmt.Errorf("loop %s: %w", cmdName, err)
	}
	return dir, nil
}
