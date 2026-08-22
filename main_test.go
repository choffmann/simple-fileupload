package main

import (
	"bytes"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/choffmann/simple-fileupload/internal/session"
	"github.com/choffmann/simple-fileupload/internal/storage"
)

func newTestApp(t *testing.T) *App {
	t.Helper()

	dir := t.TempDir()
	store := storage.New(dir)
	if err := store.EnsureUserDir("alice"); err != nil {
		t.Fatalf("EnsureUserDir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "alice", "alte fotos"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "alice", "alte fotos", "Übung 1.pdf"), []byte("pdf"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	return &App{
		store:    store,
		sessions: session.NewManager([]byte("a-secret-of-decent-length"), time.Hour, false),
		logger:   slog.New(slog.DiscardHandler),
		baseURL:  "https://files.example.com",
		origin:   "https://files.example.com",
	}
}

func signIn(t *testing.T, app *App, req *http.Request, username string) *http.Request {
	t.Helper()

	rec := httptest.NewRecorder()
	app.sessions.Issue(rec, username)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}
	return req
}

func TestQRHandlerRendersPageWithPublicURL(t *testing.T) {
	app := newTestApp(t)

	rec := httptest.NewRecorder()
	app.newMux().ServeHTTP(rec, httptest.NewRequest("GET", "/qr/alice/alte%20fotos/%C3%9Cbung%201.pdf", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body)
	}
	body := rec.Body.String()

	wantURL := "https://files.example.com/alice/alte%20fotos/%C3%9Cbung%201.pdf"
	if !strings.Contains(body, wantURL) {
		t.Errorf("page does not contain the public url %q\nbody: %s", wantURL, body)
	}
	if !strings.Contains(body, "?format=png") {
		t.Errorf("page does not embed the png image\nbody: %s", body)
	}
}

func TestQRHandlerServesPNG(t *testing.T) {
	app := newTestApp(t)

	rec := httptest.NewRecorder()
	app.newMux().ServeHTTP(rec, httptest.NewRequest("GET", "/qr/alice/alte%20fotos/%C3%9Cbung%201.pdf?format=png", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("got Content-Type %q, want %q", got, "image/png")
	}
	if !bytes.HasPrefix(rec.Body.Bytes(), []byte("\x89PNG\r\n\x1a\n")) {
		t.Error("body does not start with the PNG signature")
	}
	if got := rec.Header().Get("Content-Disposition"); got != "" {
		t.Errorf("got Content-Disposition %q, want it absent without download=1", got)
	}
}

func TestQRHandlerPNGDownloadSetsDisposition(t *testing.T) {
	app := newTestApp(t)

	rec := httptest.NewRecorder()
	app.newMux().ServeHTTP(rec, httptest.NewRequest("GET", "/qr/alice/alte%20fotos/%C3%9Cbung%201.pdf?format=png&download=1", nil))

	got := rec.Header().Get("Content-Disposition")
	if !strings.HasPrefix(got, "attachment;") {
		t.Errorf("got Content-Disposition %q, want it to start with %q", got, "attachment;")
	}
}

func TestQRHandlerUnknownPathIsNotFound(t *testing.T) {
	app := newTestApp(t)

	rec := httptest.NewRecorder()
	app.newMux().ServeHTTP(rec, httptest.NewRequest("GET", "/qr/alice/gibtsnicht.pdf", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestQRHandlerDirectoryURLKeepsTrailingSlash(t *testing.T) {
	app := newTestApp(t)

	rec := httptest.NewRecorder()
	app.newMux().ServeHTTP(rec, httptest.NewRequest("GET", "/qr/alice/alte%20fotos/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body)
	}
	wantURL := "https://files.example.com/alice/alte%20fotos/"
	if !strings.Contains(rec.Body.String(), wantURL) {
		t.Errorf("page does not contain %q\nbody: %s", wantURL, rec.Body)
	}
}

type uploadFile struct {
	name    string
	content string
}

func postUpload(t *testing.T, app *App, dir string, files ...uploadFile) *httptest.ResponseRecorder {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("dir", dir); err != nil {
		t.Fatalf("WriteField: %v", err)
	}
	for _, f := range files {
		part, err := writer.CreateFormFile("file", f.name)
		if err != nil {
			t.Fatalf("CreateFormFile: %v", err)
		}
		if _, err := part.Write([]byte(f.content)); err != nil {
			t.Fatalf("writing part: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("closing writer: %v", err)
	}

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	signIn(t, app, req, "alice")

	rec := httptest.NewRecorder()
	app.requireUser(http.HandlerFunc(app.uploadHandler)).ServeHTTP(rec, req)
	return rec
}

func TestUploadRedirectsToTheTargetFolder(t *testing.T) {
	tests := []struct {
		name string
		dir  string
		want string
	}{
		{"root of the area", "", "/alice/"},
		{"subdirectory with spaces", "alte fotos", "/alice/alte%20fotos/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newTestApp(t)

			rec := postUpload(t, app, tt.dir, uploadFile{"foo.pdf", "payload"})

			if rec.Code != http.StatusSeeOther {
				t.Fatalf("got status %d, want %d (body: %s)", rec.Code, http.StatusSeeOther, rec.Body)
			}
			if got := rec.Header().Get("Location"); got != tt.want {
				t.Errorf("got Location %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUploadSavesEverySelectedFile(t *testing.T) {
	app := newTestApp(t)

	rec := postUpload(t, app, "alte fotos",
		uploadFile{"a.txt", "first"},
		uploadFile{"../evil.txt", "second"},
		uploadFile{"Übung 2.pdf", "third"},
	)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("got status %d, want %d (body: %s)", rec.Code, http.StatusSeeOther, rec.Body)
	}

	want := map[string]string{"a.txt": "first", "evil.txt": "second", "Übung 2.pdf": "third"}
	for name, content := range want {
		got, err := os.ReadFile(filepath.Join(app.store.BaseDir, "alice", "alte fotos", name))
		if err != nil {
			t.Fatalf("ReadFile %s: %v", name, err)
		}
		if string(got) != content {
			t.Errorf("%s contains %q, want %q", name, got, content)
		}
	}
}

func TestUploadWithoutAFileIsRejected(t *testing.T) {
	app := newTestApp(t)

	rec := postUpload(t, app, "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d (body: %s)", rec.Code, http.StatusBadRequest, rec.Body)
	}
}

func TestBrowseHandlerEscapesLinks(t *testing.T) {
	app := newTestApp(t)
	if err := os.WriteFile(filepath.Join(app.store.BaseDir, "alice", "alte fotos", "a#b.png"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	mux := app.newMux()

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/alice/alte%20fotos/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body)
	}
	body := rec.Body.String()

	for _, want := range []string{
		`href="/alice/alte%20fotos/a%23b.png"`,
		`href="/qr/alice/alte%20fotos/a%23b.png"`,
		`href="/alice/alte%20fotos/%C3%9Cbung%201.pdf"`,
		`href="/qr/alice/alte%20fotos/%C3%9Cbung%201.pdf"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body is missing %s", want)
		}
	}
}

func TestUploadWithoutSessionRedirectsToLogin(t *testing.T) {
	app := newTestApp(t)

	req := httptest.NewRequest("POST", "/upload", strings.NewReader(""))
	rec := httptest.NewRecorder()
	app.requireUser(http.HandlerFunc(app.uploadHandler)).ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != "/auth/login" {
		t.Errorf("got Location %q, want %q", got, "/auth/login")
	}
}

func TestBrowseShowsUploadFormOnlyToTheOwner(t *testing.T) {
	app := newTestApp(t)

	mux := app.newMux()

	t.Run("owner sees the form", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/alice/", nil)
		signIn(t, app, req, "alice")

		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if !strings.Contains(rec.Body.String(), `action="/upload"`) {
			t.Error("the owner does not see the upload form")
		}
		if !strings.Contains(rec.Body.String(), `type="file" name="file" multiple`) {
			t.Error("the upload form does not allow picking several files")
		}
	})

	t.Run("a stranger does not", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/alice/", nil)
		signIn(t, app, req, "bob")

		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if strings.Contains(rec.Body.String(), `action="/upload"`) {
			t.Error("a stranger sees the upload form")
		}
	})
}

func TestLoginSetsStateCookieAndRedirectsToTheProvider(t *testing.T) {
	app := newTestApp(t)
	app.authorizeURL = "https://kc.example.com/auth"

	rec := httptest.NewRecorder()
	app.loginHandler(rec, httptest.NewRequest("GET", "/auth/login", nil))

	if rec.Code != http.StatusFound {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusFound)
	}

	var state *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == stateCookieName {
			state = c
		}
	}
	if state == nil {
		t.Fatal("the login handler did not set a state cookie")
	}
	if !state.HttpOnly {
		t.Error("the state cookie is not HttpOnly")
	}
	if _, _, ok := strings.Cut(state.Value, "."); !ok {
		t.Errorf("state cookie %q does not carry both state and nonce", state.Value)
	}
}

func TestCallbackWithoutStateCookieIsRejected(t *testing.T) {
	app := newTestApp(t)

	rec := httptest.NewRecorder()
	app.callbackHandler(rec, httptest.NewRequest("GET", "/auth/callback?code=abc&state=xyz", nil))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCallbackWithMismatchedStateIsRejected(t *testing.T) {
	app := newTestApp(t)

	req := httptest.NewRequest("GET", "/auth/callback?code=abc&state=wrong", nil)
	req.AddCookie(&http.Cookie{Name: stateCookieName, Value: "expected.somenonce"})

	rec := httptest.NewRecorder()
	app.callbackHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCallbackWithoutCodeIsRejected(t *testing.T) {
	app := newTestApp(t)

	req := httptest.NewRequest("GET", "/auth/callback?state=expected", nil)
	req.AddCookie(&http.Cookie{Name: stateCookieName, Value: "expected.somenonce"})

	rec := httptest.NewRecorder()
	app.callbackHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCallbackClearsTheStateCookieOnFailure(t *testing.T) {
	app := newTestApp(t)

	rec := httptest.NewRecorder()
	app.callbackHandler(rec, httptest.NewRequest("GET", "/auth/callback", nil))

	for _, c := range rec.Result().Cookies() {
		if c.Name == stateCookieName && c.MaxAge < 0 {
			return
		}
	}
	t.Error("the failing callback did not expire the state cookie")
}

func TestLogoutClearsTheSessionAndRedirects(t *testing.T) {
	app := newTestApp(t)
	app.logoutURL = "https://kc.example.com/logout"

	req := httptest.NewRequest("POST", "/auth/logout", nil)
	signIn(t, app, req, "alice")

	rec := httptest.NewRecorder()
	app.logoutHandler(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != "https://kc.example.com/logout" {
		t.Errorf("got Location %q, want the provider logout url", got)
	}

	for _, c := range rec.Result().Cookies() {
		if c.Name == session.CookieName && c.MaxAge < 0 {
			return
		}
	}
	t.Error("logout did not expire the session cookie")
}

func TestHeaderShowsSignInWithoutSession(t *testing.T) {
	app := newTestApp(t)

	mux := app.newMux()

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/alice/", nil))

	body := rec.Body.String()
	if !strings.Contains(body, `href="/auth/login"`) {
		t.Error("the page does not offer a sign in link")
	}
	if strings.Contains(body, `action="/auth/logout"`) {
		t.Error("the page offers a sign out form without a session")
	}
}

func TestHeaderShowsSignOutWithSession(t *testing.T) {
	app := newTestApp(t)

	mux := app.newMux()

	req := httptest.NewRequest("GET", "/alice/", nil)
	signIn(t, app, req, "alice")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `action="/auth/logout"`) {
		t.Error("the page does not offer a sign out form")
	}
	if strings.Contains(body, `href="/auth/login"`) {
		t.Error("the page still offers a sign in link with a session")
	}
}

func TestQRPageShowsTheSignedInUser(t *testing.T) {
	app := newTestApp(t)

	req := httptest.NewRequest("GET", "/qr/alice/", nil)
	signIn(t, app, req, "alice")

	rec := httptest.NewRecorder()
	app.newMux().ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), `action="/auth/logout"`) {
		t.Error("the qr page does not carry the session header")
	}
}

func TestBrowseRejectsEncodedTraversalInTheUsername(t *testing.T) {
	app := newTestApp(t)

	mux := app.newMux()

	for _, target := range []string{"/..%2f..%2fetc/", "/..%2f/", "/alice%2f..%2f..%2fetc/"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", target, nil))

		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: got status %d, want %d (body: %s)", target, rec.Code, http.StatusNotFound, rec.Body)
		}
	}
}

func TestMuxKeepsUploadAndMkdirBehindASession(t *testing.T) {
	app := newTestApp(t)
	mux := app.newMux()

	for _, target := range []string{"/upload", "/mkdir"} {
		req := httptest.NewRequest("POST", target, strings.NewReader(""))
		req.Header.Set("Origin", app.origin)

		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusSeeOther {
			t.Errorf("%s: got status %d, want %d", target, rec.Code, http.StatusSeeOther)
		}
		if got := rec.Header().Get("Location"); got != "/auth/login" {
			t.Errorf("%s: got Location %q, want %q", target, got, "/auth/login")
		}
	}
}

func TestMuxRoutesEveryEndpoint(t *testing.T) {
	app := newTestApp(t)
	app.authorizeURL = "https://kc.example.com/auth"
	mux := app.newMux()

	tests := []struct {
		method string
		target string
		want   int
	}{
		{"GET", "/", http.StatusOK},
		{"GET", "/alice/", http.StatusOK},
		{"GET", "/qr/alice/", http.StatusOK},
		{"GET", "/auth/login", http.StatusFound},
		{"GET", "/auth/callback", http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.target, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.target, nil))

			if rec.Code != tt.want {
				t.Errorf("got status %d, want %d (body: %s)", rec.Code, tt.want, rec.Body)
			}
		})
	}
}

func TestStateChangingRoutesRejectForeignOrigins(t *testing.T) {
	app := newTestApp(t)
	mux := app.newMux()

	origins := []struct {
		name   string
		origin string
	}{
		{"foreign origin", "https://evil.example.com"},
		{"missing origin", ""},
		{"scheme downgraded", "http://files.example.com"},
	}
	for _, target := range []string{"/upload", "/mkdir", "/auth/logout"} {
		for _, o := range origins {
			t.Run(target+" "+o.name, func(t *testing.T) {
				req := httptest.NewRequest("POST", target, strings.NewReader(""))
				if o.origin != "" {
					req.Header.Set("Origin", o.origin)
				}
				signIn(t, app, req, "alice")

				rec := httptest.NewRecorder()
				mux.ServeHTTP(rec, req)

				if rec.Code != http.StatusForbidden {
					t.Errorf("got status %d, want %d", rec.Code, http.StatusForbidden)
				}
			})
		}
	}
}

func TestLogoutAcceptsItsOwnOrigin(t *testing.T) {
	app := newTestApp(t)
	app.logoutURL = "https://kc.example.com/logout"
	mux := app.newMux()

	req := httptest.NewRequest("POST", "/auth/logout", nil)
	req.Header.Set("Origin", app.origin)
	signIn(t, app, req, "alice")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusSeeOther)
	}
}

func TestServedFilesCarryHardeningHeaders(t *testing.T) {
	app := newTestApp(t)
	mux := app.newMux()

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/alice/alte%20fotos/%C3%9Cbung%201.pdf", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("got X-Content-Type-Options %q, want %q", got, "nosniff")
	}
	if got := rec.Header().Get("Content-Security-Policy"); got == "" {
		t.Error("served files carry no Content-Security-Policy")
	}
}

func TestDirectoryListingIsNotLockedDown(t *testing.T) {
	app := newTestApp(t)
	mux := app.newMux()

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/alice/", nil))

	if got := rec.Header().Get("Content-Security-Policy"); got != "" {
		t.Errorf("the browse page carries a Content-Security-Policy %q, which would break its own markup", got)
	}
}

func TestMkdirRedirectEscapesThePath(t *testing.T) {
	tests := []struct {
		name string
		dir  string
		want string
	}{
		{"root of the area", "", "/alice/"},
		{"directory with a space", "alte fotos", "/alice/alte%20fotos/"},
		{"directory with a hash", "a#b", "/alice/a%23b/"},
		{"directory with a question mark", "a?b", "/alice/a%3Fb/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newTestApp(t)

			form := url.Values{"dir": {tt.dir}, "name": {"neuer ordner"}}
			req := httptest.NewRequest("POST", "/mkdir", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			signIn(t, app, req, "alice")

			rec := httptest.NewRecorder()
			app.requireUser(http.HandlerFunc(app.mkdirHandler)).ServeHTTP(rec, req)

			if rec.Code != http.StatusSeeOther {
				t.Fatalf("got status %d, want %d (body: %s)", rec.Code, http.StatusSeeOther, rec.Body)
			}
			if got := rec.Header().Get("Location"); got != tt.want {
				t.Errorf("got Location %q, want %q", got, tt.want)
			}
		})
	}
}

func setBuildInfo(t *testing.T, v, rev string) {
	t.Helper()

	oldVersion, oldRevision := version, revision
	version, revision = v, rev
	t.Cleanup(func() { version, revision = oldVersion, oldRevision })
}

func TestFooterShowsTheBuildVersionOnEveryPage(t *testing.T) {
	setBuildInfo(t, "1.2.3", "abc1234def5678")

	for _, path := range []string{"/", "/alice/", "/qr/alice/"} {
		t.Run(path, func(t *testing.T) {
			app := newTestApp(t)

			rec := httptest.NewRecorder()
			app.newMux().ServeHTTP(rec, httptest.NewRequest("GET", path, nil))

			body := rec.Body.String()
			if !strings.Contains(body, "v1.2.3 (abc1234)") {
				t.Errorf("the footer does not show the build version, body: %s", body)
			}
			if strings.Contains(body, "abc1234def5678") {
				t.Error("the footer shows the full revision instead of a short one")
			}
		})
	}
}

func TestFooterKeepsAnUnstampedVersionAsIs(t *testing.T) {
	setBuildInfo(t, "dev", "unknown")

	rec := httptest.NewRecorder()
	newTestApp(t).newMux().ServeHTTP(rec, httptest.NewRequest("GET", "/alice/", nil))

	if body := rec.Body.String(); !strings.Contains(body, "dev (unknown)") {
		t.Errorf("the footer does not show the unstamped build info, body: %s", body)
	}
}

func TestFooterLinksTheAppNameToTheRepository(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestApp(t).newMux().ServeHTTP(rec, httptest.NewRequest("GET", "/alice/", nil))

	if body := rec.Body.String(); !strings.Contains(body, `href="https://github.com/choffmann/simple-fileupload"`) {
		t.Errorf("the footer does not link the app name to the repository, body: %s", body)
	}
}
