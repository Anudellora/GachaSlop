package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestRoutingAndCachePolicy(t *testing.T) {
	assets := fstest.MapFS{
		"index.html":        {Data: []byte("<html>GachaSLop</html>")},
		"assets/app-123.js": {Data: []byte("console.log('app')")},
	}
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(418) })
	handler := Handler(api, assets)
	for _, tt := range []struct {
		method, path string
		code         int
		contains     string
	}{
		{"GET", "/", 200, "GachaSLop"}, {"GET", "/history", 200, "GachaSLop"},
		{"GET", "/accounts", 200, "GachaSLop"}, {"GET", "/banners", 200, "GachaSLop"},
		{"GET", "/assets/app-123.js", 200, "console.log"},
		{"GET", "/assets/missing.js", 404, ""}, {"GET", "/assets/", 404, ""},
		{"GET", "/missing", 404, ""}, {"POST", "/history", 405, ""},
		{"GET", "/api/v1/missing", 418, ""}, {"POST", "/api/v1/sync-jobs", 418, ""},
		{"GET", "/readyz", 418, ""}, {"GET", "/healthz", 418, ""},
	} {
		t.Run(tt.method+tt.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, nil))
			if w.Code != tt.code || !strings.Contains(w.Body.String(), tt.contains) {
				t.Fatalf("got %d %s", w.Code, w.Body.String())
			}
			if tt.code == 200 {
				if w.Header().Get("Content-Security-Policy") == "" {
					t.Fatal("missing CSP")
				}
				immutable := strings.Contains(w.Header().Get("Cache-Control"), "immutable")
				if immutable != strings.HasPrefix(tt.path, "/assets/") {
					t.Fatal("incorrect cache policy")
				}
			}
		})
	}
}
