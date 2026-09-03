package tk

import (
	"github.com/spf13/cobra"
)

func NewReopenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reopen <ticket-id>",
		Short: "Reopen a closed ticket (status -> open)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ticketsDir, err := findTicketsDir()
			if err != nil {
				return err
			}
			return reportStatusChange(cmd, ticketsDir, args[0], "open")
		},
	}
}
