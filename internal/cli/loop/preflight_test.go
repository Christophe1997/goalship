package loop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type preflightJSON struct {
	OK          bool     `json:"ok"`
	RemoteURL   *string  `json:"remote_url"`
	TrunkBranch *string  `json:"trunk_branch"`
	HostTool    *string  `json:"host_tool"`
	Failures    []string `json:"failures"`
}

func addOrigin(t *testing.T, repoRoot, url string) {
	t.Helper()
	runLoopGit(t, repoRoot, "remote", "add", "origin", url)
}

// TestPreflightCmd_OK_WithConfiguredRemoteAndLocalMain exercises the "ok"
// path: a configured origin and a local main branch, will-create-prs
// false so no gh/glab auth round trip runs (that path would be flaky by
// sandbox/CI environment — see TestRunPreflight_WillCreatePRs_* below for
// the deterministic, subprocess-free coverage of that logic instead).
func TestPreflightCmd_OK_WithConfiguredRemoteAndLocalMain(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	addOrigin(t, repoRoot, "https://github.com/example/repo.git")

	out := execCmd(t, NewPreflightCmd(), []string{repoRoot, "false"})

	var got preflightJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal output %q: %v", out, err)
	}
	if !got.OK {
		t.Errorf("ok = false, want true (failures: %v)", got.Failures)
	}
	if got.RemoteURL == nil || *got.RemoteURL != "https://github.com/example/repo.git" {
		t.Errorf("remote_url = %v, want the configured origin", got.RemoteURL)
	}
	if got.TrunkBranch == nil || *got.TrunkBranch != "main" {
		t.Errorf("trunk_branch = %v, want \"main\"", got.TrunkBranch)
	}
	if got.HostTool != nil {
		t.Errorf("host_tool = %v, want null (will-create-prs was false)", got.HostTool)
	}
}

func TestPreflightCmd_UnresolvableOverrideBranch_ReportsFailureNotFallback(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	addOrigin(t, repoRoot, "https://github.com/example/repo.git")

	out := execCmd(t, NewPreflightCmd(), []string{repoRoot, "false", "does-not-exist"})

	var got preflightJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal output %q: %v", out, err)
	}
	if got.OK {
		t.Error("ok = true, want false for an unresolvable trunk-branch override")
	}
	if got.TrunkBranch != nil {
		t.Errorf("trunk_branch = %v, want null (override must not silently fall back to autodetection)", got.TrunkBranch)
	}
	found := false
	for _, f := range got.Failures {
		if f == "trunk-branch override 'does-not-exist' not found as refs/heads/does-not-exist or refs/remotes/origin/does-not-exist" {
			found = true
		}
	}
	if !found {
		t.Errorf("failures = %v, want an entry naming the unresolvable override", got.Failures)
	}
}

func TestPreflightCmd_NoRemote_ReportsFailure(t *testing.T) {
	repoRoot := newLoopTestRepo(t)

	out := execCmd(t, NewPreflightCmd(), []string{repoRoot, "false"})

	var got preflightJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal output %q: %v", out, err)
	}
	if got.OK {
		t.Error("ok = true, want false with no origin configured")
	}
	if got.RemoteURL != nil {
		t.Errorf("remote_url = %v, want null", got.RemoteURL)
	}
}

func TestPreflightCmd_DirtyWorkingTree_ReportsFailure(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	addOrigin(t, repoRoot, "https://github.com/example/repo.git")
	if err := os.WriteFile(filepath.Join(repoRoot, "dirty.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write dirty.txt: %v", err)
	}

	out := execCmd(t, NewPreflightCmd(), []string{repoRoot, "false"})

	var got preflightJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal output %q: %v", out, err)
	}
	if got.OK {
		t.Error("ok = true, want false with a dirty working tree")
	}
}

// --- pure-function coverage for host-tool detection, deterministic and
// subprocess-free (see the advisor note this test file's docstring
// references above: never let host-tool auth gate a repo-fixture test).

func TestGitHostFromRemote(t *testing.T) {
	cases := []struct {
		remote string
		want   string
	}{
		{"https://github.com/foo/bar.git", "github.com"},
		{"git@github.com:foo/bar.git", "github.com"},
		{"https://gitlab.example.com/foo/bar.git", "gitlab.example.com"},
		{"git@gitlab.example.com:foo/bar.git", "gitlab.example.com"},
		{"HTTPS://GitHub.com/foo/bar.git", "github.com"},
		{"/local/bare/repo.git", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := gitHostFromRemote(tc.remote); got != tc.want {
			t.Errorf("gitHostFromRemote(%q) = %q, want %q", tc.remote, got, tc.want)
		}
	}
}

func TestPreferredHostTool(t *testing.T) {
	cases := []struct {
		remote string
		want   string
	}{
		{"https://github.com/foo/bar.git", "gh"},
		{"git@gitlab.com:foo/bar.git", "glab"},
		{"https://bitbucket.org/foo/bar.git", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := preferredHostTool(tc.remote); got != tc.want {
			t.Errorf("preferredHostTool(%q) = %q, want %q", tc.remote, got, tc.want)
		}
	}
}
