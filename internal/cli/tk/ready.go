package tk

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewReadyCmd() *cobra.Command {
	var assignee, tag string

	cmd := &cobra.Command{
		Use:   "ready",
		Short: "List open/in-progress tickets whose dependencies are all closed",
		RunE: func(cmd *cobra.Command, args []string) error {
			ticketsDir, err := findTicketsDir()
			if err != nil {
				return err
			}
			infos, err := loadTicketInfos(ticketsDir, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			statusByID := make(map[string]string, len(infos))
			for _, in := range infos {
				statusByID[in.id] = in.status
			}

			var ready []ticketInfo
			for _, in := range infos {
				if in.status != "open" && in.status != "in_progress" {
					continue
				}
				if assignee != "" && in.assignee != assignee {
					continue
				}
				if tag != "" && !hasTag(in.tags, tag) {
					continue
				}

				allClosed := true
				for _, dep := range in.deps {
					// A dangling dep id is not a key in statusByID, so
					// the lookup below returns "" — the zero value —
					// which is never "closed". This mirrors bash tk's
					// awk: an unset associative-array element also reads
					// back as "". Result: a ticket depending on a
					// nonexistent id can never be ready, silently
					// (ticket goa-krro's acceptance criterion 4) — not
					// fixed here by design, out of this ticket's scope.
					if statusByID[dep] != "closed" {
						allClosed = false
						break
					}
				}
				if allClosed {
					ready = append(ready, in)
				}
			}
			byPriorityThenID(ready)

			w := cmd.OutOrStdout()
			for _, in := range ready {
				fmt.Fprintf(w, "%-8s [P%d][%s] - %s\n", in.id, in.priority, in.status, in.title)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&assignee, "assignee", "a", "", "Filter by assignee")
	cmd.Flags().StringVarP(&tag, "tag", "T", "", "Filter by tag")

	return cmd
}
