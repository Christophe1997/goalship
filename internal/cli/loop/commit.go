package loop

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewCommitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "commit",
		Short: "Commit staged changes for the current ticket",
		RunE:  clistub.NotImplemented("loop commit"),
	}
}
