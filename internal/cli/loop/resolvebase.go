package loop

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/gitops"
)

func NewResolveBaseCmd() *cobra.Command {
	var runID string

	cmd := &cobra.Command{
		Use:   "resolve-base <repo-root> <ticket-id> <trunk-branch> [host-tool]",
		Short: "Resolve the base ref a ticket's branch should build on",
		Args:  cobra.RangeArgs(3, 4),
		RunE: func(cmd *cobra.Command, args []string) error {
			var hostTool string
			if len(args) > 3 {
				hostTool = args[3]
			}
			ticketsDir, err := runTicketsDir("resolve-base", args[0], runID)
			if err != nil {
				return err
			}
			base, err := gitops.ResolveBase(args[0], ticketsDir, args[1], args[2], hostTool)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), base)
			return nil
		},
	}

	cmd.Flags().StringVar(&runID, "run-id", "", runIDFlagUsage)
	return cmd
}
