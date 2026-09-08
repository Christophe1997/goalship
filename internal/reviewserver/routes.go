package reviewserver

import (
	"io/fs"
	"net/http"
)

// registerRoutes wires the review server's routes onto mux. Real
// ticket-graph data routes (CRUD, reject/withdraw/approve) are ticket
// U8B's job — the one placeholder below exists only to make the
// mutating-Host-check middleware independently testable here.
func registerRoutes(mux *http.ServeMux, token string) {
	mux.HandleFunc("GET /{$}", indexHandler(token))
	mux.Handle("GET /app.js", staticAsset("assets/app.js", "text/javascript; charset=utf-8"))
	mux.Handle("GET /app.css", staticAsset("assets/app.css", "text/css; charset=utf-8"))

	mux.HandleFunc("POST /api/_stub", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not implemented", http.StatusNotImplemented)
	})
}

// indexHandler templates index.html per request, interpolating the live
// token into app.js's and app.css's asset-URL query strings (KTD6) — the
// raw embedded bytes are never served directly for "/".
func indexHandler(token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		data := struct{ Token string }{Token: token}
		if err := indexTemplate.Execute(w, data); err != nil {
			http.Error(w, "template error", http.StatusInternalServerError)
		}
	}
}

// staticAsset serves one embedded asset file verbatim (app.js/app.css need
// no per-request templating, unlike index.html). It still passes through
// the same token-check middleware as every other route.
func staticAsset(path, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(assetsFS, path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Write(data)
	}
}
