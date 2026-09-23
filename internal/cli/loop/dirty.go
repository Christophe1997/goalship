package loop

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/ledger"
	"github.com/Christophe1997/goalship/internal/ticket"
)

// ignoredDirtyDirNames mirrors preflight.py's _IGNORED_DIRTY_DIR_NAMES:
// repo-relative dirs excluded from the dirty-tree check (defense-in-depth:
// writing the ledger, or tk mutating its own files — including a
// TICKETS_DIR override resolving inside repoRoot — must never trip this
// check, since both are routine side effects of running this very loop,
// unrelated to a ticket's implementation diff).
func ignoredDirtyDirNames(repoRoot string) []string {
	names := []string{ledger.LedgerDirName}
	if relTicketsDir, ok := ticket.RelativeTicketsDir(repoRoot); ok {
		names = append(names, relTicketsDir)
	}
	return names
}

// dirtyPaths mirrors preflight.py's dirty_paths: repo-relative paths git
// considers dirty, excluding the ledger dir and tk's own state dir.
func dirtyPaths(repoRoot string) ([]string, error) {
	out, ok := gitOutput(repoRoot, "status", "--short", "--untracked-files=all")
	if !ok {
		return nil, fmt.Errorf("loop dirty: git status failed in %s", repoRoot)
	}

	ignoredNames := ignoredDirtyDirNames(repoRoot)
	paths := []string{}
	for _, line := range strings.Split(out, "\n") {
		if line == "" || len(line) < 3 {
			continue
		}
		relpath := strings.TrimSpace(line[3:])
		ignored := false
		for _, name := range ignoredNames {
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
