package tk

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/ticket"
)

func newDepTreeCmd() *cobra.Command {
	var full bool
	cmd := &cobra.Command{
		Use:   "tree [--full] <id>",
		Short: "Show a ticket's dependency tree",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("Usage: ticket dep tree [--full] <id>")
			}
			return runDepTree(cmd, args[0], full)
		},
	}
	cmd.Flags().BoolVar(&full, "full", false, "Disable dedup: print every occurrence of a ticket, once per path")
	return cmd
}

func runDepTree(cmd *cobra.Command, rootPattern string, full bool) error {
	ticketsDir, err := findTicketsDir()
	if err != nil {
		return err
	}
	tickets, err := ticket.LoadAwkTickets(ticketsDir)
	if err != nil {
		return err
	}

	g := newDepGraph(tickets)
	root, err := g.resolveByPattern(rootPattern)
	if err != nil {
		return err
	}

	maxDepth := g.maxDepths(root)
	subtreeDepth := g.subtreeDepths(root, maxDepth)
	g.printTree(cmd.OutOrStdout(), root, maxDepth, subtreeDepth, full)
	return nil
}

// depGraph is the in-memory dependency graph cmd_dep_tree/cmd_dep_cycle
// walk directly — no external dependency, per this ticket's own
// description.
type depGraph struct {
	statuses map[string]string
	titles   map[string]string
	deps     map[string][]string
}

func newDepGraph(tickets []ticket.AwkTicket) *depGraph {
	g := &depGraph{
		statuses: map[string]string{},
		titles:   map[string]string{},
		deps:     map[string][]string{},
	}
	for _, t := range tickets {
		if t.ID == "" {
			continue
		}
		g.statuses[t.ID] = t.Status
		g.titles[t.ID] = t.Title
		g.deps[t.ID] = t.Deps
	}
	return g
}

// resolveByPattern is cmd_dep_tree's own root-resolution algorithm — a
// raw substring match against every known ID, with no "exact filename
// match first" shortcut (unlike ticket.Resolve/ticket_path). Message
// text (no quotes around the pattern) matches cmd_dep_tree's awk output
// exactly, distinct from ticket_path()'s quoted messages.
func (g *depGraph) resolveByPattern(pattern string) (string, error) {
	var found string
	for id := range g.statuses {
		if strings.Contains(id, pattern) {
			if found != "" {
				return "", fmt.Errorf("Error: ambiguous ID %s", pattern)
			}
			found = id
		}
	}
	if found == "" {
		return "", fmt.Errorf("Error: ticket %s not found", pattern)
	}
	return found, nil
}

// maxDepths returns, for every ID reachable from root, the greatest
// depth (edge count from root) at which it's reached across all
// non-cyclic paths — cmd_dep_tree's first awk pass.
func (g *depGraph) maxDepths(root string) map[string]int {
	type frame struct {
		id    string
		depth int
		path  string // ":"-delimited ancestor chain, e.g. ":root:child:"
	}

	maxDepth := map[string]int{}
	stack := []frame{{id: root, depth: 0, path: ":"}}
	for len(stack) > 0 {
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if _, ok := g.statuses[f.id]; !ok {
			continue
		}
		if strings.Contains(f.path, ":"+f.id+":") {
			continue // f.id is its own ancestor on this path: cycle, stop descending
		}
		if cur, ok := maxDepth[f.id]; !ok || f.depth > cur {
			maxDepth[f.id] = f.depth
		}

		newPath := f.path + f.id + ":"
		for _, child := range g.deps[f.id] {
			if child == "" {
				continue
			}
			stack = append(stack, frame{id: child, depth: f.depth + 1, path: newPath})
		}
	}
	return maxDepth
}

// subtreeDepths returns, for every ID reachable from root, the deepest
// maxDepth value reachable anywhere within that ID's own subtree —
// cmd_dep_tree's second awk pass, a memoized post-order walk. Used only
// to order siblings when printing (deepest subtree first).
func (g *depGraph) subtreeDepths(root string, maxDepth map[string]int) map[string]int {
	subtree := map[string]int{}
	var visit func(id, path string)
	visit = func(id, path string) {
		if _, ok := g.statuses[id]; !ok {
			return
		}
		if strings.Contains(path, ":"+id+":") {
			return
		}
		if _, done := subtree[id]; done {
			return
		}
		newPath := path + id + ":"
		for _, child := range g.deps[id] {
			if child == "" {
				continue
			}
			if _, done := subtree[child]; !done {
				visit(child, newPath)
			}
		}
		max := maxDepth[id]
		for _, child := range g.deps[id] {
			if v, ok := subtree[child]; ok && v > max {
				max = v
			}
		}
		subtree[id] = max
	}
	visit(root, ":")
	return subtree
}

// printTree renders the tree exactly as cmd_dep_tree's third awk pass
// does: an explicit print stack (not plain recursion), because
// non-full-mode dedup ("printed") is genuinely global across sibling
// subtrees and interacts with push-time AND pop-time filtering in a way
// that isn't a clean per-subtree recursion.
func (g *depGraph) printTree(w io.Writer, root string, maxDepth, subtreeDepth map[string]int, full bool) {
	fmt.Fprintf(w, "%s [%s] %s\n", root, g.statuses[root], g.titles[root])
	printed := map[string]bool{root: true}

	type frame struct {
		id, prefix, connector, path string
		depth                       int
	}
	var stack []frame

	pushChildren := func(id string, depth int, prefix, connector, path string) {
		var candidates []string
		for _, child := range g.deps[id] {
			if child == "" {
				continue
			}
			if !full && printed[child] {
				continue
			}
			if _, ok := maxDepth[child]; !ok {
				continue
			}
			if !full && depth+1 != maxDepth[child] {
				continue
			}
			if strings.Contains(path, ":"+child+":") {
				continue
			}
			candidates = append(candidates, child)
		}
		sort.SliceStable(candidates, func(i, j int) bool {
			a, b := candidates[i], candidates[j]
			if subtreeDepth[a] != subtreeDepth[b] {
				return subtreeDepth[a] > subtreeDepth[b]
			}
			return a < b
		})
		n := len(candidates)
		for i := n - 1; i >= 0; i-- {
			conn := "├── "
			if i == n-1 {
				conn = "└── "
			}
			stack = append(stack, frame{id: candidates[i], depth: depth + 1, prefix: prefix, connector: conn, path: path})
		}
	}

	pushChildren(root, 0, "", "", ":"+root+":")

	for len(stack) > 0 {
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if _, ok := g.statuses[f.id]; !ok {
			continue
		}
		if !full && printed[f.id] {
			continue
		}
		if strings.Contains(f.path, ":"+f.id+":") {
			continue
		}
		if !full && f.depth != maxDepth[f.id] {
			continue
		}

		fmt.Fprintf(w, "%s%s%s [%s] %s\n", f.prefix, f.connector, f.id, g.statuses[f.id], g.titles[f.id])
		if !full {
			printed[f.id] = true
		}

		newPrefix := f.prefix + "│   "
		if f.connector == "└── " {
			newPrefix = f.prefix + "    "
		}
		pushChildren(f.id, f.depth, newPrefix, f.connector, f.path+f.id+":")
	}
}
