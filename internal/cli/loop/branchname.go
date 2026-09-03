package loop

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/gitops"
)

func NewBranchNameCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "branch-name <repo-root> <type> <title>",
		Short: "Compute the branch name for a ticket",
		Args:  cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := gitops.BranchName(args[0], args[1], args[2])
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), name)
			return nil
		},
	}
}
