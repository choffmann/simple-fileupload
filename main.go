package main

import (
	"context"
	"embed"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/choffmann/simple-fileupload/internal/auth"
	"github.com/choffmann/simple-fileupload/internal/config"
	"github.com/choffmann/simple-fileupload/internal/qr"
	"github.com/choffmann/simple-fileupload/internal/storage"
)

//go:embed templates/*
var templateFS embed.FS

var indexTemplate = template.Must(template.ParseFS(templateFS, "templates/base.tmpl.html", "templates/index.tmpl.html"))
var browseTemplate = template.Must(template.ParseFS(templateFS, "templates/base.tmpl.html", "templates/browse.tmpl.html"))

type ctxKey string

const ctxUsername ctxKey = "username"

type App struct {
	store  *storage.Storage
	users  []auth.User
	logger *slog.Logger
}

func main() {
	logger := config.NewLogger()

	users, err := loadUsers(logger)
	if err != nil {
		logger.Error("failed to load users", "error", err)
		os.Exit(1)
	}

	store := storage.New(config.UploadDir())

	for _, u := range users {
		if err := store.EnsureUserDir(u.Username); err != nil {
			logger.Error("failed to create user directory", "user", u.Username, "error", err)
			os.Exit(1)
		}
	}

	app := &App{store: store, users: users, logger: logger}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", app.indexHandler)
	mux.HandleFunc("GET /{username}/{path...}", app.browseHandler)
	mux.HandleFunc("GET /qr/{username}/{path...}", app.qrHandler)
	mux.Handle("POST /upload", app.authMiddleware(http.HandlerFunc(app.uploadHandler)))
	mux.Handle("POST /mkdir", app.authMiddleware(http.HandlerFunc(app.mkdirHandler)))

	logger.Info("starting server on :8080")
	http.ListenAndServe(":8080", mux)
}

func loadUsers(logger *slog.Logger) ([]auth.User, error) {
	if f := config.UsersFile(); f != "" {
		logger.Info("loading users from file", "path", f)
		return auth.LoadUsers(f)
	}

	username := os.Getenv("BASIC_AUTH_USERNAME")
	password := os.Getenv("BASIC_AUTH_PASSWORD")
	if username != "" && password != "" {
		logger.Info("using single user from environment")
		return auth.SingleUser(username, password), nil
	}

	logger.Warn("no authentication configured, set USERS_FILE or BASIC_AUTH_USERNAME/BASIC_AUTH_PASSWORD")
	return nil, nil
}

func (app *App) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(app.users) == 0 {
			next.ServeHTTP(w, r)
			return
		}

		username, password, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted", charset="UTF-8"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		user, valid := auth.Authenticate(app.users, username, password)
		if !valid {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted", charset="UTF-8"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), ctxUsername, user.Username)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func usernameFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxUsername).(string)
	return v
}

// tryAuth checks Basic Auth credentials without rejecting unauthenticated requests.
func (app *App) tryAuth(r *http.Request) string {
	username, password, ok := r.BasicAuth()
	if !ok {
		return ""
	}
	if user, valid := auth.Authenticate(app.users, username, password); valid {
		return user.Username
	}
	return ""
}

func (app *App) indexHandler(w http.ResponseWriter, r *http.Request) {
	if username := app.tryAuth(r); username != "" {
		http.Redirect(w, r, "/"+username+"/", http.StatusFound)
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
	ParentPath       string
	Breadcrumbs      []Breadcrumb
	Entries          []storage.Entry
}

type Breadcrumb struct {
	Name string
	Path string
}

func buildBreadcrumbs(username, p string) []Breadcrumb {
	p = strings.Trim(p, "/")

	crumbs := []Breadcrumb{{Name: username, Path: "/" + username + "/"}}

	if p == "" {
		return crumbs
	}

	parts := strings.Split(p, "/")
	for i, part := range parts {
		crumbs = append(crumbs, Breadcrumb{
			Name: part,
			Path: "/" + username + "/" + strings.Join(parts[:i+1], "/") + "/",
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

	authUser := app.tryAuth(r)

	data := BrowseData{
		PageTitle:        username + " — Files",
		Username:         username,
		IsOwner:          authUser == username,
		CurrentPath:      currentPath,
		CurrentPathSlash: currentPathSlash,
		HasParent:        hasParent,
		ParentPath:       parentPath,
		Breadcrumbs:      buildBreadcrumbs(username, currentPath),
		Entries:          entries,
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

	if _, err := app.store.SaveFile(username, dir, file, header.Filename); err != nil {
		app.logger.Error("failed to save file", "error", err)
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	redirectPath := "/" + username + "/"
	if dir != "" {
		redirectPath = "/" + username + "/" + dir + "/"
	}
	http.Redirect(w, r, redirectPath, http.StatusSeeOther)
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

	_, err := app.store.IsDir(username, subpath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	baseURL := config.BaseURL()
	publicURL := baseURL + "/" + username + "/"
	if subpath != "" {
		publicURL += subpath
	}

	png, err := qr.Generate(publicURL, 256)
	if err != nil {
		http.Error(w, "Failed to generate QR code", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Write(png)
}
