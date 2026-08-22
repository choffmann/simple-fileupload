package config

import (
	"os"
	"strings"
)

func BaseURL() string {
	return strings.TrimSpace(os.Getenv("BASE_URL"))
}

func UploadDir() string {
	v := strings.TrimSpace(os.Getenv("UPLOAD_DIR"))
	if v == "" {
		return "./data"
	}
	return v
}

func UsersFile() string {
	return strings.TrimSpace(os.Getenv("USERS_FILE"))
}
