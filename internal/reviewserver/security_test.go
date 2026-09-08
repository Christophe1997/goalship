package reviewserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testToken = "test-token-0123456789abcdef0123456789ab"

func TestTokenCheck_MissingOrWrongToken_RejectsBeforeHandler(t *testing.T) {
	handlerRan := false
	inner := okHandler(&handlerRan)
	h := tokenCheck(testToken, inner)

	cases := []struct {
		name  string
		query string
	}{
		{"missing", ""},
		{"wrong", "?token=nope"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handlerRan = false
			req := httptest.NewRequest("GET", "/"+tc.query, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if handlerRan {
				t.Error("inner handler ran despite a bad/missing token")
			}
			if rec.Code != 401 {
				t.Errorf("status = %d, want 401", rec.Code)
			}
		})
	}
}

func TestTokenCheck_CorrectToken_RunsHandler(t *testing.T) {
	handlerRan := false
	inner := okHandler(&handlerRan)
	h := tokenCheck(testToken, inner)

	req := httptest.NewRequest("GET", "/?token="+testToken, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !handlerRan {
		t.Error("inner handler did not run despite a correct token")
	}
	if rec.Code != 200 {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestHostCheck_MutatingRequest_WrongHost_Rejected(t *testing.T) {
	handlerRan := false
	inner := okHandler(&handlerRan)
	h := hostCheck(inner)

	cases := []struct {
		name string
		host string
	}{
		{"remote hostname", "evil.example.com"},
		{"lookalike hostname", "localhost.evil.example.com"},
		{"remote ip with port", "10.0.0.1:9999"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handlerRan = false
			req := httptest.NewRequest("POST", "/api/withdraw", nil)
			req.Host = tc.host
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if handlerRan {
				t.Errorf("inner handler ran despite Host %q on a mutating request", tc.host)
			}
			if rec.Code != 403 {
				t.Errorf("status = %d, want 403", rec.Code)
			}
		})
	}
}

func TestHostCheck_MutatingRequest_LocalHost_Allowed(t *testing.T) {
	// Host headers only bracket an IPv6 literal when a port follows it
	// ("[::1]:8080"); a bare IPv6 host with no port is unbracketed ("::1").
	cases := []string{"localhost", "127.0.0.1", "::1", "localhost:8080", "127.0.0.1:8080", "[::1]:8080"}
	for _, host := range cases {
		t.Run(host, func(t *testing.T) {
			handlerRan := false
			inner := okHandler(&handlerRan)
			h := hostCheck(inner)

			req := httptest.NewRequest("POST", "/api/withdraw", nil)
			req.Host = host
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if !handlerRan {
				t.Errorf("inner handler did not run for allowed Host %q", host)
			}
		})
	}
}

func TestHostCheck_GetRequest_WrongHost_StillAllowed(t *testing.T) {
	// GET/HEAD are not "mutating" — the Host check only applies to methods
	// other than GET/HEAD.
	handlerRan := false
	inner := okHandler(&handlerRan)
	h := hostCheck(inner)

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "evil.example.com"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !handlerRan {
		t.Error("GET request was rejected by the Host check, but only mutating requests should be checked")
	}
}

func TestSecurityHeaders_PresentEvenOnRejectedRequest(t *testing.T) {
	h := newHandler(testToken)

	req := httptest.NewRequest("GET", "/?token=wrong", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != 401 {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	assertSecurityHeaders(t, rec)
}

func TestSecurityHeaders_PresentOnSuccessfulRequest(t *testing.T) {
	h := newHandler(testToken)

	req := httptest.NewRequest("GET", "/?token="+testToken, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	assertSecurityHeaders(t, rec)
}

func assertSecurityHeaders(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	wantCSP := "object-src 'none'; base-uri 'none'; frame-ancestors 'none'"
	if got := rec.Header().Get("Content-Security-Policy"); got != wantCSP {
		t.Errorf("Content-Security-Policy = %q, want %q", got, wantCSP)
	}
	if got := rec.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Errorf("Referrer-Policy = %q, want %q", got, "no-referrer")
	}
}

// testAPIState builds an apiState safe for a security_test.go request that
// is expected to actually reach a real handler: a fresh t.TempDir() for
// both repoRoot and ticketsDir (never the process cwd — see apiState's own
// doc), and a non-nil no-op cancel (handleApprove calls it unconditionally;
// only tests that exercise approve itself need a real one).
func testAPIState(t *testing.T) *apiState {
	t.Helper()
	dir := t.TempDir()
	return &apiState{
		repoRoot:   dir,
		runID:      "run-a",
		ticketsDir: dir,
		cancel:     func() {},
	}
}

// TestAPIRoute_ValidTokenAndHost_Reached discriminates the Host check from
// the token check: a valid token plus an allowed Host must reach a real
// mutating route (POST /api/withdraw, standing in for "any of goa-4ufc's
// routes" per hostCheck's method-based, path-agnostic enforcement) and
// succeed, proving hostCheck's earlier "wrong Host" rejections above
// weren't actually caused by a token failure.
func TestAPIRoute_ValidTokenAndHost_Reached(t *testing.T) {
	h := newReviewHandler(testToken, testAPIState(t))

	req := httptest.NewRequest("POST", "/api/withdraw?token="+testToken, nil)
	req.Host = "127.0.0.1"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	assertSecurityHeaders(t, rec)
}

func TestAPIRoute_ValidTokenWrongHost_Rejected(t *testing.T) {
	h := newReviewHandler(testToken, testAPIState(t))

	req := httptest.NewRequest("POST", "/api/withdraw?token="+testToken, nil)
	req.Host = "evil.example.com"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != 403 {
		t.Errorf("status = %d, want 403", rec.Code)
	}
	assertSecurityHeaders(t, rec)
}

// TestAssetRoutes_WrongToken_Rejected proves AC4's "every route" against
// the two static-asset routes specifically, not just structurally via
// tokenCheck wrapping the whole mux.
func TestAssetRoutes_WrongToken_Rejected(t *testing.T) {
	h := newHandler(testToken)

	for _, path := range []string{"/app.js", "/app.css"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path+"?token=wrong", nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != 401 {
				t.Errorf("status = %d, want 401", rec.Code)
			}
			assertSecurityHeaders(t, rec)
		})
	}
}

// TestIndex_TemplatesTokenIntoAssetURLs proves KTD6, the ticket's central
// mechanism: "/" is templated per request with the live token interpolated
// into app.js's and app.css's asset-URL query strings — the raw embedded
// bytes (with a literal "{{.Token}}" placeholder) must never be served.
func TestIndex_TemplatesTokenIntoAssetURLs(t *testing.T) {
	h := newHandler(testToken)
	req := httptest.NewRequest("GET", "/?token="+testToken, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"/app.js?token=" + testToken,
		"/app.css?token=" + testToken,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body does not contain %q; got:\n%s", want, body)
		}
	}
	if strings.Contains(body, "{{") {
		t.Error("body still contains an unexecuted template action")
	}
}

func okHandler(ran *bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		*ran = true
		w.WriteHeader(200)
	}
}
