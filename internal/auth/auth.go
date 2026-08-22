package auth

import (
	"encoding/json"
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	Username     string `json:"username"`
	PasswordHash string `json:"password"`
}

func LoadUsers(path string) ([]User, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading users file: %w", err)
	}

	var users []User
	if err := json.Unmarshal(data, &users); err != nil {
		return nil, fmt.Errorf("parsing users file: %w", err)
	}

	return users, nil
}

func Authenticate(users []User, username, password string) (*User, bool) {
	for _, u := range users {
		if u.Username == username {
			if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err == nil {
				return &u, true
			}
			return nil, false
		}
	}
	return nil, false
}

func SingleUser(username, password string) []User {
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return []User{{Username: username, PasswordHash: string(hash)}}
}
