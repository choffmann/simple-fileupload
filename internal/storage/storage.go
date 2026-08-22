package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Storage struct {
	BaseDir string
}

type Entry struct {
	Name    string
	IsDir   bool
	Size    int64
	ModTime time.Time
}

func New(baseDir string) *Storage {
	return &Storage{BaseDir: baseDir}
}

func (s *Storage) ResolvePath(username, subpath string) (string, error) {
	userDir := filepath.Join(s.BaseDir, filepath.Clean(username))
	full := filepath.Join(userDir, filepath.Clean("/"+subpath))

	if !strings.HasPrefix(full, userDir) {
		return "", fmt.Errorf("invalid path")
	}

	return full, nil
}

func (s *Storage) EnsureUserDir(username string) error {
	dir := filepath.Join(s.BaseDir, filepath.Clean(username))
	return os.MkdirAll(dir, 0o755)
}

func (s *Storage) SaveFile(username, subpath string, file io.Reader, filename string) error {
	dir, err := s.ResolvePath(username, subpath)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	cleanName := sanitizeFilename(filename)
	dst := filepath.Join(dir, cleanName)

	// Verify the final path is still within user directory
	userDir := filepath.Join(s.BaseDir, filepath.Clean(username))
	if !strings.HasPrefix(dst, userDir) {
		return fmt.Errorf("invalid filename")
	}

	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, file)
	return err
}

func (s *Storage) ListDir(username, subpath string) ([]Entry, error) {
	dir, err := s.ResolvePath(username, subpath)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var result []Entry
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		result = append(result, Entry{
			Name:    e.Name(),
			IsDir:   e.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}

	return result, nil
}

func (s *Storage) ListUsers() ([]string, error) {
	entries, err := os.ReadDir(s.BaseDir)
	if err != nil {
		return nil, err
	}
	var users []string
	for _, e := range entries {
		if e.IsDir() {
			users = append(users, e.Name())
		}
	}
	return users, nil
}

func (s *Storage) UserExists(username string) bool {
	dir := filepath.Join(s.BaseDir, filepath.Clean(username))
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}

func (s *Storage) CreateDir(username, subpath string) error {
	dir, err := s.ResolvePath(username, subpath)
	if err != nil {
		return err
	}
	return os.MkdirAll(dir, 0o755)
}

func (s *Storage) IsDir(username, subpath string) (bool, error) {
	full, err := s.ResolvePath(username, subpath)
	if err != nil {
		return false, err
	}

	info, err := os.Stat(full)
	if err != nil {
		return false, err
	}

	return info.IsDir(), nil
}

func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "..", "")
	name = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == '\x00' {
			return '_'
		}
		return r
	}, name)
	if name == "" || name == "." {
		name = "unnamed"
	}
	return name
}
