package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"git.hanada.info/tamersunion/danmaku/internal/store"
)

type managedUser struct {
	ID             int       `json:"id"`
	Name           string    `json:"name"`
	DisplayName    string    `json:"displayName"`
	Role           string    `json:"role"`
	SuperAdmin     bool      `json:"superAdmin"`
	Enabled        bool      `json:"enabled"`
	Provider       string    `json:"provider"`
	ProfileMutable bool      `json:"profileMutable"`
	Email          *string   `json:"email"`
	Avatar         *string   `json:"avatar"`
	CreateTime     time.Time `json:"createTime"`
	UpdateTime     time.Time `json:"updateTime"`
}

type managedUserPage struct {
	Total int           `json:"total"`
	List  []managedUser `json:"list"`
}

func toManagedUser(user domain.User, casEnabled bool) managedUser {
	provider := "local"
	displayName := user.Name
	if user.CASSubject != nil {
		provider = "cas"
		if user.CASDisplayName != nil && strings.TrimSpace(*user.CASDisplayName) != "" {
			displayName = *user.CASDisplayName
		}
	}
	role := "user"
	switch user.Role {
	case 1:
		role = "administrator"
	case 2:
		role = "danmaku_manager"
	}
	return managedUser{
		ID: user.ID, Name: user.Name, DisplayName: displayName, Role: role,
		SuperAdmin: user.Role == 1, Enabled: user.Enabled, Provider: provider,
		ProfileMutable: !casEnabled && provider == "local", Email: user.Email,
		Avatar:     user.CASAvatar,
		CreateTime: user.CreateTime, UpdateTime: user.UpdateTime,
	}
}

func (s *Server) serveUsers(w http.ResponseWriter, r *http.Request, path string, current session) {
	if !canManageUsers(current.Role) {
		s.writeJSON(w, http.StatusOK, result{Code: 401, Data: map[string]string{"desc": "没有权限"}})
		return
	}
	if path == "/api/admin/users" {
		switch r.Method {
		case http.MethodGet:
			s.listUsers(w, r)
		case http.MethodPost:
			s.createUser(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}
	id, action, ok := managedUserPath(path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	switch {
	case action == "" && r.Method == http.MethodGet:
		s.getManagedUser(w, r, id)
	case action == "" && r.Method == http.MethodPut:
		s.updateManagedUser(w, r, current, id)
	case action == "" && r.Method == http.MethodDelete:
		s.deleteManagedUser(w, r, current, id)
	case action == "status" && r.Method == http.MethodPatch:
		s.setManagedUserStatus(w, r, current, id)
	default:
		http.NotFound(w, r)
	}
}

func managedUserPath(path string) (int, string, bool) {
	remainder := strings.TrimPrefix(path, "/api/admin/users/")
	parts := strings.Split(remainder, "/")
	if len(parts) < 1 || len(parts) > 2 {
		return 0, "", false
	}
	id, err := strconv.Atoi(parts[0])
	if err != nil || id <= 0 {
		return 0, "", false
	}
	action := ""
	if len(parts) == 2 {
		action = parts[1]
	}
	return id, action, true
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	page, err := s.repository.Users(r.Context(), store.UserFilter{
		Page: queryInt(query.Get("page"), 1), Size: queryInt(query.Get("size"), 20),
		Query: query.Get("q"), Role: query.Get("role"),
	})
	if err != nil {
		s.writeError(w, err)
		return
	}
	items := make([]managedUser, 0, len(page.List))
	for _, user := range page.List {
		items = append(items, toManagedUser(user, s.config.CAS.Enabled))
	}
	s.writeJSON(w, http.StatusOK, success(managedUserPage{Total: page.Total, List: items}))
}

func (s *Server) getManagedUser(w http.ResponseWriter, r *http.Request, id int) {
	user, err := s.repository.User(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if user == nil {
		s.writeManagedUserError(w, http.StatusNotFound, "用户不存在")
		return
	}
	s.writeJSON(w, http.StatusOK, success(toManagedUser(*user, s.config.CAS.Enabled)))
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	if s.config.CAS.Enabled {
		s.writeManagedUserError(w, http.StatusConflict, "CAS 已启用，用户应在首次 CAS 登录时自动同步")
		return
	}
	var request struct {
		Name     string `json:"name"`
		Password string `json:"password"`
		Role     string `json:"role"`
		Email    string `json:"email"`
	}
	if !s.decodeJSON(w, r, &request) {
		return
	}
	role, ok := managedRole(request.Role)
	if !ok {
		s.writeManagedUserError(w, http.StatusBadRequest, "用户角色无效")
		return
	}
	user, err := s.repository.CreateUser(r.Context(), store.UserCreate{
		Name: request.Name, Password: request.Password, Role: role,
		Email: request.Email,
	})
	if err != nil {
		s.handleManagedUserError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(toManagedUser(*user, s.config.CAS.Enabled)))
}

func (s *Server) updateManagedUser(w http.ResponseWriter, r *http.Request, current session, id int) {
	target, err := s.repository.User(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if target == nil {
		s.writeManagedUserError(w, http.StatusNotFound, "用户不存在")
		return
	}
	if target.Role == 1 && current.Role != 1 {
		s.writeManagedUserError(w, http.StatusForbidden, "无权修改超级管理员")
		return
	}
	var request struct {
		Name     *string `json:"name"`
		Password *string `json:"password"`
		Role     string  `json:"role"`
		Email    *string `json:"email"`
	}
	if !s.decodeJSON(w, r, &request) {
		return
	}
	role, ok := managedRole(request.Role)
	if !ok {
		s.writeManagedUserError(w, http.StatusBadRequest, "用户角色无效")
		return
	}
	if id == current.UID && role != target.Role {
		s.writeManagedUserError(w, http.StatusConflict, "不能修改自己的角色")
		return
	}
	if s.config.CAS.Enabled && (request.Name != nil || request.Password != nil || request.Email != nil) {
		s.writeManagedUserError(w, http.StatusConflict, "CAS 已启用，用户资料只能从 CAS 同步")
		return
	}
	input := store.UserUpdate{
		Name: target.Name, Role: role, Email: pointerString(target.Email),
		PhoneNumber: pointerString(target.PhoneNumber),
	}
	if request.Name != nil {
		input.Name = *request.Name
	}
	if request.Password != nil {
		input.Password = *request.Password
	}
	if request.Email != nil {
		input.Email = *request.Email
	}
	updated, err := s.repository.UpdateUser(r.Context(), id, input)
	if err != nil {
		s.handleManagedUserError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(toManagedUser(*updated, s.config.CAS.Enabled)))
}

func managedRole(value string) (int, bool) {
	switch value {
	case "administrator":
		return 1, true
	case "danmaku_manager":
		return 2, true
	case "user":
		return 3, true
	default:
		return 0, false
	}
}

func (s *Server) setManagedUserStatus(w http.ResponseWriter, r *http.Request, current session, id int) {
	if id == current.UID {
		s.writeManagedUserError(w, http.StatusConflict, "不能停用自己的账户")
		return
	}
	target, ok := s.mutableManagedTarget(w, r, current, id)
	if !ok {
		return
	}
	var request struct {
		Enabled bool `json:"enabled"`
	}
	if !s.decodeJSON(w, r, &request) {
		return
	}
	changed, err := s.repository.SetUserEnabled(r.Context(), target.ID, request.Enabled)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, result{Code: boolCode(changed), Data: nil})
}

func (s *Server) deleteManagedUser(w http.ResponseWriter, r *http.Request, current session, id int) {
	if id == current.UID {
		s.writeManagedUserError(w, http.StatusConflict, "不能删除自己的账户")
		return
	}
	target, ok := s.mutableManagedTarget(w, r, current, id)
	if !ok {
		return
	}
	deleted, err := s.repository.DeleteUser(r.Context(), target.ID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, result{Code: boolCode(deleted), Data: nil})
}

func (s *Server) mutableManagedTarget(w http.ResponseWriter, r *http.Request, current session, id int) (*domain.User, bool) {
	target, err := s.repository.User(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return nil, false
	}
	if target == nil {
		s.writeManagedUserError(w, http.StatusNotFound, "用户不存在")
		return nil, false
	}
	if target.Role == 1 && current.Role != 1 {
		s.writeManagedUserError(w, http.StatusForbidden, "无权修改超级管理员")
		return nil, false
	}
	return target, true
}

func (s *Server) handleManagedUserError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrUserNameConflict):
		s.writeManagedUserError(w, http.StatusConflict, "用户名已存在")
	case errors.Is(err, store.ErrCASProfileReadOnly):
		s.writeManagedUserError(w, http.StatusConflict, "CAS 用户资料只能从 CAS 同步")
	default:
		s.writeError(w, err)
	}
}

func (s *Server) writeManagedUserError(w http.ResponseWriter, status int, description string) {
	s.writeJSON(w, status, result{Code: 1, Data: map[string]string{"desc": description}})
}

func pointerString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
