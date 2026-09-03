package tk

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start <ticket-id>",
		Short: "Mark a ticket as started",
		RunE:  clistub.NotImplemented("tk start"),
	}
}
