// preflight.go is a Go port of preflight.py's repo/remote/host-tool
// preconditions (run_preflight and its helpers). internal/gitops has an
// equivalent git-subprocess helper, but it's unexported to that package —
// see this ticket's own scope note — so this file (and dirty.go, which
// shares gitOutput) has its own small one, matching
// internal/cli/loop/branch_test.go's own local runLoopGit test helper.
package loop

import (
	"context"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// hostToolTimeout mirrors run_state.py's HOST_TOOL_TIMEOUT_SECONDS = 30,
// applied to the one gh/glab round trip preflight makes (auth status): an
// unattended, self-pacing loop has no one to notice a hang.
const hostToolTimeout = 30 * time.Second

// gitOutput runs git with args in dir, returning raw stdout and whether
// it exited zero — mirrors preflight.py's bare subprocess.run(...,
// capture_output=True, text=True) calls (no check=True): callers decide
// how a non-zero exit is handled, matching each Python call site's own
// behavior. Output is untrimmed: dirty_paths' git-status callers need
// each line's leading status-code column intact (e.g. " M file.txt"),
// which a blanket TrimSpace on the whole blob would corrupt; callers that
// want a single trimmed value (a ref name, a branch name) trim it
// themselves.
func gitOutput(dir string, args ...string) (stdout string, ok bool) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return string(out), err == nil
}

func gitRemoteURL(repoRoot string) string {
	url, ok := gitOutput(repoRoot, "remote", "get-url", "origin")
	if !ok {
		return ""
	}
	return strings.TrimSpace(url)
}

// resolveTrunkBranch mirrors preflight.py's _resolve_trunk_branch:
// origin/HEAD when resolvable, else a local main/master, else the current
// branch.
func resolveTrunkBranch(repoRoot string) string {
	if ref, ok := gitOutput(repoRoot, "symbolic-ref", "refs/remotes/origin/HEAD"); ok {
		ref = strings.TrimSpace(ref)
		const prefix = "refs/remotes/origin/"
		if strings.HasPrefix(ref, prefix) {
			return strings.TrimPrefix(ref, prefix)
		}
	}
	for _, candidate := range []string{"main", "master"} {
		if _, ok := gitOutput(repoRoot, "show-ref", "--verify", "--quiet", "refs/heads/"+candidate); ok {
			return candidate
		}
	}
	current, _ := gitOutput(repoRoot, "branch", "--show-current")
	return strings.TrimSpace(current)
}

// branchRefExists mirrors preflight.py's _branch_ref_exists: a local ref
// or remote-tracking ref covers both a checked-out branch and a fresh
// clone that only fetched it.
func branchRefExists(repoRoot, branch string) bool {
	for _, ref := range []string{"refs/heads/" + branch, "refs/remotes/origin/" + branch} {
		if _, ok := gitOutput(repoRoot, "show-ref", "--verify", "--quiet", ref); ok {
			return true
		}
	}
	return false
}

// remoteURLHostRe/remoteSCPHostRe mirror preflight.py's
// _REMOTE_URL_HOST_RE/_REMOTE_SCP_HOST_RE, covering both URL
// (scheme://host/...) and scp-like (git@host:owner/repo) remote syntax.
var (
	remoteURLHostRe = regexp.MustCompile(`^[a-zA-Z][\w+.-]*://(?:[^@/]*@)?([^/:]+)`)
	remoteSCPHostRe = regexp.MustCompile(`^(?:[^@/]*@)?([^/:]+):`)
)

// gitHostFromRemote mirrors preflight.py's _git_host_from_remote: "" for
// local/bare paths, which have neither an authority nor a colon-prefixed
// host segment.
func gitHostFromRemote(remoteURL string) string {
	if remoteURL == "" {
		return ""
	}
	m := remoteURLHostRe.FindStringSubmatch(remoteURL)
	if m == nil {
		m = remoteSCPHostRe.FindStringSubmatch(remoteURL)
	}
	if m == nil {
		return ""
	}
	return strings.ToLower(m[1])
}

// preferredHostTool mirrors preflight.py's _preferred_host_tool.
func preferredHostTool(remoteURL string) string {
	switch host := gitHostFromRemote(remoteURL); {
	case host == "":
		return ""
	case strings.Contains(host, "gitlab"):
		return "glab"
	case strings.Contains(host, "github"):
		return "gh"
	default:
		return ""
	}
}

// detectHostTool mirrors preflight.py's _detect_host_tool: when origin's
// host classifies as GitHub/GitLab, that tool is required and never
// silently substituted with whichever tool happens to be on PATH — that
// would run a PR create/lookup against the wrong host. Falls back to PATH
// order only when the host can't be classified.
func detectHostTool(remoteURL string) string {
	if preferred := preferredHostTool(remoteURL); preferred != "" {
		if _, err := exec.LookPath(preferred); err == nil {
			return preferred
		}
		return ""
	}
	for _, tool := range []string{"gh", "glab"} {
		if _, err := exec.LookPath(tool); err == nil {
			return tool
		}
	}
	return ""
}

// hostToolAuthenticated mirrors preflight.py's _host_tool_authenticated.
func hostToolAuthenticated(tool string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), hostToolTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, tool, "auth", "status")
	return cmd.Run() == nil
}

// preflightResult is cmd_preflight's JSON contract:
// {"ok", "remote_url", "trunk_branch", "host_tool", "failures"}.
type preflightResult struct {
	OK          bool     `json:"ok"`
	RemoteURL   *string  `json:"remote_url"`
	TrunkBranch *string  `json:"trunk_branch"`
	HostTool    *string  `json:"host_tool"`
	Failures    []string `json:"failures"`
}

// runPreflight mirrors preflight.py's run_preflight: preconditions — tk
// present, remote configured, clean tree, and (only when PR creation will
// run) an authenticated gh/glab. Never counted against the failure cap:
// this fails the whole run, not one ticket.
//
// trunkBranchOverride, when non-empty, replaces resolveTrunkBranch's
// autodetection outright rather than feeding into it — git has no signal
// for which branch a repo actually treats as trunk when that diverges
// from origin/HEAD or main/master, so this is a caller-supplied answer,
// not a smarter heuristic.
//
// A dirty_paths failure (e.g. repo_root isn't a git repository at all) is
// a hard error here rather than folded into failures: it means preflight
// itself couldn't run, not that a precondition was unmet.
func runPreflight(repoRoot string, willCreatePRs bool, trunkBranchOverride string) (preflightResult, error) {
	var failures []string

	if _, err := exec.LookPath("tk"); err != nil {
		failures = append(failures, "tk (ticket) not found on PATH")
	}

	remoteURL := gitRemoteURL(repoRoot)
	if remoteURL == "" {
		failures = append(failures, "no git remote 'origin' configured")
	}

	dirty, err := dirtyPaths(repoRoot)
	if err != nil {
		return preflightResult{}, err
	}
	if len(dirty) > 0 {
		failures = append(failures, "working tree is dirty: "+strings.Join(dirty, ", "))
	}

	var hostTool string
	if willCreatePRs {
		hostTool = detectHostTool(remoteURL)
		if hostTool == "" {
			if required := preferredHostTool(remoteURL); required != "" {
				failures = append(failures, "origin host requires "+required+", but it is not found on PATH")
			} else {
				failures = append(failures, "neither gh nor glab found on PATH")
			}
		} else if !hostToolAuthenticated(hostTool) {
			failures = append(failures, hostTool+" is not authenticated (run `"+hostTool+" auth login`)")
		}
	}

	var trunkBranch string
	switch {
	case trunkBranchOverride != "":
		if branchRefExists(repoRoot, trunkBranchOverride) {
			trunkBranch = trunkBranchOverride
		} else {
			failures = append(failures, "trunk-branch override '"+trunkBranchOverride+
				"' not found as refs/heads/"+trunkBranchOverride+" or refs/remotes/origin/"+trunkBranchOverride)
		}
	case len(failures) == 0:
		trunkBranch = resolveTrunkBranch(repoRoot)
	}

	return preflightResult{
		OK:          len(failures) == 0,
		RemoteURL:   strPtrOrNil(remoteURL),
		TrunkBranch: strPtrOrNil(trunkBranch),
		HostTool:    strPtrOrNil(hostTool),
		Failures:    nonNilStrings(failures),
	}, nil
}

func NewPreflightCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "preflight <repo-root> <true|false> [trunk-branch]",
		Short: "Run pre-run checks before the execution loop starts",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			willCreatePRs := strings.ToLower(args[1]) == "true"
			var trunkOverride string
			if len(args) > 2 {
				trunkOverride = args[2]
			}
			result, err := runPreflight(args[0], willCreatePRs, trunkOverride)
			if err != nil {
				return err
			}
			return printJSON(cmd, result)
		},
	}
}
