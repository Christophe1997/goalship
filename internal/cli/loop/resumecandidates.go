package loop

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/clistub"
)

func NewResumeCandidatesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resume-candidates",
		Short: "List runs eligible to resume",
		RunE:  clistub.NotImplemented("loop resume-candidates"),
	}
}
