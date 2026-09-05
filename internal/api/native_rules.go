package api

import (
	"errors"
	"git.hanada.info/tamersunion/danmaku/internal/store"
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) serveNativeRules(w http.ResponseWriter, r *http.Request, path string) {
	repo, ok := s.repository.(store.NativeRuleRepository)
	if !ok {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(strings.TrimPrefix(path, "/api/admin/danmaku-rules/"), "/")
	kind := parts[0]
	if !store.ValidNativeRuleKind(kind) {
		http.NotFound(w, r)
		return
	}
	writeError := func(err error) {
		message := ""
		if errors.Is(err, store.ErrInvalidNativeRule) {
			message = "规则无效，请检查关键词、用户名或 IP/CIDR 格式"
		}
		if errors.Is(err, store.ErrNativeRuleExists) {
			message = "该规则已存在，请先删除旧规则"
		}
		if message != "" {
			s.writeJSON(w, http.StatusOK, result{Code: 1, Data: map[string]string{"desc": message}})
		} else {
			s.writeError(w, err)
		}
	}
	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		data, err := repo.NativeRules(r.Context(), kind)
		if err != nil {
			writeError(err)
			return
		}
		s.writeJSON(w, http.StatusOK, success(data))
	case len(parts) == 1 && r.Method == http.MethodPost:
		var input store.NativeRuleInput
		if !s.decodeJSON(w, r, &input) {
			return
		}
		data, err := repo.CreateNativeRule(r.Context(), kind, input)
		if err != nil {
			writeError(err)
			return
		}
		s.writeJSON(w, http.StatusOK, success(data))
	case len(parts) == 2 && r.Method == http.MethodDelete:
		id, err := strconv.Atoi(parts[1])
		if err != nil || id < 1 {
			writeError(store.ErrInvalidNativeRule)
			return
		}
		found, err := repo.DeleteNativeRule(r.Context(), kind, id)
		if err != nil {
			writeError(err)
			return
		}
		if !found {
			s.writeJSON(w, http.StatusOK, failure())
			return
		}
		s.writeJSON(w, http.StatusOK, success(nil))
	default:
		http.NotFound(w, r)
	}
}
