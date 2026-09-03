package tk

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

// mtimeScanCap mirrors bash tk's cmd_closed: `ls -t *.md | head -n 100`
// runs BEFORE the status filter, so with more than 100 tickets in the
// directory, a closed ticket outside the 100 most-recently-modified never
// appears regardless of --limit.
const mtimeScanCap = 100

func NewClosedCmd() *cobra.Command {
	var limit int
	var assignee, tag string

	cmd := &cobra.Command{
		Use:   "closed",
		Short: "List recently closed tickets, most recently modified first",
		RunE: func(cmd *cobra.Command, args []string) error {
			ticketsDir, err := resolveTicketsDir(false)
			if err != nil {
				return err
			}
			infos, err := loadTicketInfos(ticketsDir)
			if err != nil {
				return err
			}

			sort.SliceStable(infos, func(i, j int) bool {
				return infos[i].modTime.After(infos[j].modTime)
			})
			if len(infos) > mtimeScanCap {
				infos = infos[:mtimeScanCap]
			}

			w := cmd.OutOrStdout()
			printed := 0
			for _, in := range infos {
				if printed >= limit {
					break
				}
				// "done" is not a valid transition target (validStatuses)
				// but bash tk's own filter accepts it alongside "closed" —
				// a status a hand-edited or migrated ticket can still
				// carry.
				if in.status != "closed" && in.status != "done" {
					continue
				}
				if assignee != "" && in.assignee != assignee {
					continue
				}
				if tag != "" && !hasTag(in.tags, tag) {
					continue
				}
				fmt.Fprintf(w, "%-8s [%s] - %s\n", in.id, in.status, in.title)
				printed++
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum tickets to show")
	cmd.Flags().StringVarP(&assignee, "assignee", "a", "", "Filter by assignee")
	cmd.Flags().StringVarP(&tag, "tag", "T", "", "Filter by tag")

	return cmd
}
