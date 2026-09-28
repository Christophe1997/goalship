package gitops

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// captureArgvHostTool is withFakeHostTool plus argv capture: the fake tool
// records its own argv (one per line, via `printf`, so an arg containing
// spaces still round-trips) to a file before running outputScript, letting
// a test assert on the exact argv this package's PR functions invoke.
func captureArgvHostTool(t *testing.T, name, outputScript string) (argvFile string) {
	t.Helper()
	argvFile = filepath.Join(t.TempDir(), "argv.txt")
	script := "printf '%s\\n' \"$@\" > \"" + argvFile + "\"\n" + outputScript
	withFakeHostTool(t, name, script)
	return argvFile
}

// warmFakeHostTool runs name once with a throwaway argument that matches no
// case branch, before a test starts timing a tight hostToolTimeout budget.
// A freshly written executable's very first invocation costs far more than
// its steady-state runtime, which would otherwise dominate a short timeout
// budget's first (state-lookup) call in a two-call test and make it flaky;
// paying that cost here, outside the timed section, keeps the timing
// assertions meaningful.
func warmFakeHostTool(t *testing.T, name string) {
	t.Helper()
	path, err := exec.LookPath(name)
	if err != nil {
		t.Fatalf("warm fake %s: %v", name, err)
	}
	_ = exec.Command(path, "warmup").Run()
}

func readArgv(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read argv file: %v", err)
	}
	trimmedStr := strings.TrimRight(string(data), "\n")
	if trimmedStr == "" {
		return nil
	}
	return strings.Split(trimmedStr, "\n")
}

func TestFindOpenPRForBranch_GH_ArgvAndURL(t *testing.T) {
	argvFile := captureArgvHostTool(t, "gh", `echo '[{"url": "https://github.com/o/r/pull/5"}]'`)

	url := FindOpenPRForBranch(t.TempDir(), "gh", "feat/x")
	if url != "https://github.com/o/r/pull/5" {
		t.Errorf("url = %q, want the PR URL", url)
	}

	want := []string{"pr", "list", "--head", "feat/x", "--state", "open", "--json", "url"}
	if got := readArgv(t, argvFile); !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %v, want %v", got, want)
	}
}

func TestFindOpenPRForBranch_GH_NoOpenPR_ReturnsEmpty(t *testing.T) {
	withFakeHostTool(t, "gh", `echo '[]'`)
	if url := FindOpenPRForBranch(t.TempDir(), "gh", "feat/x"); url != "" {
		t.Errorf("url = %q, want empty", url)
	}
}

func TestFindOpenPRForBranch_GH_NonzeroExit_ReturnsEmpty(t *testing.T) {
	withFakeHostTool(t, "gh", `exit 1`)
	if url := FindOpenPRForBranch(t.TempDir(), "gh", "feat/x"); url != "" {
		t.Errorf("url = %q, want empty", url)
	}
}

func TestFindOpenPRForBranch_Glab_ArgvAndURL(t *testing.T) {
	argvFile := captureArgvHostTool(t, "glab", `echo '[{"web_url": "https://gitlab.com/o/r/-/merge_requests/9"}]'`)

	url := FindOpenPRForBranch(t.TempDir(), "glab", "feat/x")
	if url != "https://gitlab.com/o/r/-/merge_requests/9" {
		t.Errorf("url = %q, want the MR URL", url)
	}

	want := []string{"mr", "list", "--source-branch", "feat/x", "-F", "json"}
	if got := readArgv(t, argvFile); !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %v, want %v", got, want)
	}
}

func TestFindOpenPRForBranch_UnsupportedHostTool_ReturnsEmpty(t *testing.T) {
	if url := FindOpenPRForBranch(t.TempDir(), "hub", "feat/x"); url != "" {
		t.Errorf("url = %q, want empty", url)
	}
}

func TestCreatePullRequest_GH_ArgvAndURL(t *testing.T) {
	argvFile := captureArgvHostTool(t, "gh", `echo "https://github.com/o/r/pull/9"`)

	url, err := CreatePullRequest(t.TempDir(), "gh", "feat/x", "main", "Title", "Body text")
	if err != nil {
		t.Fatalf("CreatePullRequest: %v", err)
	}
	if url != "https://github.com/o/r/pull/9" {
		t.Errorf("url = %q, want the created PR URL", url)
	}

	want := []string{"pr", "create", "--head", "feat/x", "--base", "main", "--title", "Title", "--body", "Body text"}
	if got := readArgv(t, argvFile); !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %v, want %v", got, want)
	}
}

func TestCreatePullRequest_Glab_ArgvAndURL(t *testing.T) {
	argvFile := captureArgvHostTool(t, "glab", `echo "https://gitlab.com/o/r/-/merge_requests/2"`)

	url, err := CreatePullRequest(t.TempDir(), "glab", "feat/x", "main", "Title", "Body text")
	if err != nil {
		t.Fatalf("CreatePullRequest: %v", err)
	}
	if url != "https://gitlab.com/o/r/-/merge_requests/2" {
		t.Errorf("url = %q, want the created MR URL", url)
	}

	want := []string{
		"mr", "create",
		"--source-branch", "feat/x", "--target-branch", "main",
		"--title", "Title", "--description", "Body text", "--yes",
	}
	if got := readArgv(t, argvFile); !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %v, want %v", got, want)
	}
}

func TestCreatePullRequest_PicksLastURLLine(t *testing.T) {
	withFakeHostTool(t, "gh", "echo 'Creating pull request'\necho 'https://github.com/o/r/pull/1'")
	url, err := CreatePullRequest(t.TempDir(), "gh", "feat/x", "main", "T", "B")
	if err != nil {
		t.Fatalf("CreatePullRequest: %v", err)
	}
	if url != "https://github.com/o/r/pull/1" {
		t.Errorf("url = %q, want the last URL line", url)
	}
}

func TestCreatePullRequest_NoURLInOutput_Errors(t *testing.T) {
	withFakeHostTool(t, "gh", `echo 'no url here'`)
	_, err := CreatePullRequest(t.TempDir(), "gh", "feat/x", "main", "T", "B")
	if err == nil || !strings.Contains(err.Error(), "did not print a URL") {
		t.Errorf("err = %v, want an error mentioning a missing URL", err)
	}
}

func TestCreatePullRequest_NonzeroExit_ReturnsExitError(t *testing.T) {
	withFakeHostTool(t, "gh", `echo boom 1>&2; exit 3`)
	_, err := CreatePullRequest(t.TempDir(), "gh", "feat/x", "main", "T", "B")
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("err = %v, want *ExitError", err)
	}
	if exitErr.ExitCode != 3 {
		t.Errorf("ExitCode = %d, want 3", exitErr.ExitCode)
	}
	if exitErr.TimedOut {
		t.Errorf("TimedOut = true, want false for a plain nonzero exit")
	}
}

func TestCreatePullRequest_UnsupportedHostTool_ErrorsWithoutShellingOut(t *testing.T) {
	_, err := CreatePullRequest(t.TempDir(), "hub", "feat/x", "main", "T", "B")
	if err == nil {
		t.Fatal("err = nil, want an error for an unsupported host tool")
	}
}

func TestRetargetPullRequest_GH_Argv(t *testing.T) {
	argvFile := captureArgvHostTool(t, "gh", `case "$2" in
  view) echo OPEN ;;
  edit) exit 0 ;;
esac`)

	if err := RetargetPullRequest(t.TempDir(), "gh", "123", "main"); err != nil {
		t.Fatalf("RetargetPullRequest: %v", err)
	}

	want := []string{"pr", "edit", "123", "--base", "main"}
	if got := readArgv(t, argvFile); !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %v, want %v", got, want)
	}
}

func TestRetargetPullRequest_Glab_Argv(t *testing.T) {
	argvFile := captureArgvHostTool(t, "glab", `case "$2" in
  view) echo '{"state": "opened"}' ;;
  update) exit 0 ;;
esac`)

	if err := RetargetPullRequest(t.TempDir(), "glab", "123", "main"); err != nil {
		t.Fatalf("RetargetPullRequest: %v", err)
	}

	want := []string{"mr", "update", "123", "--target-branch", "main"}
	if got := readArgv(t, argvFile); !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %v, want %v", got, want)
	}
}

func TestRetargetPullRequest_NonzeroExit_ReturnsExitError(t *testing.T) {
	withFakeHostTool(t, "gh", `case "$2" in
  view) echo OPEN ;;
  edit) exit 7 ;;
esac`)
	err := RetargetPullRequest(t.TempDir(), "gh", "123", "main")
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("err = %v, want *ExitError", err)
	}
	if exitErr.ExitCode != 7 {
		t.Errorf("ExitCode = %d, want 7", exitErr.ExitCode)
	}
}

func TestRetargetPullRequest_UnsupportedHostTool_ErrorsWithoutShellingOut(t *testing.T) {
	if err := RetargetPullRequest(t.TempDir(), "hub", "123", "main"); err == nil {
		t.Fatal("err = nil, want an error for an unsupported host tool")
	}
}

func TestRetargetPullRequest_ClosedState_Refused(t *testing.T) {
	argvFile := captureArgvHostTool(t, "gh", `case "$2" in
  view) echo CLOSED ;;
  edit) exit 0 ;;
esac`)

	err := RetargetPullRequest(t.TempDir(), "gh", "123", "main")
	if err == nil {
		t.Fatal("err = nil, want a refusal error for a closed PR")
	}
	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		t.Fatalf("err = %v (*ExitError), want a plain refusal error, not the host tool's own edit-rejection", err)
	}

	// argvFile is overwritten by each invocation of the fake binary; if it
	// still holds the view call's argv, the edit call never ran.
	want := []string{"pr", "view", "123", "--json", "state", "-q", ".state"}
	if got := readArgv(t, argvFile); !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %v, want %v (edit must never have been called)", got, want)
	}
}

func TestRetargetPullRequest_MergedState_Refused(t *testing.T) {
	argvFile := captureArgvHostTool(t, "glab", `case "$2" in
  view) echo '{"state": "merged"}' ;;
  update) exit 0 ;;
esac`)

	err := RetargetPullRequest(t.TempDir(), "glab", "123", "main")
	if err == nil {
		t.Fatal("err = nil, want a refusal error for a merged PR")
	}
	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		t.Fatalf("err = %v (*ExitError), want a plain refusal error", err)
	}

	want := []string{"mr", "view", "123", "-F", "json"}
	if got := readArgv(t, argvFile); !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %v, want %v (update must never have been called)", got, want)
	}
}

func TestRetargetPullRequest_StateLookupFails_Refused(t *testing.T) {
	argvFile := captureArgvHostTool(t, "gh", `case "$2" in
  view) exit 1 ;;
  edit) exit 0 ;;
esac`)

	err := RetargetPullRequest(t.TempDir(), "gh", "123", "main")
	if err == nil {
		t.Fatal("err = nil, want a refusal error when PR state cannot be determined")
	}
	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		t.Fatalf("err = %v (*ExitError), want a plain refusal error", err)
	}

	want := []string{"pr", "view", "123", "--json", "state", "-q", ".state"}
	if got := readArgv(t, argvFile); !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %v, want %v (edit must never have been called)", got, want)
	}
}

// TestCreatePullRequest_TimeoutKillsHungHostTool proves the 30s watchdog
// actually kills a hung gh/glab process rather than blocking forever, and
// that a timeout is reported distinctly (ExitError.TimedOut) from a
// generic non-zero exit. hostToolTimeout is shrunk for the duration of
// this test rather than waiting out a real 30s timeout.
func TestCreatePullRequest_TimeoutKillsHungHostTool(t *testing.T) {
	orig := hostToolTimeout
	hostToolTimeout = 20 * time.Millisecond
	t.Cleanup(func() { hostToolTimeout = orig })

	withFakeHostTool(t, "gh", `sleep 5`)

	start := time.Now()
	_, err := CreatePullRequest(t.TempDir(), "gh", "feat/x", "main", "T", "B")
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Fatalf("CreatePullRequest took %v, want it killed near the 20ms timeout instead of left hanging", elapsed)
	}
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("err = %v, want *ExitError", err)
	}
	if !exitErr.TimedOut {
		t.Errorf("TimedOut = false, want true: a hang must be reported distinctly from a normal nonzero exit")
	}
}

func TestRetargetPullRequest_TimeoutKillsHungHostTool(t *testing.T) {
	orig := hostToolTimeout
	// 300ms rather than the 20ms other single-call timeout tests use: the
	// state lookup (view) must itself complete inside the budget before the
	// edit call's hang is exercised, and even warmed up this still needs
	// more margin than a single already-hanging call does.
	hostToolTimeout = 300 * time.Millisecond
	t.Cleanup(func() { hostToolTimeout = orig })

	// Answers the state lookup (view) immediately with an open state, and
	// hangs only on the edit call (update) — this proves the edit-call
	// watchdog specifically, not the lookup call's. `exec sleep 5` (rather
	// than a plain `sleep 5`) replaces the shell process instead of forking
	// it, so killing the timed-out child on cancellation can't leave an
	// orphaned sleep holding the stdout/stderr pipe open behind it.
	withFakeHostTool(t, "glab", `case "$2" in
  view) echo '{"state": "opened"}' ;;
  update) exec sleep 5 ;;
esac`)
	warmFakeHostTool(t, "glab")

	start := time.Now()
	err := RetargetPullRequest(t.TempDir(), "glab", "123", "main")
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Fatalf("RetargetPullRequest took %v, want it killed near the 300ms timeout instead of left hanging", elapsed)
	}
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("err = %v, want *ExitError", err)
	}
	if !exitErr.TimedOut {
		t.Errorf("TimedOut = false, want true")
	}
}

// TestRetargetPullRequest_StateLookupTimeout_Refused proves a hung state
// lookup is refused via the plain-error path, not *ExitError: PRState uses
// runUnchecked internally, which collapses a timeout into an ordinary
// lookup failure (ok == false) rather than propagating a timeout
// *ExitError the way the edit call's own watchdog does (see
// TestRetargetPullRequest_TimeoutKillsHungHostTool above).
func TestRetargetPullRequest_StateLookupTimeout_Refused(t *testing.T) {
	orig := hostToolTimeout
	hostToolTimeout = 20 * time.Millisecond
	t.Cleanup(func() { hostToolTimeout = orig })

	withFakeHostTool(t, "gh", `case "$2" in
  view) exec sleep 5 ;;
  edit) exit 0 ;;
esac`)

	start := time.Now()
	err := RetargetPullRequest(t.TempDir(), "gh", "123", "main")
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Fatalf("RetargetPullRequest took %v, want it killed near the 20ms timeout instead of left hanging", elapsed)
	}
	if err == nil {
		t.Fatal("err = nil, want a refusal error when the state lookup hangs")
	}
	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		t.Fatalf("err = %v (*ExitError), want a plain refusal error: a lookup timeout collapses into ok=false, not a propagated ExitError", err)
	}
}
