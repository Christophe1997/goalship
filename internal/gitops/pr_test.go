package gitops

import (
	"errors"
	"os"
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
	argvFile := captureArgvHostTool(t, "gh", `exit 0`)

	if err := RetargetPullRequest(t.TempDir(), "gh", "123", "main"); err != nil {
		t.Fatalf("RetargetPullRequest: %v", err)
	}

	want := []string{"pr", "edit", "123", "--base", "main"}
	if got := readArgv(t, argvFile); !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %v, want %v", got, want)
	}
}

func TestRetargetPullRequest_Glab_Argv(t *testing.T) {
	argvFile := captureArgvHostTool(t, "glab", `exit 0`)

	if err := RetargetPullRequest(t.TempDir(), "glab", "123", "main"); err != nil {
		t.Fatalf("RetargetPullRequest: %v", err)
	}

	want := []string{"mr", "update", "123", "--target-branch", "main"}
	if got := readArgv(t, argvFile); !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %v, want %v", got, want)
	}
}

func TestRetargetPullRequest_NonzeroExit_ReturnsExitError(t *testing.T) {
	withFakeHostTool(t, "gh", `exit 7`)
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
	hostToolTimeout = 20 * time.Millisecond
	t.Cleanup(func() { hostToolTimeout = orig })

	withFakeHostTool(t, "glab", `sleep 5`)

	start := time.Now()
	err := RetargetPullRequest(t.TempDir(), "glab", "123", "main")
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Fatalf("RetargetPullRequest took %v, want it killed near the 20ms timeout instead of left hanging", elapsed)
	}
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("err = %v, want *ExitError", err)
	}
	if !exitErr.TimedOut {
		t.Errorf("TimedOut = false, want true")
	}
}
