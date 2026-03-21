package main

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static
var staticFiles embed.FS

// staticFS returns the embedded static file system.
func staticFS() http.FileSystem {
	sub, _ := fs.Sub(staticFiles, "static")
	return http.FS(sub)
}
