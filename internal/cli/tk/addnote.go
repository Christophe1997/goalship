package tk

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewAddNoteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add-note <ticket-id> <text>",
		Short: "Append a timestamped note to a ticket",
		RunE:  clistub.NotImplemented("tk add-note"),
	}
}
