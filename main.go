package main

import (
	"embed"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/template"
)

//go:embed templates/*
var templates embed.FS

func main() {

	mux := http.NewServeMux()
	mux.Handle("GET /", authMiddleware(http.HandlerFunc(indexHandler)))
	mux.Handle("POST /", authMiddleware(http.HandlerFunc(uploadHandler)))
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("./data"))))

	http.ListenAndServe(":8080", mux)
}

var indexTemplate = template.Must(template.ParseFS(templates, "templates/index.tmpl.html", "templates/base.tmpl.html"))

type IndexData struct {
	PageTitle string
}

func indexHandler(w http.ResponseWriter, _ *http.Request) {
	indexTemplate.ExecuteTemplate(w, "base", IndexData{PageTitle: "Upload File"})
}

var uploadTemplate = template.Must(template.ParseFS(templates, "templates/upload.tmpl.html", "templates/base.tmpl.html"))

type UploadData struct {
	PageTitle string
	Filename  string
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	if header.Size > 1024*1024*10 {
		http.Error(w, "File is too big", http.StatusBadRequest)
		return
	}

	osFile, err := os.Create(fmt.Sprintf("data/%s", header.Filename))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = io.Copy(osFile, file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	uploadTemplate.ExecuteTemplate(w, "base", UploadData{PageTitle: "Upload file successfully", Filename: header.Filename})
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted", charset="UTF-8"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if username != os.Getenv("BASIC_AUTH_USERNAME") || password != os.Getenv("BASIC_AUTH_PASSWORD") {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted", charset="UTF-8"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
