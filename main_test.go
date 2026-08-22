package main

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/choffmann/simple-fileupload/internal/auth"
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
		store:   store,
		users:   auth.SingleUser("alice", "secret"),
		logger:  slog.New(slog.DiscardHandler),
		baseURL: "https://files.example.com",
	}
}

func qrMux(app *App) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /qr/{username}/{path...}", app.qrHandler)
	return mux
}

func TestQRHandlerRendersPageWithPublicURL(t *testing.T) {
	app := newTestApp(t)

	rec := httptest.NewRecorder()
	qrMux(app).ServeHTTP(rec, httptest.NewRequest("GET", "/qr/alice/alte%20fotos/%C3%9Cbung%201.pdf", nil))

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
	qrMux(app).ServeHTTP(rec, httptest.NewRequest("GET", "/qr/alice/alte%20fotos/%C3%9Cbung%201.pdf?format=png", nil))

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
	qrMux(app).ServeHTTP(rec, httptest.NewRequest("GET", "/qr/alice/alte%20fotos/%C3%9Cbung%201.pdf?format=png&download=1", nil))

	got := rec.Header().Get("Content-Disposition")
	if !strings.HasPrefix(got, "attachment;") {
		t.Errorf("got Content-Disposition %q, want it to start with %q", got, "attachment;")
	}
}

func TestQRHandlerUnknownPathIsNotFound(t *testing.T) {
	app := newTestApp(t)

	rec := httptest.NewRecorder()
	qrMux(app).ServeHTTP(rec, httptest.NewRequest("GET", "/qr/alice/gibtsnicht.pdf", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestQRHandlerDirectoryURLKeepsTrailingSlash(t *testing.T) {
	app := newTestApp(t)

	rec := httptest.NewRecorder()
	qrMux(app).ServeHTTP(rec, httptest.NewRequest("GET", "/qr/alice/alte%20fotos/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body)
	}
	wantURL := "https://files.example.com/alice/alte%20fotos/"
	if !strings.Contains(rec.Body.String(), wantURL) {
		t.Errorf("page does not contain %q\nbody: %s", wantURL, rec.Body)
	}
}
