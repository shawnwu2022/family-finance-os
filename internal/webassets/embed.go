package webassets

import (
	"bytes"
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

// embeddedDist contains a placeholder in source builds. The production Docker
// build replaces the directory contents with Vite's web/dist before compiling.
//
//go:embed all:dist
var embeddedDist embed.FS

func Handler() http.Handler {
	root, err := fs.Sub(embeddedDist, "dist")
	if err != nil {
		panic("open embedded web dist: " + err.Error())
	}
	return NewHandlerFS(root)
}

func NewHandlerFS(root fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		requestPath := path.Clean("/" + strings.TrimPrefix(r.URL.Path, "/"))
		name := strings.TrimPrefix(requestPath, "/")
		if name == "" || name == "." {
			name = "index.html"
		}

		data, info, err := readAsset(root, name)
		if err != nil {
			if path.Ext(name) != "" || strings.HasPrefix(name, "assets/") {
				http.NotFound(w, r)
				return
			}
			name = "index.html"
			data, info, err = readAsset(root, name)
			if err != nil {
				http.NotFound(w, r)
				return
			}
		}

		if strings.HasPrefix(name, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if contentType := contentTypeFor(name); contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		http.ServeContent(w, r, path.Base(name), info.ModTime(), bytes.NewReader(data))
	})
}

func readAsset(root fs.FS, name string) ([]byte, fs.FileInfo, error) {
	file, err := root.Open(name)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}
	if info.IsDir() {
		return nil, nil, fs.ErrNotExist
	}
	data, err := fs.ReadFile(root, name)
	if err != nil {
		return nil, nil, err
	}
	return data, info, nil
}

func contentTypeFor(name string) string {
	switch path.Ext(name) {
	case ".webmanifest":
		return "application/manifest+json; charset=utf-8"
	case ".js":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".html":
		return "text/html; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	default:
		return mime.TypeByExtension(path.Ext(name))
	}
}
