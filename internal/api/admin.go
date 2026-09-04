package api

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"git.hanada.info/tamersunion/danmaku/internal/store"
)

func (s *Server) serveAdmin(w http.ResponseWriter, r *http.Request, path string, current session) {
	if !isAdministrator(current.Role) && path != "/api/admin/user/changepassword" && path != "/api/admin/user/changeinfo" && path != "/api/admin/user/user" {
		s.writeJSON(w, http.StatusOK, result{Code: 401, Data: map[string]string{"desc": "没有权限"}})
		return
	}
	switch {
	case path == "/api/admin/users" || strings.HasPrefix(path, "/api/admin/users/"):
		s.serveUsers(w, r, path, current)
	case path == "/api/admin/danmakulist" && r.Method == http.MethodGet:
		s.listDanmaku(w, r)
	case path == "/api/admin/danmakulist/vids" && r.Method == http.MethodGet:
		s.listVids(w, r)
	case path == "/api/admin/danmakulist/dateselect" && r.Method == http.MethodGet:
		s.searchDanmaku(w, r, true)
	case path == "/api/admin/danmakulist/baseselect" && r.Method == http.MethodGet:
		s.searchDanmaku(w, r, false)
	case path == "/api/admin/danmakuedit" && r.Method == http.MethodGet:
		s.getDanmaku(w, r)
	case path == "/api/admin/danmakuedit/edit" && r.Method == http.MethodPost:
		s.editDanmaku(w, r)
	case path == "/api/admin/danmakuedit/delete" && r.Method == http.MethodGet:
		s.deleteDanmaku(w, r)
	case path == "/api/admin/user/changepassword" && r.Method == http.MethodPost:
		s.changePassword(w, r, current)
	case path == "/api/admin/user/changeinfo" && r.Method == http.MethodPost:
		s.changeUserInfo(w, r, current)
	case path == "/api/admin/user/user" && r.Method == http.MethodGet:
		s.userInfo(w, r, current)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) listDanmaku(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	page := queryInt(query.Get("page"), 1)
	size := queryInt(query.Get("size"), 30)
	descending := queryBool(query.Get("descending"), true)
	data, err := s.repository.List(r.Context(), query.Get("vid"), page, size, descending)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) listVids(w http.ResponseWriter, r *http.Request) {
	data, err := s.repository.Vids(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) searchDanmaku(w http.ResponseWriter, r *http.Request, dateOnly bool) {
	query := r.URL.Query()
	filter := store.SearchFilter{
		Page: queryInt(query.Get("page"), 1), Size: queryInt(query.Get("size"), 30),
		Start: parseDate(query.Get("startDate")), End: parseDate(query.Get("endDate")),
		Descending: true,
	}
	if !dateOnly {
		filter.Descending = queryBool(query.Get("descending"), true)
		filter.Vid = query.Get("vid")
		filter.Author = query.Get("author")
		filter.Key = query.Get("key")
		if value := query.Get("authorId"); value != "" {
			if parsed, err := strconv.Atoi(value); err == nil {
				filter.AuthorID = &parsed
			}
		}
		if value := query.Get("mode"); value != "" {
			if parsed, err := strconv.Atoi(value); err == nil && parsed < 100 {
				filter.Mode = &parsed
			}
		}
		if parsed := net.ParseIP(query.Get("ip")); parsed != nil {
			filter.IP = parsed
		}
	}
	data, err := s.repository.Search(r.Context(), filter)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) getDanmaku(w http.ResponseWriter, r *http.Request) {
	data, err := s.repository.Get(r.Context(), r.URL.Query().Get("id"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) editDanmaku(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ID        string             `json:"id"`
		Data      domain.DanmakuData `json:"data"`
		IsDeleted bool               `json:"isDelete"`
	}
	request.Data = domain.NewDanmakuData()
	if !s.decodeJSON(w, r, &request) {
		return
	}
	data, err := s.repository.Edit(r.Context(), request.ID, request.Data, request.IsDeleted)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) deleteDanmaku(w http.ResponseWriter, r *http.Request) {
	ok, err := s.repository.Delete(r.Context(), r.URL.Query().Get("id"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	if ok {
		s.writeJSON(w, http.StatusOK, success(nil))
	} else {
		s.writeJSON(w, http.StatusOK, failure())
	}
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request, current session) {
	if !s.requireLocalSession(w, r) {
		return
	}
	var request struct {
		UID  int    `json:"uid"`
		OldP string `json:"oldP"`
		NewP string `json:"newP"`
	}
	if !s.decodeJSON(w, r, &request) {
		return
	}
	if request.UID != current.UID {
		http.Error(w, "cannot change another user's password", http.StatusForbidden)
		return
	}
	ok, err := s.repository.ChangePassword(r.Context(), request.UID, request.OldP, request.NewP)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, result{Code: boolCode(ok), Data: nil})
}

func (s *Server) changeUserInfo(w http.ResponseWriter, r *http.Request, current session) {
	if !s.requireLocalSession(w, r) {
		return
	}
	var request struct {
		ID          int     `json:"id"`
		Name        string  `json:"name"`
		Email       *string `json:"email"`
		PhoneNumber *string `json:"phoneNumber"`
	}
	if !s.decodeJSON(w, r, &request) {
		return
	}
	if request.ID != current.UID {
		http.Error(w, "cannot change another user's profile", http.StatusForbidden)
		return
	}
	ok, err := s.repository.ChangeUserInfo(r.Context(), request.ID, request.Name, request.Email, request.PhoneNumber)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, result{Code: boolCode(ok), Data: nil})
}

func (s *Server) userInfo(w http.ResponseWriter, r *http.Request, current session) {
	uid := queryInt(r.URL.Query().Get("uid"), 0)
	if uid != current.UID && !isAdministrator(current.Role) {
		http.Error(w, "cannot view another user's profile", http.StatusForbidden)
		return
	}
	user, err := s.repository.User(r.Context(), uid)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if value, ok := s.requestSession(r); ok && value.Provider == "cas" && user != nil && value.UID == user.ID {
		if value.DisplayName != "" {
			user.Name = value.DisplayName
		}
		if value.Email != "" {
			user.Email = &value.Email
		}
	}
	s.writeJSON(w, http.StatusOK, success(user))
}

func boolCode(ok bool) int {
	if ok {
		return 0
	}
	return 1
}

func queryInt(raw string, fallback int) int {
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func queryBool(raw string, fallback bool) bool {
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}

func parseDate(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339Nano, "2006-01-02"} {
		if value, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return &value
		}
	}
	return nil
}
