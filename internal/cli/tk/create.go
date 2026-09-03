package tk

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/ticket"
)

// gitUserName is a seam over `git config user.name`, matching bash tk's
// own default-assignee source (`git config user.name 2>/dev/null || true`
// — any failure degrades to "", never an error). Reassignable in tests to
// avoid depending on the test machine's git config.
var gitUserName = func() string {
	out, err := exec.Command("git", "config", "user.name").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// resolveAssignee applies bash tk's create-time assignee precedence: the
// -a/--assignee flag when given, else git config user.name.
func resolveAssignee(assigneeFlagChanged bool, assigneeFlagValue string) string {
	if assigneeFlagChanged {
		return assigneeFlagValue
	}
	return gitUserName()
}

type createOptions struct {
	title       string
	description string
	design      string
	acceptance  string
	ticketType  string
	priority    int
	assignee    string
	externalRef string
	parent      string
	tags        string
}

// runCreate is cmd_create's Go core: resolve --parent, generate a new-shape
// ID, and write the frontmatter + body bash tk's own create writes.
func runCreate(ticketsDir string, opts createOptions) (string, error) {
	if err := os.MkdirAll(ticketsDir, 0o755); err != nil {
		return "", fmt.Errorf("tk create: %w", err)
	}

	resolvedParent := ""
	if opts.parent != "" {
		parentPath, err := ticket.Resolve(ticketsDir, opts.parent)
		if err != nil {
			return "", fmt.Errorf("tk create: --parent: %w", err)
		}
		resolvedParent = strings.TrimSuffix(filepath.Base(parentPath), ".md")
	}

	id, err := ticket.GenerateID(ticketsDir)
	if err != nil {
		return "", fmt.Errorf("tk create: %w", err)
	}

	title := opts.title
	if title == "" {
		title = "Untitled"
	}
	ticketType := opts.ticketType
	if ticketType == "" {
		ticketType = "task"
	}

	var extra []ticket.Field
	if opts.assignee != "" {
		extra = append(extra, ticket.Field{Key: "assignee", Value: " " + opts.assignee})
	}
	if opts.externalRef != "" {
		extra = append(extra, ticket.Field{Key: "external-ref", Value: " " + opts.externalRef})
	}
	if resolvedParent != "" {
		extra = append(extra, ticket.Field{Key: "parent", Value: " " + resolvedParent})
	}
	if opts.tags != "" {
		extra = append(extra, ticket.Field{Key: "tags", Value: " [" + strings.ReplaceAll(opts.tags, ",", ", ") + "]"})
	}

	t := &ticket.Ticket{
		ID:       id,
		Status:   "open",
		Created:  isoNow(),
		Type:     ticketType,
		Priority: opts.priority,
		Extra:    extra,
		Body:     buildCreateBody(title, opts.description, opts.design, opts.acceptance),
	}

	if err := t.Save(filepath.Join(ticketsDir, id+".md")); err != nil {
		return "", fmt.Errorf("tk create: %w", err)
	}
	return id, nil
}

// buildCreateBody reproduces cmd_create's body layout exactly: title, then
// description/design/acceptance sections only when non-empty, each
// followed by a blank line.
func buildCreateBody(title, description, design, acceptance string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", title)
	if description != "" {
		fmt.Fprintf(&b, "%s\n\n", description)
	}
	if design != "" {
		fmt.Fprintf(&b, "## Design\n\n%s\n\n", design)
	}
	if acceptance != "" {
		fmt.Fprintf(&b, "## Acceptance Criteria\n\n%s\n\n", acceptance)
	}
	return b.String()
}

func NewCreateCmd() *cobra.Command {
	var opts createOptions

	cmd := &cobra.Command{
		Use:   "create [title]",
		Short: "Create a new ticket, printing its ID",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.title = args[0]
			}
			opts.assignee = resolveAssignee(cmd.Flags().Changed("assignee"), opts.assignee)

			ticketsDir, err := findOrInitTicketsDir()
			if err != nil {
				return err
			}
			id, err := runCreate(ticketsDir, opts)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), id)
			return nil
		},
	}

	cmd.Flags().StringVarP(&opts.description, "description", "d", "", "Description text")
	cmd.Flags().StringVar(&opts.design, "design", "", "Design notes")
	cmd.Flags().StringVar(&opts.acceptance, "acceptance", "", "Acceptance criteria")
	cmd.Flags().StringVarP(&opts.ticketType, "type", "t", "task", "Type (bug|feature|task|epic|chore)")
	cmd.Flags().IntVarP(&opts.priority, "priority", "p", 2, "Priority 0-4, 0=highest")
	cmd.Flags().StringVarP(&opts.assignee, "assignee", "a", "", "Assignee")
	cmd.Flags().StringVar(&opts.externalRef, "external-ref", "", "External reference (e.g. gh-123, JIRA-456)")
	cmd.Flags().StringVar(&opts.parent, "parent", "", "Parent ticket ID")
	cmd.Flags().StringVar(&opts.tags, "tags", "", "Comma-separated tags (e.g. ui,backend,urgent)")

	return cmd
}
