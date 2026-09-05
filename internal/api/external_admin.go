package api

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"git.hanada.info/tamersunion/danmaku/internal/store"
)

const maxExternalImportBody = 128 << 20

func (s *Server) serveExternalAdmin(w http.ResponseWriter, r *http.Request, path string) {
	repository, ok := s.repository.(store.ExternalRepository)
	if !ok {
		s.writeExternalFailure(w, "外部弹幕库不可用")
		return
	}
	const base = "/api/admin/external"
	suffix := strings.Trim(strings.TrimPrefix(path, base), "/")
	if suffix == "" {
		switch r.Method {
		case http.MethodGet:
			query := r.URL.Query()
			data, err := repository.ExternalPools(r.Context(), store.ExternalPoolFilter{Page: queryInt(query.Get("page"), 1), Size: queryInt(query.Get("size"), 20), Query: query.Get("query")})
			if err != nil {
				s.writeError(w, err)
				return
			}
			s.writeJSON(w, http.StatusOK, success(data))
		case http.MethodPost:
			s.writeExternalImport(w, r, repository, "")
		default:
			http.NotFound(w, r)
		}
		return
	}
	parts := strings.Split(suffix, "/")
	id := parts[0]
	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		data, err := repository.ExternalPool(r.Context(), id)
		if err != nil {
			s.writeError(w, err)
			return
		}
		if data == nil {
			s.writeExternalFailure(w, "弹幕池不存在")
			return
		}
		s.writeJSON(w, http.StatusOK, success(data))
	case len(parts) == 1 && r.Method == http.MethodPut:
		s.writeExternalImport(w, r, repository, id)
	case len(parts) == 2 && parts[1] == "danmaku" && r.Method == http.MethodGet:
		query := r.URL.Query()
		data, err := repository.ExternalDanmaku(r.Context(), store.ExternalDanmakuFilter{Page: queryInt(query.Get("page"), 1), Size: queryInt(query.Get("size"), 50), PoolID: id, Query: query.Get("query")})
		if err != nil {
			s.writeError(w, err)
			return
		}
		s.writeJSON(w, http.StatusOK, success(data))
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) writeExternalImport(w http.ResponseWriter, r *http.Request, repository store.ExternalRepository, id string) {
	var request struct {
		Name         string               `json:"name"`
		SourceFormat string               `json:"sourceFormat"`
		Danmaku      []domain.DanmakuData `json:"danmaku"`
	}
	if !s.decodeJSONLimit(w, r, &request, maxExternalImportBody) {
		return
	}
	request.Name, request.SourceFormat = strings.TrimSpace(request.Name), strings.TrimSpace(request.SourceFormat)
	if request.Name == "" || utf8.RuneCountInString(request.Name) > 200 || request.SourceFormat == "" || utf8.RuneCountInString(request.SourceFormat) > 64 {
		s.writeExternalFailure(w, "名称和导入格式不能为空，名称不能超过 200 个字符")
		return
	}
	var data *domain.ExternalPool
	var err error
	if id == "" {
		data, err = repository.CreateExternalPool(r.Context(), request.Name, request.SourceFormat, request.Danmaku)
	} else {
		data, err = repository.ReplaceExternalPool(r.Context(), id, request.Name, request.SourceFormat, request.Danmaku)
	}
	if err != nil {
		s.writeError(w, err)
		return
	}
	if data == nil {
		s.writeExternalFailure(w, "弹幕池不存在")
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) writeExternalFailure(w http.ResponseWriter, message string) {
	s.writeJSON(w, http.StatusOK, result{Code: 1, Data: map[string]string{"desc": message}})
}
