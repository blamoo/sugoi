package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"
)

var sessionStore *sessions.CookieStore

func InitializeSession() {
	sessionStore = sessions.NewCookieStore(config.SessionCookieKey)
	sessionStore.Options.Secure = false
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
		log.Println(err)
	}

	fmt.Println(r.Cookies())

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

	if !CheckPasswordHash(fPassword, foundPass) {
		return "", true
	}

	return foundUser, false
}

func HandleAuth(w http.ResponseWriter, r *http.Request) (string, bool) {
	userBasic, failedBasic := CheckAuthBasic(w, r)
	debugPrintf("failedBasic: %t\n", failedBasic)

	if !failedBasic {
		return userBasic, failedBasic
	}

	userSession, failedSession := CheckAuth(w, r)
	debugPrintf("failedSession: %t\n", failedSession)

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
