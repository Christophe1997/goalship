package tk

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/ticket"
)

func newDepCycleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cycle",
		Short: "Find dependency cycles across all open tickets",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDepCycle(cmd)
		},
	}
}

func runDepCycle(cmd *cobra.Command) error {
	ticketsDir, err := findTicketsDir()
	if err != nil {
		return err
	}
	tickets, err := ticket.LoadAwkTickets(ticketsDir)
	if err != nil {
		return err
	}

	// cmd_dep_cycle's own store() only keeps non-closed tickets: a chain
	// through a closed ticket is a dead end for cycle detection.
	g := &depGraph{statuses: map[string]string{}, titles: map[string]string{}, deps: map[string][]string{}}
	for _, t := range tickets {
		if t.ID == "" || t.Status == "closed" {
			continue
		}
		g.statuses[t.ID] = t.Status
		g.titles[t.ID] = t.Title
		g.deps[t.ID] = t.Deps
	}

	printCycles(cmd.OutOrStdout(), g, findCycles(g))
	return nil
}

// cycle is one detected dependency cycle: header is the raw DFS-returned
// chain (e.g. "b -> c -> a -> b" — whichever node the DFS happened to
// start from, not necessarily the lexicographically smallest); Members
// is the normalized rotation (starting at the smallest ID) cmd_dep_cycle
// prints one line per, used here for both the printed member list and
// cross-cycle dedup.
type cycle struct {
	header  string
	members []string
}

// findCycles runs cmd_dep_cycle's own DFS cycle detection: white/gray/
// black node coloring, one DFS per still-white top-level node (sorted by
// ID for deterministic output — bash's own `for (id in statuses)` order
// is unspecified awk hash iteration, so cross-run cycle order/rotation
// isn't a bash-parity target, only cycle count and membership are), the
// first cycle found from each start normalized and deduped.
//
// A node that closes a cycle is left gray forever (bash's dfs() returns
// before ever marking it black on that path) — reachable-but-unvisited
// nodes on a separate branch of the same start are still exhaustively
// explored by the recursion below them; this only means a node already
// marked gray by an earlier top-level DFS is never re-driven as its own
// fresh start, exactly matching bash's `if (state[id] == 0)` guard.
func findCycles(g *depGraph) []cycle {
	ids := make([]string, 0, len(g.statuses))
	for id := range g.statuses {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	state := map[string]int{} // 0 white (default), 1 gray, 2 black
	var dfs func(node string, path []string) string
	dfs = func(node string, path []string) string {
		if _, ok := g.statuses[node]; !ok {
			return ""
		}
		switch state[node] {
		case 2:
			return ""
		case 1:
			c := node
			for i := len(path) - 1; i >= 0; i-- {
				c = path[i] + " -> " + c
				if path[i] == node {
					break
				}
			}
			return c
		}
		state[node] = 1
		newPath := append(append([]string{}, path...), node)
		for _, child := range g.deps[node] {
			if child == "" {
				continue
			}
			if result := dfs(child, newPath); result != "" {
				return result
			}
		}
		state[node] = 2
		return ""
	}

	var cycles []cycle
	seen := map[string]bool{}
	for _, id := range ids {
		if state[id] != 0 {
			continue
		}
		result := dfs(id, nil)
		if result == "" {
			continue
		}
		parts := strings.Split(result, " -> ")
		members := parts[:len(parts)-1] // parts[len-1] duplicates parts[0], the closing edge

		minIdx := 0
		for i := 1; i < len(members); i++ {
			if members[i] < members[minIdx] {
				minIdx = i
			}
		}
		norm := make([]string, len(members))
		for i := range members {
			norm[i] = members[(minIdx+i)%len(members)]
		}

		key := strings.Join(norm, ",")
		if seen[key] {
			continue
		}
		seen[key] = true
		cycles = append(cycles, cycle{header: result, members: norm})
	}
	return cycles
}

func printCycles(w io.Writer, g *depGraph, cycles []cycle) {
	if len(cycles) == 0 {
		fmt.Fprintln(w, "No dependency cycles found")
		return
	}
	for i, c := range cycles {
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "Cycle %d: %s\n", i+1, c.header)
		for _, id := range c.members {
			fmt.Fprintf(w, "  %-8s [%s] %s\n", id, g.statuses[id], g.titles[id])
		}
	}
}
