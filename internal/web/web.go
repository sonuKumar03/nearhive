package web

import (
	"embed"
	"net/http"
)

//go:embed index.html
var Content embed.FS

// Handler serves the embedded interactive dashboard single-page application.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := Content.ReadFile("index.html")
		if err != nil {
			http.Error(w, "Internal Server Error: failed to load dashboard", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
}
