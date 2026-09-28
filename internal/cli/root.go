// Package cli wires goalship's Cobra command tree: three cobra.Group
// sections (tk, loop, review) as laid out in the plan's KTD3.
package cli

import (
	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/cli/loop"
	"github.com/Christophe1997/goalship/internal/cli/tk"
)

const (
	groupTK     = "tk"
	groupLoop   = "loop"
	groupReview = "review"
)

// version is stamped at release time via -ldflags "-X ...cli.version=<tag>";
// it must stay a string var for -X to apply.
var version = "dev"

// NewRootCmd builds the full goalship command tree.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "goalship",
		Short:   "goalship: ticket tracking and execution-loop mechanics in one CLI",
		Version: version,
		// main.go prints the returned error itself; without both Silence
		// flags Cobra would print it a second time.
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddGroup(
		&cobra.Group{ID: groupTK, Title: "Ticket Commands:"},
		&cobra.Group{ID: groupLoop, Title: "Loop Commands:"},
		&cobra.Group{ID: groupReview, Title: "Review Commands:"},
	)

	tkCmd := tk.NewCmd()
	tkCmd.GroupID = groupTK

	loopCmd := loop.NewCmd()
	loopCmd.GroupID = groupLoop

	reviewCmd := NewReviewCmd()
	reviewCmd.GroupID = groupReview

	reviewStatusCmd := NewReviewStatusCmd()
	reviewStatusCmd.GroupID = groupReview

	root.AddCommand(tkCmd, loopCmd, reviewCmd, reviewStatusCmd)

	return root
}

// Execute runs the goalship command tree against os.Args.
func Execute() error {
	return NewRootCmd().Execute()
}
