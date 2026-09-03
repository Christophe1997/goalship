package tk

import (
	"github.com/spf13/cobra"
)

func NewCloseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "close <ticket-id>",
		Short: "Close a ticket (status -> closed)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ticketsDir, err := findTicketsDir()
			if err != nil {
				return err
			}
			return reportStatusChange(cmd, ticketsDir, args[0], "closed")
		},
	}
}
