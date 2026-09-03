package tk

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// realTkBinary is the real bash tk 0.3.2 this package ports, used as a
// live fixture per the ticket's acceptance criterion 5: the same
// operation sequence against bash tk and goalship tk must produce
// identical resulting files.
const realTkBinary = "/opt/homebrew/Cellar/ticket/0.3.2/bin/tk"

// TestParity_SameSequenceProducesIdenticalFiles runs an identical
// create/start/add-note/close/reopen/add-note sequence against the real
// bash tk binary and against goalship's own Cobra commands (in-process),
// then diffs the resulting ticket files.
//
// Both repos use the same directory basename ("goalship-parity") because
// bash tk's generate_id() derives its id prefix from `basename "$(pwd)"`
// while ticket.GenerateID derives it from the ticketsDir's parent
// directory name — matching basenames is what keeps the prefixes (and so
// everything but the random suffix) identical.
//
// status/add-note mutate an existing file differently under the hood
// (bash: an in-place `sed` substitution on one line; Go: full parse via
// ticket.Load then re-render via Ticket.Bytes()) — those only produce
// byte-identical output when the starting file's frontmatter is already
// canonical, which is true for anything bash tk's own `create` writes.
// Seeding this sequence with a tk-created file (rather than a hand-written
// fixture) is what keeps that divergence out of scope for this test.
func TestParity_SameSequenceProducesIdenticalFiles(t *testing.T) {
	if _, err := os.Stat(realTkBinary); err != nil {
		t.Skipf("real tk binary not found at %s: %v", realTkBinary, err)
	}

	base := t.TempDir()
	bashRepo := filepath.Join(base, "bash", "goalship-parity")
	goRepo := filepath.Join(base, "go", "goalship-parity")
	if err := os.MkdirAll(bashRepo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(goRepo, 0o755); err != nil {
		t.Fatal(err)
	}
	goTicketsDir := filepath.Join(goRepo, ".tickets")

	// bash tk locates .tickets by walking up from cmd.Dir; it must NOT see
	// TICKETS_DIR (set below, for the in-process Go calls) or it would
	// write into goTicketsDir instead of bashRepo/.tickets.
	bashEnv := make([]string, 0, len(os.Environ()))
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "TICKETS_DIR=") {
			bashEnv = append(bashEnv, kv)
		}
	}

	runBash := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(realTkBinary, args...)
		cmd.Dir = bashRepo
		cmd.Env = bashEnv
		var out, stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("bash tk %v: %v\nstderr: %s", args, err, stderr.String())
		}
		return out.String()
	}

	t.Setenv("TICKETS_DIR", goTicketsDir)

	// -a is passed explicitly on both sides so neither run depends on the
	// test machine's `git config user.name` (bash tk's own default source
	// for assignee, which a stubbed gitUserName only overrides on the Go
	// side).
	createArgs := []string{"Parity ticket", "-d", "a description",
		"--design", "a design", "--acceptance", "acceptance text",
		"-t", "bug", "-p", "1", "-a", "parity-tester", "--tags", "ui,backend"}

	bashCreateOut := runBash(append([]string{"create"}, createArgs...)...)
	bashID := trimTrailingNewline(bashCreateOut)

	goCreateCmd := NewCreateCmd()
	var goCreateOut bytes.Buffer
	goCreateCmd.SetOut(&goCreateOut)
	goCreateCmd.SetArgs(createArgs)
	if err := goCreateCmd.Execute(); err != nil {
		t.Fatalf("goalship create: %v", err)
	}
	goID := trimTrailingNewline(goCreateOut.String())

	// start -> add-note -> close -> reopen -> add-note
	runBash("start", bashID)
	execTk(t, NewStartCmd(), []string{goID})

	runBash("add-note", bashID, "claim", "note")
	execTk(t, NewAddNoteCmd(), []string{goID, "claim", "note"})

	runBash("close", bashID)
	execTk(t, NewCloseCmd(), []string{goID})

	runBash("reopen", bashID)
	execTk(t, NewReopenCmd(), []string{goID})

	runBash("add-note", bashID, "branch: chore/x\npr: https://example.com/1\nsha: deadbeef")
	execTk(t, NewAddNoteCmd(), []string{goID, "branch: chore/x\npr: https://example.com/1\nsha: deadbeef"})

	bashFile, err := os.ReadFile(filepath.Join(bashRepo, ".tickets", bashID+".md"))
	if err != nil {
		t.Fatalf("read bash result: %v", err)
	}
	goFile, err := os.ReadFile(filepath.Join(goTicketsDir, goID+".md"))
	if err != nil {
		t.Fatalf("read go result: %v", err)
	}

	normBash := normalizeParityOutput(string(bashFile), bashID)
	normGo := normalizeParityOutput(string(goFile), goID)
	if normBash != normGo {
		t.Errorf("parity mismatch after normalizing id/created:\n--- bash ---\n%s\n--- go ---\n%s", normBash, normGo)
	}
}

// execTk runs a freshly constructed Cobra command with args, failing the
// test on error. Generic over exactly the two methods this test needs so
// each call site can pass e.g. NewStartCmd() inline.
func execTk(t *testing.T, cmd interface {
	SetArgs([]string)
	Execute() error
}, args []string) {
	t.Helper()
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("goalship tk: %v", err)
	}
}

var parityCreatedLineRe = regexp.MustCompile(`created: \S+`)

// normalizeParityOutput replaces the two fields expected to differ
// between independently generated tickets — the id (embedded in the
// id: line and, incidentally, nowhere else in a ticket with no deps/
// links) and the created timestamp — with fixed placeholders.
func normalizeParityOutput(content, id string) string {
	s := content
	s = regexp.MustCompile(regexp.QuoteMeta(id)).ReplaceAllString(s, "TICKET_ID")
	s = parityCreatedLineRe.ReplaceAllString(s, "created: TIMESTAMP")
	return s
}

func trimTrailingNewline(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
