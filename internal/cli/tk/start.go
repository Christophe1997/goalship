package tk

import (
	"github.com/spf13/cobra"
)

func NewStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start <ticket-id>",
		Short: "Mark a ticket as started (status -> in_progress)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ticketsDir, err := resolveTicketsDir(false)
			if err != nil {
				return err
			}
			return reportStatusChange(cmd, ticketsDir, args[0], "in_progress")
		},
	}
}
