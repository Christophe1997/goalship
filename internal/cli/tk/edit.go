package tk

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/ticket"
)

// runEditor is a seam over launching the real editor process, so tests
// can assert which editor/path would run without actually spawning one.
var runEditor = func(editor, path string) error {
	c := exec.Command(editor, path)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func NewEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit <ticket-id>",
		Short: "Edit a ticket's raw file in $EDITOR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ticketsDir, err := findTicketsDir()
			if err != nil {
				return err
			}
			path, err := ticket.Resolve(ticketsDir, args[0])
			if err != nil {
				return fmt.Errorf("tk edit: %w", err)
			}

			// bash tk's cmd_edit only launches $EDITOR when both stdin and
			// stdout are interactive; otherwise it just prints the path —
			// an agent-driven caller with no tty attached gets the path to
			// act on directly instead of a blocking editor process.
			if !stdinAndStdoutAreTTY() {
				fmt.Fprintf(cmd.OutOrStdout(), "Edit ticket file: %s\n", path)
				return nil
			}

			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}
			return runEditor(editor, path)
		},
	}
}
