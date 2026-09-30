package web

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"sync"
)

//go:embed templates/* templates/components/* static/*
var webFS embed.FS

var (
	tmplCache *template.Template
	tmplOnce  sync.Once
	tmplErr   error
)

// GetTemplate returns the parsed HTML templates including all component sub-templates
func GetTemplate() (*template.Template, error) {
	// Parse templates once or return parsed template
	tmplOnce.Do(func() {
		tmplCache, tmplErr = template.ParseFS(webFS, "templates/index.html", "templates/components/*.html")
	})
	return tmplCache, tmplErr
}

// GetStaticHandler returns an http.Handler that serves embedded static assets (JS, CSS)
func GetStaticHandler() http.Handler {
	sub, err := fs.Sub(webFS, "static")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}
