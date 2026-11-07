package web

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func (srv *Server) checkAuth(w http.ResponseWriter, r *http.Request) (string, bool) {
	session, _ := srv.sessions.Get(r, srv.cfg.SessionCookieName)
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

func (srv *Server) checkAuthBasic(w http.ResponseWriter, r *http.Request) (string, bool) {
	fUser, fPassword, ok1 := r.BasicAuth()

	if !ok1 {
		return "", true
	}

	found := false
	foundUser := ""
	foundPass := ""
	userLowercase := strings.ToLower(fUser)

	for k, v := range srv.cfg.Users {
		if userLowercase == strings.ToLower(k) {
			foundUser = k
			foundPass = v
			found = found || true
		}
	}

	if !found {
		return "", true
	}

	if !srv.checkPasswordHashCached(fPassword, foundPass) {
		return "", true
	}

	return foundUser, false
}

// srv.basicAuthOk remembers credentials that already passed bcrypt. HTTP Basic
// sends the password on every request (every thumbnail), and a cost-14 bcrypt
// takes about a second. Keys are sha256(hash + password), so plaintext
// passwords are not kept in memory, and a changed hash in the config never
// matches an old entry.

func (srv *Server) checkPasswordHashCached(password, hash string) bool {
	sum := sha256.Sum256([]byte(hash + "\x00" + password))
	if _, ok := srv.basicAuthOk.Load(sum); ok {
		return true
	}

	if !CheckPasswordHash(password, hash) {
		return false
	}

	srv.basicAuthOk.Store(sum, struct{}{})
	return true
}

// CheckAnyAuth accepts HTTP Basic or a session cookie, without redirecting.
// Used by JSON endpoints, which answer 403 instead of sending to /login.
func (srv *Server) checkAnyAuth(w http.ResponseWriter, r *http.Request) (string, bool) {
	if user, failed := srv.checkAuthBasic(w, r); !failed {
		return user, false
	}

	return srv.checkAuth(w, r)
}

func (srv *Server) handleAuth(w http.ResponseWriter, r *http.Request) (string, bool) {
	userSession, failedSession := srv.checkAnyAuth(w, r)

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
