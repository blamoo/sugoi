package main

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"
)

var sessionStore *sessions.CookieStore

func InitializeSession() {
	sessionStore = sessions.NewCookieStore(config.SessionCookieKey)
	sessionStore.Options.Secure = false
	sessionStore.Options.SameSite = http.SameSiteStrictMode
	sessionStore.MaxAge(config.SessionCookieMaxAge)
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func CheckAuth(w http.ResponseWriter, r *http.Request) (string, bool) {
	session, _ := sessionStore.Get(r, config.SessionCookieName)
	err := session.Save(r, w)
	if err != nil {
		slog.Error("saving session", "err", err)
	}

	auth, ok1 := session.Values["authenticated"].(bool)
	user, ok2 := session.Values["user"].(string)

	if !ok1 || !ok2 || !auth {
		return "", true
	}

	return user, false
}

func CheckAuthBasic(w http.ResponseWriter, r *http.Request) (string, bool) {
	fUser, fPassword, ok1 := r.BasicAuth()

	if !ok1 {
		return "", true
	}

	found := false
	foundUser := ""
	foundPass := ""
	userLowercase := strings.ToLower(fUser)

	for k, v := range config.Users {
		if userLowercase == strings.ToLower(k) {
			foundUser = k
			foundPass = v
			found = found || true
		}
	}

	if !found {
		return "", true
	}

	if !checkPasswordHashCached(fPassword, foundPass) {
		return "", true
	}

	return foundUser, false
}

// basicAuthOk remembers credentials that already passed bcrypt. HTTP Basic
// sends the password on every request (every thumbnail), and a cost-14 bcrypt
// takes about a second. Keys are sha256(hash + password), so plaintext
// passwords are not kept in memory, and a changed hash in the config never
// matches an old entry.
var basicAuthOk sync.Map

func checkPasswordHashCached(password, hash string) bool {
	sum := sha256.Sum256([]byte(hash + "\x00" + password))
	if _, ok := basicAuthOk.Load(sum); ok {
		return true
	}

	if !CheckPasswordHash(password, hash) {
		return false
	}

	basicAuthOk.Store(sum, struct{}{})
	return true
}

// CheckAnyAuth accepts HTTP Basic or a session cookie, without redirecting.
// Used by JSON endpoints, which answer 403 instead of sending to /login.
func CheckAnyAuth(w http.ResponseWriter, r *http.Request) (string, bool) {
	if user, failed := CheckAuthBasic(w, r); !failed {
		return user, false
	}

	return CheckAuth(w, r)
}

func HandleAuth(w http.ResponseWriter, r *http.Request) (string, bool) {
	userSession, failedSession := CheckAnyAuth(w, r)

	if failedSession {
		returnPath := fmt.Sprintf("%s?%s", r.URL.Path, r.URL.RawQuery)
		u := new(url.URL)
		u.Path = "/login"
		q := u.Query()
		q.Set("return", returnPath)
		u.RawQuery = q.Encode()
		http.Redirect(w, r, u.String(), http.StatusFound)
	}

	return userSession, failedSession
}
