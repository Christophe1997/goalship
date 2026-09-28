package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCmd_VersionFlag_PrintsBuildVersion(t *testing.T) {
	old := version
	version = "1.2.3"
	t.Cleanup(func() { version = old })

	var out bytes.Buffer
	root := NewRootCmd()
	root.SetOut(&out)
	root.SetArgs([]string{"--version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("--version returned error: %v", err)
	}
	if !strings.Contains(out.String(), "1.2.3") {
		t.Errorf("--version output = %q, want it to contain %q", out.String(), "1.2.3")
	}
}
