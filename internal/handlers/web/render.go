package web

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
)

//go:embed static
var staticFS embed.FS

// assetFS picks where the /admin assets are read from. With staticDir set, files
// are read LIVE from that on-disk dir (edit + browser reload, no Go rebuild — for
// local dev); when it's empty, from the copy embedded in the binary (the
// self-contained default the container image ships). staticDir is resolved
// relative to the process working directory.
func assetFS(staticDir string) fs.FS {
	if staticDir != "" {
		return os.DirFS(staticDir)
	}

	sub, _ := fs.Sub(staticFS, "static")

	return sub
}

// StaticHandler serves the /admin/static/ assets (Vue, CSS, the SPA — no CDN,
// because RU1 DNS is unreliable) from staticDir on disk, or the embedded copy when
// staticDir is empty.
func StaticHandler(staticDir string) http.Handler {
	return http.StripPrefix("/admin/static/", http.FileServer(http.FS(assetFS(staticDir))))
}

// ServePage writes a static HTML page (the SPA shell or the login page) from the
// on-disk dir or the embedded copy. name is relative to the assets root, e.g.
// "index.html".
func ServePage(w http.ResponseWriter, staticDir, name string) {
	b, err := fs.ReadFile(assetFS(staticDir), name)
	if err != nil {
		http.Error(w, "not found", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}
