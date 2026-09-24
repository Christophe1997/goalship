package gitops

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// ---- ValidateOpenPRGraph ----

func TestValidateOpenPRGraph_LinearStack_NoError(t *testing.T) {
	prs := []OpenPR{
		{Number: 1, Branch: "feat/a", Base: "main"},
		{Number: 2, Branch: "feat/b", Base: "feat/a"},
		{Number: 3, Branch: "feat/c", Base: "feat/b"},
	}
	if err := ValidateOpenPRGraph(prs); err != nil {
		t.Errorf("err = %v, want nil for a linear stack", err)
	}
}

func TestValidateOpenPRGraph_Cycle_Errors(t *testing.T) {
	prs := []OpenPR{
		{Number: 1, Branch: "feat/a", Base: "feat/b"},
		{Number: 2, Branch: "feat/b", Base: "feat/a"},
	}
	err := ValidateOpenPRGraph(prs)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("err = %v, want an error naming a cycle", err)
	}
	if !strings.Contains(err.Error(), "feat/a") || !strings.Contains(err.Error(), "feat/b") {
		t.Errorf("err = %v, want the cycle's branches named", err)
	}
}

func TestValidateOpenPRGraph_SelfReferential_Errors(t *testing.T) {
	prs := []OpenPR{{Number: 1, Branch: "feat/a", Base: "feat/a"}}
	err := ValidateOpenPRGraph(prs)
	if err == nil || !strings.Contains(err.Error(), "self-referential") {
		t.Fatalf("err = %v, want an error naming self-reference", err)
	}
}

func TestValidateOpenPRGraph_EmptyBranch_Errors(t *testing.T) {
	prs := []OpenPR{{Number: 1, Branch: "", Base: "main"}}
	err := ValidateOpenPRGraph(prs)
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("err = %v, want an error naming an empty field", err)
	}
}

func TestValidateOpenPRGraph_EmptyBase_Errors(t *testing.T) {
	prs := []OpenPR{{Number: 1, Branch: "feat/a", Base: ""}}
	err := ValidateOpenPRGraph(prs)
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("err = %v, want an error naming an empty field", err)
	}
}

func TestValidateOpenPRGraph_SharedHeadBranch_NoOtherBaseReferencesIt_NoError(t *testing.T) {
	prs := []OpenPR{
		{Number: 1, Branch: "feat/x", Base: "main"},
		{Number: 2, Branch: "feat/x", Base: "develop"},
	}
	if err := ValidateOpenPRGraph(prs); err != nil {
		t.Errorf("err = %v, want nil: no PR's base references the shared branch", err)
	}
}

func TestValidateOpenPRGraph_FanInAmbiguity_Errors(t *testing.T) {
	prs := []OpenPR{
		{Number: 1, Branch: "feat/x", Base: "main"},
		{Number: 2, Branch: "feat/x", Base: "develop"},
		{Number: 3, Branch: "feat/y", Base: "feat/x"},
	}
	err := ValidateOpenPRGraph(prs)
	if err == nil || !strings.Contains(err.Error(), "fan-in") {
		t.Fatalf("err = %v, want an error naming fan-in ambiguity", err)
	}
}

// ---- TopologicalMergeOrder ----

func TestTopologicalMergeOrder_LinearStack_RootFirst(t *testing.T) {
	top := OpenPR{Number: 3, Branch: "feat/c", Base: "feat/b"}
	root := OpenPR{Number: 1, Branch: "feat/a", Base: "main"}
	middle := OpenPR{Number: 2, Branch: "feat/b", Base: "feat/a"}
	prs := []OpenPR{top, root, middle} // scrambled: top first, root in the middle

	got, err := TopologicalMergeOrder(prs)
	if err != nil {
		t.Fatalf("TopologicalMergeOrder: %v", err)
	}
	want := []OpenPR{root, middle, top}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got = %+v, want %+v", got, want)
	}
}

func TestTopologicalMergeOrder_TwoIndependentStacks_BothRootsFirst(t *testing.T) {
	a1 := OpenPR{Number: 1, Branch: "feat/a1", Base: "main"}
	a2 := OpenPR{Number: 2, Branch: "feat/a2", Base: "feat/a1"}
	b1 := OpenPR{Number: 3, Branch: "feat/b1", Base: "main"}
	b2 := OpenPR{Number: 4, Branch: "feat/b2", Base: "feat/b1"}
	prs := []OpenPR{b2, a2, a1, b1} // interleaved, tops of both stacks first

	got, err := TopologicalMergeOrder(prs)
	if err != nil {
		t.Fatalf("TopologicalMergeOrder: %v", err)
	}
	want := []OpenPR{a1, b1, a2, b2}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got = %+v, want %+v", got, want)
	}
}

func TestTopologicalMergeOrder_ZeroOpenPRs_ReturnsEmptyOrder(t *testing.T) {
	got, err := TopologicalMergeOrder(nil)
	if err != nil {
		t.Fatalf("TopologicalMergeOrder: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len(got) = %d, want 0", len(got))
	}
}

func TestTopologicalMergeOrder_Cycle_PropagatesValidationErrorUnchanged(t *testing.T) {
	prs := []OpenPR{
		{Number: 1, Branch: "feat/a", Base: "feat/b"},
		{Number: 2, Branch: "feat/b", Base: "feat/a"},
	}
	wantErr := ValidateOpenPRGraph(prs)
	if wantErr == nil {
		t.Fatal("ValidateOpenPRGraph: want a cycle error for this fixture, got nil")
	}

	got, err := TopologicalMergeOrder(prs)
	if got != nil {
		t.Errorf("got = %+v, want nil order on error", got)
	}
	if err == nil || err.Error() != wantErr.Error() {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

func TestTopologicalMergeOrder_FanInAmbiguity_PropagatesValidationErrorUnchanged(t *testing.T) {
	prs := []OpenPR{
		{Number: 1, Branch: "feat/x", Base: "main"},
		{Number: 2, Branch: "feat/x", Base: "develop"},
		{Number: 3, Branch: "feat/y", Base: "feat/x"},
	}
	wantErr := ValidateOpenPRGraph(prs)
	if wantErr == nil {
		t.Fatal("ValidateOpenPRGraph: want a fan-in ambiguity error for this fixture, got nil")
	}

	got, err := TopologicalMergeOrder(prs)
	if got != nil {
		t.Errorf("got = %+v, want nil order on error", got)
	}
	if err == nil || err.Error() != wantErr.Error() {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

// ---- ListOpenPRs: gh ----

func TestListOpenPRs_GH_ArgvAndParse(t *testing.T) {
	argvFile := captureArgvHostTool(t, "gh", `echo '[`+
		`{"number":1,"url":"https://github.com/o/r/pull/1","baseRefName":"main","headRefName":"feat/a"},`+
		`{"number":2,"url":"https://github.com/o/r/pull/2","baseRefName":"feat/a","headRefName":"feat/b"}`+
		`]'`)

	prs, err := ListOpenPRs(t.TempDir(), "gh")
	if err != nil {
		t.Fatalf("ListOpenPRs: %v", err)
	}
	want := []OpenPR{
		{Number: 1, URL: "https://github.com/o/r/pull/1", Branch: "feat/a", Base: "main"},
		{Number: 2, URL: "https://github.com/o/r/pull/2", Branch: "feat/b", Base: "feat/a"},
	}
	if !reflect.DeepEqual(prs, want) {
		t.Errorf("prs = %+v, want %+v", prs, want)
	}

	wantArgv := []string{"pr", "list", "--state", "open", "--json", "number,url,baseRefName,headRefName", "--limit", "1000"}
	if got := readArgv(t, argvFile); !reflect.DeepEqual(got, wantArgv) {
		t.Errorf("argv = %v, want %v", got, wantArgv)
	}
}

func TestListOpenPRs_GH_ZeroOpenPRs_ReturnsEmptySlice(t *testing.T) {
	withFakeHostTool(t, "gh", `echo '[]'`)
	prs, err := ListOpenPRs(t.TempDir(), "gh")
	if err != nil {
		t.Fatalf("ListOpenPRs: %v", err)
	}
	if len(prs) != 0 {
		t.Errorf("len(prs) = %d, want 0", len(prs))
	}
}

// ghOpenPRsJSON builds a gh-shaped JSON array of n well-formed open PRs
// starting at number 1, for exercising the truncation guard without hand
// writing a 1000-element literal.
func ghOpenPRsJSON(n int) string {
	var sb strings.Builder
	sb.WriteByte('[')
	for i := 1; i <= n; i++ {
		if i > 1 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, `{"number":%d,"url":"https://github.com/o/r/pull/%d","baseRefName":"main","headRefName":"feat/%d"}`, i, i, i)
	}
	sb.WriteByte(']')
	return sb.String()
}

func TestListOpenPRs_GH_ExactlyLimitResults_ReturnsTruncationError(t *testing.T) {
	withFakeHostTool(t, "gh", "echo '"+ghOpenPRsJSON(1000)+"'")
	_, err := ListOpenPRs(t.TempDir(), "gh")
	if err == nil || !strings.Contains(err.Error(), "truncat") {
		t.Fatalf("err = %v, want an error mentioning truncation", err)
	}
}

func TestListOpenPRs_GH_NonzeroExit_ReturnsExitError(t *testing.T) {
	withFakeHostTool(t, "gh", `echo boom 1>&2; exit 3`)
	_, err := ListOpenPRs(t.TempDir(), "gh")
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("err = %v, want *ExitError", err)
	}
	if exitErr.ExitCode != 3 {
		t.Errorf("ExitCode = %d, want 3", exitErr.ExitCode)
	}
}

// ---- ListOpenPRs: glab ----

// glabMRsJSON builds a glab-shaped JSON array of n well-formed open MRs,
// with iid starting at startIID.
func glabMRsJSON(startIID, n int) string {
	var sb strings.Builder
	sb.WriteByte('[')
	for i := 0; i < n; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		iid := startIID + i
		fmt.Fprintf(&sb, `{"iid":%d,"web_url":"https://gitlab.com/o/r/-/merge_requests/%d","source_branch":"feat/%d","target_branch":"main"}`, iid, iid, iid)
	}
	sb.WriteByte(']')
	return sb.String()
}

func TestListOpenPRs_Glab_ArgvAndParse(t *testing.T) {
	argvFile := captureArgvHostTool(t, "glab", `echo '`+glabMRsJSON(1, 2)+`'`)

	prs, err := ListOpenPRs(t.TempDir(), "glab")
	if err != nil {
		t.Fatalf("ListOpenPRs: %v", err)
	}
	want := []OpenPR{
		{Number: 1, URL: "https://gitlab.com/o/r/-/merge_requests/1", Branch: "feat/1", Base: "main"},
		{Number: 2, URL: "https://gitlab.com/o/r/-/merge_requests/2", Branch: "feat/2", Base: "main"},
	}
	if !reflect.DeepEqual(prs, want) {
		t.Errorf("prs = %+v, want %+v", prs, want)
	}

	wantArgv := []string{"mr", "list", "-F", "json", "--per-page", "100", "--page", "1"}
	if got := readArgv(t, argvFile); !reflect.DeepEqual(got, wantArgv) {
		t.Errorf("argv = %v, want %v", got, wantArgv)
	}
}

func TestListOpenPRs_Glab_ZeroOpenPRs_ReturnsEmptySlice(t *testing.T) {
	withFakeHostTool(t, "glab", `echo '[]'`)
	prs, err := ListOpenPRs(t.TempDir(), "glab")
	if err != nil {
		t.Fatalf("ListOpenPRs: %v", err)
	}
	if len(prs) != 0 {
		t.Errorf("len(prs) = %d, want 0", len(prs))
	}
}

func TestListOpenPRs_Glab_FullPagePaginatesToNext_ShortPageStops(t *testing.T) {
	page1 := glabMRsJSON(1, 100)
	page2 := glabMRsJSON(101, 3)
	script := `case "$*" in
  *"--page 1"*) echo '` + page1 + `' ;;
  *"--page 2"*) echo '` + page2 + `' ;;
  *) exit 1 ;;
esac
`
	argvFile := captureArgvHostTool(t, "glab", script)

	prs, err := ListOpenPRs(t.TempDir(), "glab")
	if err != nil {
		t.Fatalf("ListOpenPRs: %v", err)
	}
	if len(prs) != 103 {
		t.Fatalf("len(prs) = %d, want 103 (100 + 3 across two pages)", len(prs))
	}
	if prs[0].Number != 1 || prs[102].Number != 103 {
		t.Errorf("prs[0].Number = %d, prs[102].Number = %d, want 1 and 103", prs[0].Number, prs[102].Number)
	}

	// The last invocation captured must be the page-2 request: proves
	// the 100-result first page triggered another round-trip instead of
	// stopping.
	wantArgv := []string{"mr", "list", "-F", "json", "--per-page", "100", "--page", "2"}
	if got := readArgv(t, argvFile); !reflect.DeepEqual(got, wantArgv) {
		t.Errorf("last argv = %v, want %v", got, wantArgv)
	}
}

func TestListOpenPRs_Glab_NonzeroExit_ReturnsExitError(t *testing.T) {
	withFakeHostTool(t, "glab", `echo boom 1>&2; exit 5`)
	_, err := ListOpenPRs(t.TempDir(), "glab")
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("err = %v, want *ExitError", err)
	}
	if exitErr.ExitCode != 5 {
		t.Errorf("ExitCode = %d, want 5", exitErr.ExitCode)
	}
}

// TestListOpenPRs_Glab_TimeoutKillsHungHostTool proves the per-page
// hostToolTimeout actually bounds each glab round-trip: a page that hangs
// is killed and reported as *ExitError.TimedOut rather than left to block
// the whole pagination loop forever.
func TestListOpenPRs_Glab_TimeoutKillsHungHostTool(t *testing.T) {
	orig := hostToolTimeout
	hostToolTimeout = 20 * time.Millisecond
	t.Cleanup(func() { hostToolTimeout = orig })

	withFakeHostTool(t, "glab", `sleep 5`)

	start := time.Now()
	_, err := ListOpenPRs(t.TempDir(), "glab")
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Fatalf("ListOpenPRs took %v, want it killed near the 20ms timeout instead of left hanging", elapsed)
	}
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("err = %v, want *ExitError", err)
	}
	if !exitErr.TimedOut {
		t.Errorf("TimedOut = false, want true: a hang must be reported distinctly from a normal nonzero exit")
	}
}

// ---- ListOpenPRs: unsupported host tool ----

func TestListOpenPRs_UnsupportedHostTool_ErrorsWithoutShellingOut(t *testing.T) {
	_, err := ListOpenPRs(t.TempDir(), "hub")
	if err == nil || !strings.Contains(err.Error(), "unsupported host tool") {
		t.Fatalf("err = %v, want an error naming the unsupported host tool", err)
	}
}
