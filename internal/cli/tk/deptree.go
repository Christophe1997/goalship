package tk

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func newDepTreeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tree <ticket-id>",
		Short: "Show a ticket's dependency tree",
		RunE:  clistub.NotImplemented("tk dep tree"),
	}
}
