package loop

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// printJSON writes v to cmd's stdout as indented JSON, HTML-escaping
// disabled to agree with internal/ledger's own encodeIndented: Python's
// json.dumps never escapes '<'/'>'/'&', so neither does this, keeping a
// goal string containing those characters identical whether it's printed
// here (e.g. resume-candidates) or persisted to a ledger file. This
// matches loop_runner.py's json.dumps contract's shape (keys and values),
// not its exact separator/whitespace choices, which nothing downstream
// depends on byte-for-byte.
func printJSON(cmd *cobra.Command, v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("loop: marshal JSON output: %w", err)
	}
	fmt.Fprint(cmd.OutOrStdout(), buf.String())
	return nil
}

// nonNilStrings substitutes a non-nil empty slice for nil: encoding/json
// marshals a nil []string as `null`, but loop_runner.py's own json.dumps([])
// output (and this ticket's fixtures) expect `[]`.
func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// strPtrOrNil returns nil for "" and &s otherwise, so an unset value prints
// as JSON null (mirroring Python's Optional[str] = None) rather than an
// empty string indistinguishable from a real one.
func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
