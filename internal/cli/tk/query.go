package tk

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/ticket"
)

func NewQueryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "query [jq-filter]",
		Short: "Output tickets as JSON, optionally filtered",
		RunE: func(cmd *cobra.Command, args []string) error {
			ticketsDir, err := findTicketsDir()
			if err != nil {
				return err
			}
			var filter string
			if len(args) > 0 {
				filter = args[0]
			}

			lines, err := ticket.Query(ticketsDir, filter)
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			for _, line := range lines {
				w.Write(line)
				fmt.Fprintln(w)
			}
			return nil
		},
	}
}
