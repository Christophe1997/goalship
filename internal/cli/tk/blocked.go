package tk

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewBlockedCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "blocked",
		Short: "List tickets blocked on dependencies",
		RunE:  clistub.NotImplemented("tk blocked"),
	}
}
