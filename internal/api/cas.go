package api

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	casclient "git.hanada.info/tamersunion/danmaku/internal/cas"
	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"git.hanada.info/tamersunion/danmaku/internal/store"
)

const defaultCASReturnTo = "/danmaku/index"

func (s *Server) serveCAS(w http.ResponseWriter, r *http.Request) {
	s.noStore(w)
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	path := strings.TrimSuffix(r.URL.Path, "/")
	switch path {
	case "/cas/login":
		s.casLogin(w, r, "/cas/callback")
	case "/cas/callback":
		s.casCallback(w, r, "/cas/callback")
	case "/cas/auth":
		if r.URL.Query().Get("ticket") == "" {
			s.casLogin(w, r, "/cas/auth")
		} else {
			s.casCallback(w, r, "/cas/auth")
		}
	case "/cas/logout":
		s.casLogout(w)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) casLogin(w http.ResponseWriter, r *http.Request, callbackPath string) {
	returnTo := safeReturnTo(r.URL.Query().Get("returnTo"))
	if _, ok := s.requestSession(r); ok {
		http.Redirect(w, r, returnTo, http.StatusFound)
		return
	}
	if s.cas == nil {
		http.Redirect(w, r, localLoginURL(returnTo), http.StatusFound)
		return
	}
	service := s.casServiceURL(callbackPath, returnTo)
	http.Redirect(w, r, s.cas.LoginURL(service), http.StatusFound)
}

func (s *Server) casCallback(w http.ResponseWriter, r *http.Request, callbackPath string) {
	if s.cas == nil {
		http.Redirect(w, r, localLoginURL(safeReturnTo(r.URL.Query().Get("returnTo"))), http.StatusFound)
		return
	}
	returnTo := safeReturnTo(r.URL.Query().Get("returnTo"))
	service := s.casServiceURL(callbackPath, returnTo)
	identity, err := s.cas.Validate(r.Context(), r.URL.Query().Get("ticket"), service)
	if errors.Is(err, casclient.ErrTicketInvalid) {
		http.Redirect(w, r, "/cas/login?"+url.Values{"returnTo": []string{returnTo}}.Encode(), http.StatusFound)
		return
	}
	if err != nil {
		s.logger.Error("validate CAS ticket", "error", err)
		http.Error(w, "CAS validation failed", http.StatusBadGateway)
		return
	}
	profile := domain.CASProfile{
		Subject: identity.Subject, UserName: identity.UserName, Email: identity.Email,
		DisplayName: identity.DisplayName, Avatar: identity.Avatar,
	}
	user, created, err := s.repository.UpsertCASUser(r.Context(), profile, s.config.CAS.DefaultRole, s.config.CAS.AutoCreateUsers)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, store.ErrCASUserNotFound) || errors.Is(err, store.ErrCASIdentityConflict) || errors.Is(err, store.ErrUserDisabled) {
			status = http.StatusForbidden
		}
		s.logger.Error("provision CAS user", "subject", identity.Subject, "error", err)
		http.Error(w, "CAS account is not permitted", status)
		return
	}
	if created {
		s.logger.Info("created CAS user", "uid", user.ID, "username", user.Name)
	}
	value := session{
		UID: user.ID, User: user.Name, Role: user.Role, Email: identity.Email,
		DisplayName: identity.DisplayName, Avatar: identity.Avatar, Provider: "cas",
	}
	if err := s.setSession(w, value, time.Duration(s.config.CAS.SessionMaxAgeSeconds)*time.Second); err != nil {
		s.writeError(w, err)
		return
	}
	http.Redirect(w, r, returnTo, http.StatusFound)
}

func (s *Server) casLogout(w http.ResponseWriter) {
	s.clearSession(w)
	if s.cas == nil {
		w.Header().Set("Location", "/")
	} else {
		w.Header().Set("Location", s.cas.LogoutURL(strings.TrimRight(s.config.CAS.PublicURL, "/")+"/"))
	}
	w.WriteHeader(http.StatusFound)
}

func (s *Server) casServiceURL(callbackPath, returnTo string) string {
	return strings.TrimRight(s.config.CAS.PublicURL, "/") + callbackPath + "?" + url.Values{"returnTo": []string{returnTo}}.Encode()
}

func safeReturnTo(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultCASReturnTo
	}
	if !isLocalURL(value) {
		return defaultCASReturnTo
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.IsAbs() || parsed.Host != "" {
		return defaultCASReturnTo
	}
	return value
}

func localLoginURL(returnTo string) string {
	query := url.Values{"skipsso": []string{"true"}, "redirect": []string{returnTo}}
	return "/login?" + query.Encode()
}

func (s *Server) requireLocalSession(w http.ResponseWriter, r *http.Request) bool {
	value, ok := s.requestSession(r)
	if ok && !s.config.CAS.Enabled && value.Provider != "cas" {
		return true
	}
	http.Error(w, "local authentication required", http.StatusForbidden)
	return false
}
