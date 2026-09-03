package gitops

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	nonAlnumRE    = regexp.MustCompile(`[^a-z0-9]+`)
	multiHyphenRE = regexp.MustCompile(`-{2,}`)
)

// Slugify lowercases text and collapses every run of non-alphanumeric
// characters into a single hyphen, trimming leading/trailing hyphens —
// mirrors branching.py's slugify(). An all-punctuation title slugifies to
// "" (and BranchName then produces a bare "<type>/"), the same hole the
// Python original has; this is a faithful port, not a place to add
// validation the source doesn't have.
func Slugify(text string) string {
	slug := nonAlnumRE.ReplaceAllString(strings.ToLower(text), "-")
	slug = strings.Trim(slug, "-")
	return multiHyphenRE.ReplaceAllString(slug, "-")
}

// allBranchNames returns every local branch name plus origin/* remote-
// tracking branch names, short-formed — so a branch that exists only on
// origin (left by a prior run) still counts as taken.
func allBranchNames(repoRoot string) (map[string]struct{}, error) {
	out, err := run(repoRoot, "git", "branch", "-a", "--format=%(refname:short)")
	if err != nil {
		return nil, err
	}
	names := make(map[string]struct{})
	for _, line := range strings.Split(out, "\n") {
		name := strings.TrimSpace(line)
		name = strings.TrimPrefix(name, "origin/")
		if name != "" && name != "HEAD" {
			names[name] = struct{}{}
		}
	}
	return names, nil
}

// BranchName computes "<ticketType>/<slug(title)>", appending a numeric
// suffix on collision against local and origin/ refs — mirrors
// branching.py's branch_name_for_ticket.
func BranchName(repoRoot, ticketType, title string) (string, error) {
	base := ticketType + "/" + Slugify(title)
	existing, err := allBranchNames(repoRoot)
	if err != nil {
		return "", err
	}
	if _, taken := existing[base]; !taken {
		return base, nil
	}
	for suffix := 2; ; suffix++ {
		candidate := fmt.Sprintf("%s-%d", base, suffix)
		if _, taken := existing[candidate]; !taken {
			return candidate, nil
		}
	}
}
