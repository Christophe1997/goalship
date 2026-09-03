package tk

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List tickets",
		RunE:    clistub.NotImplemented("tk ls"),
	}
}
