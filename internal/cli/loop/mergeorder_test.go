package loop

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestMergeOrderCmd_PrintsLandOrderJSON(t *testing.T) {
	withFakeHostTool(t, "gh", `echo '[`+
		`{"number":2,"url":"https://github.com/o/r/pull/2","baseRefName":"feat/a","headRefName":"feat/b"},`+
		`{"number":1,"url":"https://github.com/o/r/pull/1","baseRefName":"main","headRefName":"feat/a"}`+
		`]'`)
	repoRoot := newLoopTestRepo(t)

	out := execCmd(t, NewMergeOrderCmd(), []string{repoRoot, "gh"})

	want := `[{"number":1,"url":"https://github.com/o/r/pull/1","branch":"feat/a","base":"main"},` +
		`{"number":2,"url":"https://github.com/o/r/pull/2","branch":"feat/b","base":"feat/a"}]`
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(out)); err != nil {
		t.Fatalf("json.Compact(%q): %v", out, err)
	}
	if compact.String() != want {
		t.Errorf("output = %s, want %s", compact.String(), want)
	}
}

func TestMergeOrderCmd_ZeroOpenPRs_PrintsEmptyArray(t *testing.T) {
	withFakeHostTool(t, "gh", `echo '[]'`)
	repoRoot := newLoopTestRepo(t)

	out := execCmd(t, NewMergeOrderCmd(), []string{repoRoot, "gh"})
	if strings.TrimSpace(out) != "[]" {
		t.Errorf("output = %q, want %q", out, "[]")
	}
}

func TestMergeOrderCmd_CyclicGraph_ErrorsWithoutPrintingJSON(t *testing.T) {
	withFakeHostTool(t, "gh", `echo '[`+
		`{"number":1,"url":"https://github.com/o/r/pull/1","baseRefName":"feat/b","headRefName":"feat/a"},`+
		`{"number":2,"url":"https://github.com/o/r/pull/2","baseRefName":"feat/a","headRefName":"feat/b"}`+
		`]'`)
	repoRoot := newLoopTestRepo(t)

	cmd := NewMergeOrderCmd()
	outBuf := &strings.Builder{}
	errBuf := &strings.Builder{}
	cmd.SetOut(outBuf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{repoRoot, "gh"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute: want an error for a cyclic PR graph, got nil")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("err = %v, want an error naming a cycle", err)
	}
	if strings.Contains(outBuf.String(), `"number"`) {
		t.Errorf("stdout = %q, want no JSON printed on error", outBuf.String())
	}
}
