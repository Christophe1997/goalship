package gitops

import (
	"context"
	"os/exec"
	"regexp"
	"strings"
)

// gitRemoteURL returns `git remote get-url origin`, trimmed, or "" on any
// failure (no remote configured, not a git repo, etc.) — mirrors
// preflight.py's _git_remote_url. internal/cli/loop/preflight.go carries an
// equivalent os/exec-backed copy on a still-unmerged branch this ticket has
// no dependency edge to; this is a deliberate second copy using this
// package's own run/runUnchecked subprocess conventions instead.
func gitRemoteURL(repoRoot string) string {
	out, err := run(repoRoot, "git", "remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

var (
	remoteURLHostRe = regexp.MustCompile(`^[a-zA-Z][\w+.-]*://(?:[^@/]*@)?([^/:]+)`)
	remoteSCPHostRe = regexp.MustCompile(`^(?:[^@/]*@)?([^/:]+):`)
)

// gitHostFromRemote extracts the lowercase host from a remote URL, covering
// both "https://host/owner/repo" and "git@host:owner/repo" remote syntax —
// mirrors preflight.py's _git_host_from_remote.
func gitHostFromRemote(remoteURL string) string {
	if remoteURL == "" {
		return ""
	}
	if m := remoteURLHostRe.FindStringSubmatch(remoteURL); m != nil {
		return strings.ToLower(m[1])
	}
	if m := remoteSCPHostRe.FindStringSubmatch(remoteURL); m != nil {
		return strings.ToLower(m[1])
	}
	return ""
}

// preferredHostTool maps a remote's host to the CLI it implies — mirrors
// preflight.py's _preferred_host_tool.
func preferredHostTool(remoteURL string) string {
	host := gitHostFromRemote(remoteURL)
	switch {
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

// detectHostTool picks which of gh/glab to use for remoteURL: the
// host-classified tool if it's actually on PATH — never silently
// substituting the other tool when the host is classified but that
// specific tool is missing — else the first of gh/glab found on PATH when
// the host is unclassified, or "" when neither is available. Mirrors
// preflight.py's _detect_host_tool.
func detectHostTool(remoteURL string) string {
	if preferred := preferredHostTool(remoteURL); preferred != "" {
		if _, err := exec.LookPath(preferred); err != nil {
			return ""
		}
		return preferred
	}
	for _, tool := range []string{"gh", "glab"} {
		if _, err := exec.LookPath(tool); err == nil {
			return tool
		}
	}
	return ""
}

// hostToolAuthenticated reports whether tool has a working credential —
// `<tool> auth status` exiting zero — timing out after hostToolTimeout like
// every other gh/glab round-trip this package makes. Mirrors preflight.py's
// _host_tool_authenticated.
func hostToolAuthenticated(tool string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), hostToolTimeout)
	defer cancel()
	_, code := runUnchecked(ctx, "", tool, "auth", "status")
	return code == 0
}
