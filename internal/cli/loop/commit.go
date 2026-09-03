package loop

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/gitops"
)

func NewCommitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "commit <repo-root> <message>",
		Short: "Stage everything except .tickets/ and commit, printing the new head SHA",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			sha, err := gitops.CommitAll(args[0], args[1])
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), sha)
			return nil
		},
	}
}
