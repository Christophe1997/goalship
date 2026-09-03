package tk

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/ticket"
)

// depUsageErr is bash tk's own multi-line cmd_dep usage text (printed to
// stderr, exit 1) for an argc mismatch — Cobra's own auto-usage is
// suppressed by root.go's SilenceUsage, so this is the only usage text a
// caller sees.
var depUsageErr = errors.New("Usage: ticket dep <id> <dependency-id>\n" +
	"       ticket dep tree <id>  - show dependency tree\n" +
	"       ticket dep cycle      - find dependency cycles")

func NewDepCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dep <id> <dependency-id>",
		Short: "Add a dependency (id depends on dep-id)",
		// Cobra's default Args validator special-cases a parentless
		// command with subcommands (legacyArgs) to reject any args not
		// matching a subcommand name — irrelevant once dep is attached
		// under tk, but this keeps `dep <id> <dep-id>` working whether
		// or not it is. runDep does its own arg-count check below.
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return depUsageErr
			}
			return runDep(cmd, args[0], args[1])
		},
	}
	cmd.AddCommand(newDepTreeCmd(), newDepCycleCmd())
	return cmd
}

func runDep(cmd *cobra.Command, rawID, rawDepID string) error {
	ticketsDir, err := findTicketsDir()
	if err != nil {
		return err
	}

	path, err := resolveOrBashErr(ticketsDir, rawID)
	if err != nil {
		return err
	}
	depPath, err := resolveOrBashErr(ticketsDir, rawDepID)
	if err != nil {
		return err
	}
	id, depID := idFromPath(path), idFromPath(depPath)

	t, err := ticket.Load(path)
	if err != nil {
		return err
	}

	if containsID(t.Deps, depID) {
		fmt.Fprintln(cmd.OutOrStdout(), "Dependency already exists")
		return nil
	}

	t.Deps = append(t.Deps, depID)
	if err := t.Save(path); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Added dependency: %s -> %s\n", id, depID)
	return nil
}
