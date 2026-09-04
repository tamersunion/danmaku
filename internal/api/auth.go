package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
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

// loginSuccessData deliberately keeps URL before UID. The legacy ASP.NET
// response used this property order, and existing reverse-proxy integrations
// match the serialized response body exactly.
type loginSuccessData struct {
	URL string `json:"url"`
	UID int    `json:"uid"`
}

type session struct {
	UID         int    `json:"uid,omitempty"`
	User        string `json:"user"`
	Role        int    `json:"role"`
	Email       string `json:"email,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	Avatar      string `json:"avatar,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Expires     int64  `json:"expires"`
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

	if err := s.setSession(w, session{UID: uid, User: request.Name, Role: role, Provider: "local"}, time.Duration(s.config.Admin.MaxAge)*time.Minute); err != nil {
		s.writeError(w, err)
		return
	}
	redirect := "/"
	if request.URL != nil && isLocalURL(*request.URL) {
		redirect = *request.URL
	}
	s.writeJSON(w, http.StatusOK, success(loginSuccessData{URL: redirect, UID: uid}))
}

func (s *Server) logout(w http.ResponseWriter, _ *http.Request) {
	s.clearSession(w)
	w.Header().Set("Location", "/")
	w.WriteHeader(http.StatusFound)
}

func (s *Server) setSession(w http.ResponseWriter, value session, maxAge time.Duration) error {
	if maxAge <= 0 {
		return errors.New("session lifetime must be positive")
	}
	expires := time.Now().Add(maxAge)
	value.Expires = expires.Unix()
	signed, err := s.signSession(value)
	if err != nil {
		return err
	}
	secure := s.config.CAS.Enabled && s.config.CAS.CookieSecure
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: signed, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: int(maxAge.Seconds()), Expires: expires})
	http.SetCookie(w, &http.Cookie{Name: roleCookie, Value: roleName(value.Role), Path: "/", HttpOnly: false, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: int(maxAge.Seconds()), Expires: expires})
	return nil
}

func (s *Server) clearSession(w http.ResponseWriter) {
	secure := s.config.CAS.Enabled && s.config.CAS.CookieSecure
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(1, 0), HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode})
	http.SetCookie(w, &http.Cookie{Name: roleCookie, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(1, 0), Secure: secure, SameSite: http.SameSiteLaxMode})
	// Remove the former OpenResty CAS session during the cut-over.
	http.SetCookie(w, &http.Cookie{Name: "resty_cas_jwt", Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(1, 0), HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode})
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

func (s *Server) requestSession(r *http.Request) (session, bool) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return session{}, false
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 {
		return session{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return session{}, false
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return session{}, false
	}
	mac := hmac.New(sha256.New, s.sessionSecret)
	_, _ = mac.Write(payload)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return session{}, false
	}
	var value session
	if json.Unmarshal(payload, &value) != nil || value.Expires < time.Now().Unix() || (value.Role != 1 && value.Role != 2 && value.Role != 3) {
		return session{}, false
	}
	if value.Provider == "" {
		value.Provider = "local"
	}
	return value, true
}

func (s *Server) authOptions(w http.ResponseWriter, _ *http.Request) {
	s.noStore(w)
	s.writeJSON(w, http.StatusOK, success(map[string]any{
		"casEnabled":     s.config.CAS.Enabled,
		"defaultCAS":     s.config.CAS.Enabled && s.config.CAS.DefaultLogin,
		"casLoginPath":   "/cas/login",
		"localLoginPath": "/login?skipsso=true",
	}))
}

func (s *Server) currentSession(w http.ResponseWriter, r *http.Request) {
	s.noStore(w)
	value, ok := s.requestSession(r)
	if !ok {
		s.writeJSON(w, http.StatusOK, result{Code: 401, Data: nil})
		return
	}
	name := value.DisplayName
	if name == "" {
		name = value.User
	}
	s.writeJSON(w, http.StatusOK, success(map[string]any{
		"id": value.UID, "name": name, "username": value.User,
		"role": roleName(value.Role), "email": value.Email,
		"avatar": value.Avatar, "provider": value.Provider,
	}))
}

func isAdministrator(role int) bool {
	return role == 1 || role == 2
}

func (s *Server) noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Add("Vary", "Cookie")
}

func isLocalURL(value string) bool {
	return len(value) <= 2048 && strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") && !strings.Contains(value, "\\") && !strings.ContainsAny(value, "\r\n")
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
