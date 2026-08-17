package webassets

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestHandlerServesSPAAndStaticAssets(t *testing.T) {
	files := fstest.MapFS{
		"index.html":               &fstest.MapFile{Data: []byte("<html>dashboard</html>")},
		"assets/app-abc123.js":     &fstest.MapFile{Data: []byte("console.log('ok')")},
		"manifest.webmanifest":     &fstest.MapFile{Data: []byte(`{"name":"Finance"}`)},
		"sw.js":                    &fstest.MapFile{Data: []byte("self.addEventListener('fetch',()=>{})")},
	}
	handler := NewHandlerFS(files)

	tests := []struct {
		name       string
		path       string
		status     int
		contains   string
		cache      string
	}{
		{name: "root", path: "/", status: http.StatusOK, contains: "dashboard", cache: "no-cache"},
		{name: "spa fallback", path: "/goals/active", status: http.StatusOK, contains: "dashboard", cache: "no-cache"},
		{name: "hashed asset", path: "/assets/app-abc123.js", status: http.StatusOK, contains: "console.log", cache: "public, max-age=31536000, immutable"},
		{name: "service worker", path: "/sw.js", status: http.StatusOK, contains: "fetch", cache: "no-cache"},
		{name: "manifest", path: "/manifest.webmanifest", status: http.StatusOK, contains: "Finance", cache: "no-cache"},
		{name: "missing asset", path: "/assets/missing.js", status: http.StatusNotFound},
		{name: "missing file", path: "/missing.css", status: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, req)
			if resp.Code != tt.status {
				t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
			}
			if tt.contains != "" && !contains(resp.Body.String(), tt.contains) {
				t.Fatalf("body=%q missing %q", resp.Body.String(), tt.contains)
			}
			if tt.cache != "" && resp.Header().Get("Cache-Control") != tt.cache {
				t.Fatalf("Cache-Control=%q want %q", resp.Header().Get("Cache-Control"), tt.cache)
			}
		})
	}
}

func TestEmbeddedFSHasBuildPlaceholder(t *testing.T) {
	if _, err := fs.Stat(embeddedDist, "dist/.gitkeep"); err != nil {
		t.Fatalf("embedded build placeholder: %v", err)
	}
}

func contains(value, part string) bool {
	for i := 0; i+len(part) <= len(value); i++ {
		if value[i:i+len(part)] == part {
			return true
		}
	}
	return false
}
