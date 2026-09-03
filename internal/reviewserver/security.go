package reviewserver

import (
	"crypto/subtle"
	"net"
	"net/http"
)

// cspHeader and referrerPolicyHeader are the exact, fixed header values
// every response carries — the ticket names precisely these directives and
// nothing else.
const (
	cspHeader            = "object-src 'none'; base-uri 'none'; frame-ancestors 'none'"
	referrerPolicyHeader = "no-referrer"
)

// localHosts is the set of Host values (already port-stripped) a mutating
// request may target.
var localHosts = map[string]bool{
	"localhost": true,
	"127.0.0.1": true,
	"::1":       true,
}

// newHandler builds the full request-handling chain for one review-server
// invocation: security headers (outermost, so even a rejected request
// carries them) wrapping the token check (applies to every route) wrapping
// the Host check (mutating routes only) wrapping the actual routes.
func newHandler(token string) http.Handler {
	mux := http.NewServeMux()
	registerRoutes(mux, token)

	var h http.Handler = mux
	h = hostCheck(h)
	h = tokenCheck(token, h)
	h = securityHeaders(h)
	return h
}

// securityHeaders sets CSP and Referrer-Policy on every response,
// regardless of what happens downstream — set before calling next so a
// 403/401 written by an inner check still carries them.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", cspHeader)
		w.Header().Set("Referrer-Policy", referrerPolicyHeader)
		next.ServeHTTP(w, r)
	})
}

// tokenCheck validates the ?token= query parameter on every request,
// including the initial page load and the future SSE route (query string,
// not a header — EventSource can't set custom headers, and a cookie would
// auto-attach to any-origin requests, reintroducing CSRF). On mismatch it
// responds and returns immediately: no downstream handler — and therefore
// no ticket/ledger read or write — ever runs.
func tokenCheck(token string, next http.Handler) http.Handler {
	want := []byte(token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := []byte(r.URL.Query().Get("token"))
		if subtle.ConstantTimeCompare(got, want) != 1 {
			http.Error(w, "invalid or missing token", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// hostCheck rejects a mutating request (any method other than GET/HEAD)
// whose port-stripped Host header isn't one of localhost/127.0.0.1/::1 —
// defined structurally by method, so routes U8B adds later are covered
// automatically without this file needing to know their paths.
func hostCheck(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			host := r.Host
			if h, _, err := net.SplitHostPort(host); err == nil {
				host = h
			}
			if !localHosts[host] {
				http.Error(w, "forbidden host", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
