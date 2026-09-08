package loop

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/gitops"
)

func NewCreatePRCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create-pr <repo-root> <host-tool> <branch> <base> <title> <body>",
		Short: "Open a pull/merge request for branch against base, printing its URL",
		Args:  cobra.ExactArgs(6),
		RunE: func(cmd *cobra.Command, args []string) error {
			url, err := gitops.CreatePullRequest(args[0], args[1], args[2], args[3], args[4], args[5])
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), url)
			return nil
		},
	}
}
