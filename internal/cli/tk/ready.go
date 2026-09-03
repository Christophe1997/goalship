package tk

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewReadyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ready",
		Short: "List tickets with no unmet dependencies",
		RunE:  clistub.NotImplemented("tk ready"),
	}
}
