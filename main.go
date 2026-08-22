package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"html/template"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/choffmann/simple-fileupload/internal/config"
	appoidc "github.com/choffmann/simple-fileupload/internal/oidc"
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
	oidc          *appoidc.Provider
	logger        *slog.Logger
	baseURL       string
	origin        string
	secureCookies bool

	// test seams: when set they replace the urls computed from the provider
	authorizeURL string
	logoutURL    string
}

func main() {
	logger := config.NewLogger()

	baseURL, err := config.RequireBaseURL()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	origin, err := config.Origin(baseURL)
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

	oidcCfg, err := config.RequireOIDC()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	provider, err := appoidc.New(context.Background(), appoidc.Config{
		Issuer:       oidcCfg.Issuer,
		ClientID:     oidcCfg.ClientID,
		ClientSecret: oidcCfg.ClientSecret,
		RedirectURL:  baseURL + "/auth/callback",
	})
	if err != nil {
		logger.Error("failed to reach the identity provider", "error", err)
		os.Exit(1)
	}

	app := &App{
		store:         storage.New(config.UploadDir()),
		sessions:      session.NewManager(secret, 12*time.Hour, secureCookies),
		oidc:          provider,
		logger:        logger,
		baseURL:       baseURL,
		origin:        origin,
		secureCookies: secureCookies,
	}

	logger.Info("starting server on :8080")
	if err := http.ListenAndServe(":8080", app.newMux()); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func (app *App) newMux() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", app.indexHandler)
	mux.HandleFunc("GET /{username}/{path...}", app.browseHandler)
	mux.HandleFunc("GET /qr/{username}/{path...}", app.qrHandler)
	mux.Handle("POST /upload", app.sameOrigin(app.requireUser(http.HandlerFunc(app.uploadHandler))))
	mux.Handle("POST /mkdir", app.sameOrigin(app.requireUser(http.HandlerFunc(app.mkdirHandler))))
	mux.HandleFunc("GET /auth/login", app.loginHandler)
	mux.HandleFunc("GET /auth/callback", app.callbackHandler)
	mux.Handle("POST /auth/logout", app.sameOrigin(http.HandlerFunc(app.logoutHandler)))

	return mux
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

func (app *App) sameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// SameSite=Lax does not help here: anyone can upload html into their own
		// area, and it is then served from this very origin.
		if r.Header.Get("Origin") != app.origin {
			app.logger.Warn("rejected a cross origin request", "origin", r.Header.Get("Origin"), "path", r.URL.Path)
			http.Error(w, "Cross origin request rejected", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
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

	if err := indexTemplate.ExecuteTemplate(w, "base", struct {
		PageTitle string
		Username  string
		Users     []string
	}{
		PageTitle: "Areas",
		Users:     users,
	}); err != nil {
		app.logger.Error("failed to render the index page", "error", err)
	}
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
	Username    string
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
		// uploaded files come back from this origin, so html or svg would
		// otherwise run as first party script against every visitor's session
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; form-action 'none'")
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
		Username:         authUser,
		IsOwner:          authUser == username,
		CurrentPath:      currentPath,
		CurrentPathSlash: currentPathSlash,
		HasParent:        hasParent,
		ParentHref:       publicurl.Path(username, parentPath),
		Breadcrumbs:      buildBreadcrumbs(username, currentPath),
		Entries:          viewEntries,
	}

	if err := browseTemplate.ExecuteTemplate(w, "base", data); err != nil {
		app.logger.Error("failed to render the browse page", "error", err)
	}
}

func (app *App) uploadHandler(w http.ResponseWriter, r *http.Request) {
	username := usernameFromContext(r.Context())

	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10 MB
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer func() { _ = file.Close() }()

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

	parent := ""
	if trimmed := strings.Trim(dir, "/"); trimmed != "" {
		parent = trimmed + "/"
	}
	http.Redirect(w, r, publicurl.Path(username, parent), http.StatusSeeOther)
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

	if err := qrTemplate.ExecuteTemplate(w, "base", QRData{
		PageTitle:   "QR — " + name,
		Username:    app.sessions.Username(r),
		Heading:     name,
		PublicURL:   publicURL,
		ImageURL:    escaped + "?format=png",
		DownloadURL: escaped + "?format=png&download=1",
		BackURL:     publicurl.Path(username, backPath),
		BackLabel:   backLabel,
	}); err != nil {
		app.logger.Error("failed to render the qr page", "error", err)
	}
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

	_, _ = w.Write(png)
}

const stateCookieName = "oidc_state"

const stateCookieTTL = 10 * time.Minute

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (app *App) writeStateCookie(w http.ResponseWriter, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   app.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

func (app *App) loginHandler(w http.ResponseWriter, r *http.Request) {
	state, err := randomToken()
	if err != nil {
		app.logger.Error("failed to generate the login state", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	nonce, err := randomToken()
	if err != nil {
		app.logger.Error("failed to generate the login nonce", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	app.writeStateCookie(w, state+"."+nonce, int(stateCookieTTL.Seconds()))

	target := app.authorizeURL
	if target == "" {
		target = app.oidc.AuthCodeURL(state, nonce)
	}
	http.Redirect(w, r, target, http.StatusFound)
}

func (app *App) callbackHandler(w http.ResponseWriter, r *http.Request) {
	// the state is single use, so it goes regardless of how this ends
	app.writeStateCookie(w, "", -1)

	cookie, err := r.Cookie(stateCookieName)
	if err != nil {
		app.logger.Warn("callback without a state cookie")
		http.Error(w, "Your login expired, please try again", http.StatusBadRequest)
		return
	}

	state, nonce, ok := strings.Cut(cookie.Value, ".")
	if !ok || state == "" {
		app.logger.Warn("callback with a malformed state cookie")
		http.Error(w, "Your login expired, please try again", http.StatusBadRequest)
		return
	}

	if subtle.ConstantTimeCompare([]byte(state), []byte(r.URL.Query().Get("state"))) != 1 {
		app.logger.Warn("callback with a mismatched state")
		http.Error(w, "Invalid login state", http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		app.logger.Warn("callback without an authorization code")
		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		return
	}

	identity, err := app.oidc.Exchange(r.Context(), code, nonce)
	if err != nil {
		app.logger.Error("login failed", "error", err)
		http.Error(w, "Login failed", http.StatusBadRequest)
		return
	}

	if err := app.store.EnsureUserDir(identity.Username); err != nil {
		app.logger.Error("failed to create the user directory", "user", identity.Username, "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	app.sessions.Issue(w, identity.Username)
	app.logger.Info("user signed in", "user", identity.Username)

	http.Redirect(w, r, publicurl.Path(identity.Username, ""), http.StatusSeeOther)
}

func (app *App) logoutHandler(w http.ResponseWriter, r *http.Request) {
	app.sessions.Clear(w)
	app.writeStateCookie(w, "", -1)

	target := app.logoutURL
	if target == "" {
		target = app.oidc.LogoutURL(app.baseURL)
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}
