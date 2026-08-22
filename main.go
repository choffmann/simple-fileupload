package main

import (
	"context"
	"embed"
	"html/template"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/choffmann/simple-fileupload/internal/config"
	"github.com/choffmann/simple-fileupload/internal/publicurl"
	"github.com/choffmann/simple-fileupload/internal/qr"
	"github.com/choffmann/simple-fileupload/internal/session"
	"github.com/choffmann/simple-fileupload/internal/storage"
)

//go:embed templates/*
var templateFS embed.FS

var indexTemplate = template.Must(template.ParseFS(templateFS, "templates/base.tmpl.html", "templates/index.tmpl.html"))
var browseTemplate = template.Must(template.ParseFS(templateFS, "templates/base.tmpl.html", "templates/browse.tmpl.html"))
var qrTemplate = template.Must(template.ParseFS(templateFS, "templates/base.tmpl.html", "templates/qr.tmpl.html"))

type ctxKey string

const ctxUsername ctxKey = "username"

type App struct {
	store         *storage.Storage
	sessions      *session.Manager
	logger        *slog.Logger
	baseURL       string
	secureCookies bool
}

func main() {
	logger := config.NewLogger()

	baseURL, err := config.RequireBaseURL()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	secret, generated, err := config.SessionSecret()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	if generated {
		logger.Warn("SESSION_SECRET is not set, using a random one; every restart signs all users out")
	}

	secureCookies := strings.HasPrefix(baseURL, "https://")

	app := &App{
		store:         storage.New(config.UploadDir()),
		sessions:      session.NewManager(secret, 12*time.Hour, secureCookies),
		logger:        logger,
		baseURL:       baseURL,
		secureCookies: secureCookies,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", app.indexHandler)
	mux.HandleFunc("GET /{username}/{path...}", app.browseHandler)
	mux.HandleFunc("GET /qr/{username}/{path...}", app.qrHandler)
	mux.Handle("POST /upload", app.requireUser(http.HandlerFunc(app.uploadHandler)))
	mux.Handle("POST /mkdir", app.requireUser(http.HandlerFunc(app.mkdirHandler)))

	logger.Info("starting server on :8080")
	http.ListenAndServe(":8080", mux)
}

func (app *App) requireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username := app.sessions.Username(r)
		if username == "" {
			http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
			return
		}

		ctx := context.WithValue(r.Context(), ctxUsername, username)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func usernameFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxUsername).(string)
	return v
}

func (app *App) indexHandler(w http.ResponseWriter, r *http.Request) {
	if username := app.sessions.Username(r); username != "" {
		http.Redirect(w, r, publicurl.Path(username, ""), http.StatusFound)
		return
	}

	users, err := app.store.ListUsers()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	indexTemplate.ExecuteTemplate(w, "base", struct {
		PageTitle string
		Username  string
		Users     []string
	}{
		PageTitle: "Areas",
		Users:     users,
	})
}

type BrowseData struct {
	PageTitle        string
	Username         string
	IsOwner          bool
	CurrentPath      string
	CurrentPathSlash string
	HasParent        bool
	ParentHref       string
	Breadcrumbs      []Breadcrumb
	Entries          []BrowseEntry
}

type BrowseEntry struct {
	storage.Entry
	Href   string
	QRHref string
}

type Breadcrumb struct {
	Name string
	Path string
}

type QRData struct {
	PageTitle   string
	Heading     string
	PublicURL   string
	ImageURL    string
	DownloadURL string
	BackURL     string
	BackLabel   string
}

func buildBreadcrumbs(username, p string) []Breadcrumb {
	p = strings.Trim(p, "/")

	crumbs := []Breadcrumb{{Name: username, Path: publicurl.Path(username, "")}}

	if p == "" {
		return crumbs
	}

	parts := strings.Split(p, "/")
	for i, part := range parts {
		crumbs = append(crumbs, Breadcrumb{
			Name: part,
			Path: publicurl.Path(username, strings.Join(parts[:i+1], "/")+"/"),
		})
	}
	return crumbs
}

func (app *App) browseHandler(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	subpath := r.PathValue("path")

	// Check if this user/area exists
	if !app.store.UserExists(username) {
		http.NotFound(w, r)
		return
	}

	isDir, err := app.store.IsDir(username, subpath)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if !isDir {
		fullPath, err := app.store.ResolvePath(username, subpath)
		if err != nil {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		http.ServeFile(w, r, fullPath)
		return
	}

	entries, err := app.store.ListDir(username, subpath)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	currentPath := strings.Trim(subpath, "/")
	currentPathSlash := ""
	if currentPath != "" {
		currentPathSlash = currentPath + "/"
	}

	hasParent := currentPath != ""
	parentPath := ""
	if hasParent {
		parent := path.Dir(currentPath)
		if parent != "." {
			parentPath = parent + "/"
		}
	}

	authUser := app.sessions.Username(r)

	viewEntries := make([]BrowseEntry, 0, len(entries))
	for _, e := range entries {
		sub := currentPathSlash + e.Name
		if e.IsDir {
			sub += "/"
		}
		p := publicurl.Path(username, sub)
		viewEntries = append(viewEntries, BrowseEntry{Entry: e, Href: p, QRHref: "/qr" + p})
	}

	data := BrowseData{
		PageTitle:        username + " — Files",
		Username:         username,
		IsOwner:          authUser == username,
		CurrentPath:      currentPath,
		CurrentPathSlash: currentPathSlash,
		HasParent:        hasParent,
		ParentHref:       publicurl.Path(username, parentPath),
		Breadcrumbs:      buildBreadcrumbs(username, currentPath),
		Entries:          viewEntries,
	}

	browseTemplate.ExecuteTemplate(w, "base", data)
}

func (app *App) uploadHandler(w http.ResponseWriter, r *http.Request) {
	username := usernameFromContext(r.Context())

	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10 MB
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	dir := r.FormValue("dir")

	savedName, err := app.store.SaveFile(username, dir, file, header.Filename)
	if err != nil {
		app.logger.Error("failed to save file", "error", err)
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	uploaded := savedName
	if dir := strings.Trim(dir, "/"); dir != "" {
		uploaded = dir + "/" + savedName
	}

	http.Redirect(w, r, "/qr"+publicurl.Path(username, uploaded), http.StatusSeeOther)
}

func (app *App) mkdirHandler(w http.ResponseWriter, r *http.Request) {
	username := usernameFromContext(r.Context())
	dir := r.FormValue("dir")
	name := r.FormValue("name")

	if name == "" {
		http.Error(w, "Folder name required", http.StatusBadRequest)
		return
	}

	subpath := name
	if dir != "" {
		subpath = dir + "/" + name
	}

	if err := app.store.CreateDir(username, subpath); err != nil {
		app.logger.Error("failed to create directory", "error", err)
		http.Error(w, "Failed to create folder", http.StatusInternalServerError)
		return
	}

	redirectPath := "/" + username + "/"
	if dir != "" {
		redirectPath = "/" + username + "/" + dir + "/"
	}
	http.Redirect(w, r, redirectPath, http.StatusSeeOther)
}

func (app *App) qrHandler(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	subpath := strings.Trim(r.PathValue("path"), "/")

	if !app.store.UserExists(username) {
		http.NotFound(w, r)
		return
	}

	isDir, err := app.store.IsDir(username, subpath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	target := subpath
	if isDir && target != "" {
		target += "/"
	}

	publicURL, err := publicurl.For(app.baseURL, username, target)
	if err != nil {
		app.logger.Error("failed to build public url", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if r.URL.Query().Get("format") == "png" {
		app.serveQRImage(w, r, username, subpath, publicURL)
		return
	}

	name := username
	if subpath != "" {
		name = path.Base(subpath)
	}

	backPath := subpath
	if !isDir {
		if d := path.Dir(subpath); d != "." {
			backPath = d
		} else {
			backPath = ""
		}
	}
	if backPath != "" {
		backPath += "/"
	}

	backLabel := username
	if backPath != "" {
		backLabel = path.Base(backPath)
	}

	escaped := r.URL.EscapedPath()

	qrTemplate.ExecuteTemplate(w, "base", QRData{
		PageTitle:   "QR — " + name,
		Heading:     name,
		PublicURL:   publicURL,
		ImageURL:    escaped + "?format=png",
		DownloadURL: escaped + "?format=png&download=1",
		BackURL:     publicurl.Path(username, backPath),
		BackLabel:   backLabel,
	})
}

func (app *App) serveQRImage(w http.ResponseWriter, r *http.Request, username, subpath, publicURL string) {
	png, err := qr.Generate(publicURL, 512)
	if err != nil {
		app.logger.Error("failed to generate qr code", "error", err)
		http.Error(w, "Failed to generate QR code", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")

	if r.URL.Query().Get("download") == "1" {
		name := username
		if subpath != "" {
			name = path.Base(subpath)
		}
		// FormatMediaType handles quoting and non-ascii names; an empty result
		// means the name could not be encoded, so the header is skipped.
		if disposition := mime.FormatMediaType("attachment", map[string]string{"filename": name + ".qr.png"}); disposition != "" {
			w.Header().Set("Content-Disposition", disposition)
		}
	}

	w.Write(png)
}
