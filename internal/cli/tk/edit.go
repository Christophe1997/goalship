package tk

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit <ticket-id>",
		Short: "Edit a ticket in $EDITOR",
		RunE:  clistub.NotImplemented("tk edit"),
	}
}
