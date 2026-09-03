package tk

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/ticket"
)

const beadsIssuesPath = ".beads/issues.jsonl"

func NewMigrateBeadsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate-beads",
		Short: "Import tickets from .beads/issues.jsonl",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrateBeads(cmd)
		},
	}
}

func runMigrateBeads(cmd *cobra.Command) error {
	data, err := os.ReadFile(beadsIssuesPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("Error: %s not found", beadsIssuesPath)
		}
		return err
	}

	// migrate-beads is a WRITE_COMMANDS command: it may initialize a
	// fresh .tickets/ in the current directory if none is found.
	ticketsDir, err := findOrInitTicketsDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(ticketsDir, 0o755); err != nil {
		return err
	}

	w := cmd.OutOrStdout()
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var issue map[string]any
		if err := json.Unmarshal([]byte(line), &issue); err != nil {
			return fmt.Errorf("ticket: migrate-beads: parse %s: %w", beadsIssuesPath, err)
		}

		id, content, err := ticket.RenderBeadsIssue(issue)
		if err != nil {
			return err
		}
		t, err := ticket.Parse([]byte(content))
		if err != nil {
			return fmt.Errorf("ticket: migrate-beads: rendered ticket %q: %w", id, err)
		}
		if err := t.Save(filepath.Join(ticketsDir, id+".md")); err != nil {
			return err
		}

		fmt.Fprintf(w, "Migrated: %s\n", id)
		count++
	}
	fmt.Fprintf(w, "Migrated %d tickets from beads\n", count)
	return nil
}
