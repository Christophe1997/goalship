package tk

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <ticket-id>",
		Short: "Show a ticket's full content",
		RunE:  clistub.NotImplemented("tk show"),
	}
}
