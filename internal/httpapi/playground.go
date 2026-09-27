package httpapi

import (
	"embed"
	"net/http"
)

//go:embed playground/*
var playgroundAssets embed.FS

var playgroundFiles = map[string]struct {
	path        string
	contentType string
}{
	"/playground/":           {path: "playground/index.html", contentType: "text/html; charset=utf-8"},
	"/playground/app.js":     {path: "playground/app.js", contentType: "text/javascript; charset=utf-8"},
	"/playground/mark.svg":   {path: "playground/mark.svg", contentType: "image/svg+xml"},
	"/playground/styles.css": {path: "playground/styles.css", contentType: "text/css; charset=utf-8"},
}

func redirectPlayground(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/playground/", http.StatusPermanentRedirect)
}

func servePlayground(w http.ResponseWriter, r *http.Request) {
	asset, ok := playgroundFiles[r.URL.Path]
	if !ok {
		http.NotFound(w, r)
		return
	}
	contents, err := playgroundAssets.ReadFile(asset.path)
	if err != nil {
		http.Error(w, "playground asset unavailable", http.StatusInternalServerError)
		return
	}
	setPlaygroundSecurityHeaders(w.Header())
	w.Header().Set("Content-Type", asset.contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(contents)
}

func servePlaygroundConfig(credentialTesting bool) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		setPlaygroundSecurityHeaders(w.Header())
		writeJSON(w, http.StatusOK, map[string]bool{"credential_testing": credentialTesting})
	}
}

func setPlaygroundSecurityHeaders(header http.Header) {
	header.Set("Cache-Control", "no-store")
	header.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; media-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("X-Frame-Options", "DENY")
}
