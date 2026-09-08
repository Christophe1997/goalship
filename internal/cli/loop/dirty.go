package loop

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/ledger"
)

// ticketsDirName mirrors run_state.py's TICKETS_DIR_NAME and
// internal/gitops/reset.go's own ticketsDirName const — tk's state
// directory, excluded from the dirty-tree check for the same reason as
// ledger.LedgerDirName: `tk start`/`tk add-note` mutate it as a routine
// side effect of running this very loop, unrelated to a ticket's
// implementation diff.
const ticketsDirName = ".tickets"

// ignoredDirtyDirNames mirrors preflight.py's _IGNORED_DIRTY_DIR_NAMES.
var ignoredDirtyDirNames = []string{ledger.LedgerDirName, ticketsDirName}

// dirtyPaths mirrors preflight.py's dirty_paths: repo-relative paths git
// considers dirty, excluding the ledger dir and tk's own state dir
// (defense-in-depth: writing the ledger, or tk mutating its own files,
// must never trip this check).
func dirtyPaths(repoRoot string) ([]string, error) {
	out, ok := gitOutput(repoRoot, "status", "--short", "--untracked-files=all")
	if !ok {
		return nil, fmt.Errorf("loop dirty: git status failed in %s", repoRoot)
	}

	paths := []string{}
	for _, line := range strings.Split(out, "\n") {
		if line == "" || len(line) < 3 {
			continue
		}
		relpath := strings.TrimSpace(line[3:])
		ignored := false
		for _, name := range ignoredDirtyDirNames {
			if relpath == name || strings.HasPrefix(relpath, name+"/") {
				ignored = true
				break
			}
		}
		if !ignored {
			paths = append(paths, relpath)
		}
	}
	return paths, nil
}

func NewDirtyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dirty <repo-root>",
		Short: "Report whether the working tree has unexpected dirty paths",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := dirtyPaths(args[0])
			if err != nil {
				return err
			}
			return printJSON(cmd, paths)
		},
	}
}
