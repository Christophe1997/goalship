package loop

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/gitops"
)

func NewRunBranchCmd() *cobra.Command {
	var runID string

	cmd := &cobra.Command{
		Use:   "run-branch <repo-root> [ticket-id...]",
		Short: "Find an already-claimed shared branch among ticket IDs",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ticketsDir, err := runTicketsDir("run-branch", args[0], runID)
			if err != nil {
				return err
			}
			branch, err := gitops.RunBranch(ticketsDir, args[1:])
			if err != nil {
				return err
			}
			if branch != "" {
				fmt.Fprintln(cmd.OutOrStdout(), branch)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&runID, "run-id", "", runIDFlagUsage)
	return cmd
}
