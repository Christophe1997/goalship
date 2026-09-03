package tk

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func NewLsCmd() *cobra.Command {
	var status, assignee, tag string

	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List tickets",
		RunE: func(cmd *cobra.Command, args []string) error {
			ticketsDir, err := findTicketsDir()
			if err != nil {
				return err
			}
			infos, err := loadTicketInfos(ticketsDir, cmd.ErrOrStderr())
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()
			for _, in := range infos {
				if status != "" && in.status != status {
					continue
				}
				if assignee != "" && in.assignee != assignee {
					continue
				}
				if tag != "" && !hasTag(in.tags, tag) {
					continue
				}
				depsSuffix := ""
				if len(in.deps) > 0 {
					depsSuffix = " <- [" + strings.Join(in.deps, ", ") + "]"
				}
				fmt.Fprintf(w, "%-8s [%s] - %s%s\n", in.id, in.status, in.title, depsSuffix)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Filter by status")
	cmd.Flags().StringVarP(&assignee, "assignee", "a", "", "Filter by assignee")
	cmd.Flags().StringVarP(&tag, "tag", "T", "", "Filter by tag")

	return cmd
}
