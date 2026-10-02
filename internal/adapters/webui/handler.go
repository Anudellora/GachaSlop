// Package webui composes the API with an optional built SPA. It knows nothing
// about accounts, storage, or vendor APIs and can be replaced by a CDN/proxy.
package webui

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

func Handler(api http.Handler, assets fs.FS) http.Handler {
	files := http.FileServer(http.FS(assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") ||
			r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			api.ServeHTTP(w, r)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' https://act-webstatic.hoyoverse.com https://fastcdn.hoyoverse.com https://aki-gm-resources-back.aki-game.com https://enka.network; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" || name == "." {
			name = "index.html"
		}
		if info, err := fs.Stat(assets, name); err == nil && !info.IsDir() {
			w.Header().Set("Cache-Control", "no-cache")
			if strings.HasPrefix(name, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			files.ServeHTTP(w, r)
			return
		}
		// Known client routes only; missing chunks and unknown URLs must stay 404.
		switch r.URL.Path {
		case "/history", "/accounts", "/banners":
			clone := r.Clone(r.Context())
			clone.URL.Path = "/"
			w.Header().Set("Cache-Control", "no-cache")
			files.ServeHTTP(w, clone)
		default:
			http.NotFound(w, r)
		}
	})
}
