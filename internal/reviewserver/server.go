// Package reviewserver implements `goalship review`'s local HTTP checkpoint:
// a loopback-only server, gated by a per-invocation token and Host
// validation (security.go), serving the embedded review UI. Route handlers
// for actual ticket-graph data are ticket U8B; this package ships the
// server skeleton, security checks, asset serving, and process lifecycle.
package reviewserver

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/Christophe1997/goalship/internal/ledger"
)

//go:embed assets
var assetsFS embed.FS

var indexTemplate = template.Must(template.ParseFS(assetsFS, "assets/index.html"))

// tokenBytes is crypto/rand.Read's input length: 32 bytes (256 bits) is
// comfortably beyond brute-force range for a token that lives only as long
// as one review invocation.
const tokenBytes = 32

// shutdownTimeout bounds how long Run waits for in-flight requests to
// finish once ctx is done, so a stuck handler can't hang process exit
// indefinitely.
const shutdownTimeout = 5 * time.Second

// Options configures a single `goalship review` invocation.
type Options struct {
	RepoRoot string
	RunID    string

	// Stdout receives the tokened URL announcement. Required — Run writes
	// to it directly rather than os.Stdout, so callers (and tests) can
	// capture or redirect it.
	Stdout io.Writer
	// Stderr receives non-fatal diagnostics, e.g. a failed browser-open
	// call. Defaults to Stdout when nil.
	Stderr io.Writer

	// OpenBrowser launches the OS's default browser at url. Defaults to a
	// real cross-platform implementation (openBrowser) when nil; tests
	// inject a no-op so they never spawn a real browser process.
	OpenBrowser func(url string) error
}

// Run acquires runID's review lock, binds an ephemeral loopback port, mints
// a per-invocation token, prints the tokened URL, attempts to open it in
// the browser, and serves until ctx is done. A clean return (ctx canceled,
// or a graceful shutdown) releases the lock and leaves review_state
// untouched — this package never writes ledger state.
func Run(ctx context.Context, opts Options) error {
	lock, err := ledger.AcquireReviewLock(opts.RepoRoot, opts.RunID)
	if err != nil {
		return fmt.Errorf("reviewserver: %w", err)
	}
	defer lock.Release()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("reviewserver: listen: %w", err)
	}
	defer listener.Close()

	token, err := generateToken()
	if err != nil {
		return fmt.Errorf("reviewserver: %w", err)
	}

	stderr := opts.Stderr
	if stderr == nil {
		stderr = opts.Stdout
	}
	openBrowser := opts.OpenBrowser
	if openBrowser == nil {
		openBrowser = defaultOpenBrowser
	}

	handler := newHandler(token)
	srv := &http.Server{Handler: handler}

	url := fmt.Sprintf("http://%s/?token=%s", listener.Addr(), token)
	fmt.Fprintf(opts.Stdout, "Review server ready: %s\n", url)

	if err := openBrowser(url); err != nil {
		fmt.Fprintf(stderr, "review: could not open browser automatically: %v\n", err)
	}

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.Serve(listener)
	}()

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("reviewserver: serve: %w", err)
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("reviewserver: shutdown: %w", err)
		}
		return nil
	}
}

// generateToken mints a per-invocation token from a CSPRNG (never
// math/rand), encoded for direct use as a URL query value (unpadded
// base64url — no characters that need percent-encoding).
func generateToken() (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// defaultOpenBrowser launches the OS's default-browser handler. A failure
// here (headless host, no DISPLAY, missing binary, SSH without forwarding)
// is reported by the caller as a non-fatal diagnostic — the tokened URL
// already printed to stdout is the operator's fallback.
func defaultOpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		// "start" is a cmd.exe builtin, not a standalone executable.
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
