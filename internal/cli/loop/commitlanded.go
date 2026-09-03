package loop

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/gitops"
)

func NewCommitLandedCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "commit-landed <repo-root> <branch> <claim-sha>",
		Short: "Report whether a commit has landed on branch since claim-sha",
		Args:  cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			landed, err := gitops.CommitLanded(args[0], args[1], args[2])
			if err != nil {
				return err
			}
			answer := "no"
			if landed {
				answer = "yes"
			}
			fmt.Fprintln(cmd.OutOrStdout(), answer)
			return nil
		},
	}
}
