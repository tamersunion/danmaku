package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

const (
	sessionCookie = "DCookie"
	roleCookie    = "ClientAuth"
)

type loginRequest struct {
	Name     string  `json:"name"`
	Password string  `json:"password"`
	URL      *string `json:"url"`
}

type session struct {
	User    string `json:"user"`
	Role    int    `json:"role"`
	Expires int64  `json:"expires"`
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var request loginRequest
	if !s.decodeJSON(w, r, &request) {
		return
	}
	ok, uid, role, err := s.repository.VerifyPassword(r.Context(), request.Name, request.Password)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if !ok {
		s.writeJSON(w, http.StatusOK, result{Code: 1, Data: map[string]any{"url": request.URL}})
		return
	}

	maxAge := time.Duration(s.config.Admin.MaxAge) * time.Minute
	expires := time.Now().Add(maxAge)
	value, err := s.signSession(session{User: request.Name, Role: role, Expires: expires.Unix()})
	if err != nil {
		s.writeError(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: value, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: int(maxAge.Seconds()), Expires: expires})
	http.SetCookie(w, &http.Cookie{Name: roleCookie, Value: roleName(role), Path: "/", HttpOnly: false, SameSite: http.SameSiteLaxMode, MaxAge: int(maxAge.Seconds()), Expires: expires})
	redirect := "/"
	if request.URL != nil && isLocalURL(*request.URL) {
		redirect = *request.URL
	}
	s.writeJSON(w, http.StatusOK, success(map[string]any{"url": redirect, "uid": uid}))
}

func (s *Server) logout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(1, 0), HttpOnly: true, SameSite: http.SameSiteLaxMode})
	http.SetCookie(w, &http.Cookie{Name: roleCookie, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(1, 0), SameSite: http.SameSiteLaxMode})
	w.Header().Set("Location", "/")
	w.WriteHeader(http.StatusFound)
}

func (s *Server) signSession(value session) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, s.sessionSecret)
	_, _ = mac.Write(payload)
	signature := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (s *Server) authenticate(r *http.Request) (string, int, bool) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return "", 0, false
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 {
		return "", 0, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", 0, false
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", 0, false
	}
	mac := hmac.New(sha256.New, s.sessionSecret)
	_, _ = mac.Write(payload)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return "", 0, false
	}
	var value session
	if json.Unmarshal(payload, &value) != nil || value.Expires < time.Now().Unix() || (value.Role != 1 && value.Role != 2) {
		return "", 0, false
	}
	return value.User, value.Role, true
}

func isLocalURL(value string) bool {
	return strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") && !strings.HasPrefix(value, "/\\")
}

func roleName(role int) string {
	switch role {
	case 1:
		return "SuperAdmin"
	case 2:
		return "Admin"
	case 3:
		return "GeneralUser"
	default:
		return "Guests"
	}
}
