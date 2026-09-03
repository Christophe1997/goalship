package tk

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <ticket-id>",
		Short: "Show or set a ticket's status",
		RunE:  clistub.NotImplemented("tk status"),
	}
}
