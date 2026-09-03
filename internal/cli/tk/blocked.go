package tk

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func NewBlockedCmd() *cobra.Command {
	var assignee, tag string

	cmd := &cobra.Command{
		Use:   "blocked",
		Short: "List open/in-progress tickets with at least one unresolved dependency",
		RunE: func(cmd *cobra.Command, args []string) error {
			ticketsDir, err := resolveTicketsDir(false)
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

			type blockedEntry struct {
				info     ticketInfo
				blockers []string
			}
			var blocked []blockedEntry
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
				if len(in.deps) == 0 {
					continue
				}

				var blockers []string
				for _, dep := range in.deps {
					// Same zero-value lookup as ready, but here it makes
					// the ticket blocked (and the dangling id itself
					// appears in the blocker list) rather than excluded —
					// cmd_blocked's awk never checks the dep id exists.
					if statusByID[dep] != "closed" {
						blockers = append(blockers, dep)
					}
				}
				if len(blockers) > 0 {
					blocked = append(blocked, blockedEntry{info: in, blockers: blockers})
				}
			}

			infosOnly := make([]ticketInfo, len(blocked))
			for i, b := range blocked {
				infosOnly[i] = b.info
			}
			byPriorityThenID(infosOnly)
			blockersByID := make(map[string][]string, len(blocked))
			for _, b := range blocked {
				blockersByID[b.info.id] = b.blockers
			}

			w := cmd.OutOrStdout()
			for _, in := range infosOnly {
				fmt.Fprintf(w, "%-8s [P%d][%s] - %s <- [%s]\n",
					in.id, in.priority, in.status, in.title, strings.Join(blockersByID[in.id], ", "))
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&assignee, "assignee", "a", "", "Filter by assignee")
	cmd.Flags().StringVarP(&tag, "tag", "T", "", "Filter by tag")

	return cmd
}
