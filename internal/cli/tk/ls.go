package tk

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/ticket"
)

type lsFilter struct {
	status, assignee, tag string
}

func (f lsFilter) matches(in ticketInfo) bool {
	return (f.status == "" || in.status == f.status) &&
		(f.assignee == "" || in.assignee == f.assignee) &&
		(f.tag == "" || hasTag(in.tags, f.tag))
}

func NewLsCmd() *cobra.Command {
	var filter lsFilter
	var all bool

	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List tickets",
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				return runLsAll(cmd, filter)
			}
			ticketsDir, err := findSelectedTicketsDir(cmd)
			if err != nil {
				return err
			}
			return printLsFolder(cmd, ticketsDir, filter)
		},
	}

	cmd.Flags().StringVar(&filter.status, "status", "", "Filter by status")
	cmd.Flags().StringVarP(&filter.assignee, "assignee", "a", "", "Filter by assignee")
	cmd.Flags().StringVarP(&filter.tag, "tag", "T", "", "Filter by tag")
	cmd.Flags().BoolVar(&all, "all", false, "List the loose tickets, then every run's tickets under a heading per run")

	return withRunIDFlag(cmd)
}

// runLsAll prints the loose tickets, then each run folder under an
// "== <run-id>: <goal>" heading, so every run shows even when empty.
func runLsAll(cmd *cobra.Command, filter lsFilter) error {
	area, err := findTicketsDir()
	if err != nil {
		return err
	}
	runDirs, err := ticket.ListRunDirs(area)
	if err != nil {
		return err
	}
	if err := printLsFolder(cmd, area, filter); err != nil {
		return err
	}
	w := cmd.OutOrStdout()
	for _, dir := range runDirs {
		runID, goal := runLabel(dir)
		if goal == "" {
			fmt.Fprintf(w, "== %s\n", runID)
		} else {
			fmt.Fprintf(w, "== %s: %s\n", runID, goal)
		}
		if err := printLsFolder(cmd, dir, filter); err != nil {
			return err
		}
	}
	return nil
}

func printLsFolder(cmd *cobra.Command, ticketsDir string, filter lsFilter) error {
	infos, err := loadTicketInfos(ticketsDir, cmd.ErrOrStderr())
	if err != nil {
		return err
	}
	w := cmd.OutOrStdout()
	for _, in := range infos {
		if filter.matches(in) {
			writeLsLine(w, in)
		}
	}
	return nil
}

func writeLsLine(w io.Writer, in ticketInfo) {
	depsSuffix := ""
	if len(in.deps) > 0 {
		depsSuffix = " <- [" + strings.Join(in.deps, ", ") + "]"
	}
	fmt.Fprintf(w, "%-8s [%s] - %s%s\n", in.id, in.status, in.title, depsSuffix)
}
