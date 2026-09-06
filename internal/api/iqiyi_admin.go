package api

import (
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"git.hanada.info/tamersunion/danmaku/internal/store"
)

func (s *Server) serveIqiyiAdmin(w http.ResponseWriter, r *http.Request, path string) {
	switch {
	case path == "/api/admin/iqiyi/search" && r.Method == http.MethodGet:
		s.searchIqiyi(w, r)
	case strings.HasPrefix(path, "/api/admin/iqiyi/anime/") && strings.HasSuffix(path, "/episodes") && r.Method == http.MethodGet:
		s.iqiyiEpisodes(w, r, path)
	case path == "/api/admin/iqiyi/pools" && r.Method == http.MethodGet:
		s.listIqiyiPools(w, r)
	case path == "/api/admin/iqiyi/pools" && r.Method == http.MethodPost:
		s.createIqiyiPool(w, r)
	case strings.HasPrefix(path, "/api/admin/iqiyi/pools/") && strings.HasSuffix(path, "/sync") && r.Method == http.MethodPost:
		s.syncIqiyiPool(w, r, path)
	case strings.HasPrefix(path, "/api/admin/iqiyi/pools/") && strings.HasSuffix(path, "/danmaku") && r.Method == http.MethodGet:
		s.listIqiyiPoolDanmaku(w, r, path)
	case strings.HasPrefix(path, "/api/admin/iqiyi/danmaku/") && strings.HasSuffix(path, "/blocked") && r.Method == http.MethodPatch:
		s.blockIqiyiDanmaku(w, r, path)
	case path == "/api/admin/iqiyi/keywords" && r.Method == http.MethodGet:
		s.listIqiyiKeywords(w, r)
	case path == "/api/admin/iqiyi/keywords" && r.Method == http.MethodPost:
		s.createIqiyiKeyword(w, r)
	case strings.HasPrefix(path, "/api/admin/iqiyi/keywords/") && r.Method == http.MethodDelete:
		s.deleteIqiyiKeyword(w, r, path)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) listIqiyiPools(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	data, err := s.repository.IqiyiPools(r.Context(), store.IqiyiPoolFilter{
		Page: queryInt(query.Get("page"), 1), Size: queryInt(query.Get("size"), 20), Query: query.Get("query"),
	})
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) createIqiyiPool(w http.ResponseWriter, r *http.Request) {
	var request struct {
		VID string `json:"vid"`
	}
	if !s.decodeJSON(w, r, &request) {
		return
	}
	request.VID = strings.TrimSpace(request.VID)
	if request.VID == "" || utf8.RuneCountInString(request.VID) > 128 {
		s.writeIqiyiAdminFailure(w, "爱奇艺 VID 长度应为 1 到 128 个字符")
		return
	}
	pool, inserted, err := s.iqiyi.PreparePool(r.Context(), request.VID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if pool == nil {
		s.writeIqiyiAdminFailure(w, "未找到对应的爱奇艺弹幕池")
		return
	}
	s.writeJSON(w, http.StatusOK, success(map[string]any{"pool": pool, "inserted": inserted}))
}

func (s *Server) syncIqiyiPool(w http.ResponseWriter, r *http.Request, path string) {
	id := pathID(path, "/api/admin/iqiyi/pools/", "/sync")
	if id < 1 {
		s.writeIqiyiAdminFailure(w, "弹幕池 ID 无效")
		return
	}
	pool, inserted, err := s.iqiyi.SyncPool(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if pool == nil {
		s.writeIqiyiAdminFailure(w, "弹幕池不存在")
		return
	}
	s.writeJSON(w, http.StatusOK, success(map[string]any{"pool": pool, "inserted": inserted}))
}

func (s *Server) listIqiyiPoolDanmaku(w http.ResponseWriter, r *http.Request, path string) {
	poolID := pathID(path, "/api/admin/iqiyi/pools/", "/danmaku")
	if poolID < 1 {
		s.writeIqiyiAdminFailure(w, "弹幕池 ID 无效")
		return
	}
	query := r.URL.Query()
	var blocked *bool
	if raw := query.Get("blocked"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			s.writeIqiyiAdminFailure(w, "屏蔽状态无效")
			return
		}
		blocked = &value
	}
	data, err := s.repository.IqiyiDanmaku(r.Context(), store.IqiyiDanmakuFilter{
		Page: queryInt(query.Get("page"), 1), Size: queryInt(query.Get("size"), 20), PoolID: poolID,
		Query: query.Get("query"), Blocked: blocked,
	})
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) blockIqiyiDanmaku(w http.ResponseWriter, r *http.Request, path string) {
	id := pathID(path, "/api/admin/iqiyi/danmaku/", "/blocked")
	if id < 1 {
		s.writeIqiyiAdminFailure(w, "弹幕 ID 无效")
		return
	}
	var request struct {
		Blocked bool `json:"blocked"`
	}
	if !s.decodeJSON(w, r, &request) {
		return
	}
	ok, err := s.repository.SetIqiyiDanmakuBlocked(r.Context(), int64(id), request.Blocked)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if !ok {
		s.writeIqiyiAdminFailure(w, "弹幕不存在")
		return
	}
	s.writeJSON(w, http.StatusOK, success(nil))
}

func (s *Server) listIqiyiKeywords(w http.ResponseWriter, r *http.Request) {
	data, err := s.repository.IqiyiKeywords(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) createIqiyiKeyword(w http.ResponseWriter, r *http.Request) {
	var request struct {
		PoolID  *int   `json:"poolId"`
		Keyword string `json:"keyword"`
	}
	if !s.decodeJSON(w, r, &request) {
		return
	}
	request.Keyword = strings.TrimSpace(request.Keyword)
	if request.Keyword == "" || utf8.RuneCountInString(request.Keyword) > 200 {
		s.writeIqiyiAdminFailure(w, "关键词长度应为 1 到 200 个字符")
		return
	}
	data, err := s.repository.CreateIqiyiKeyword(r.Context(), request.PoolID, request.Keyword)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) deleteIqiyiKeyword(w http.ResponseWriter, r *http.Request, path string) {
	id := pathID(path, "/api/admin/iqiyi/keywords/", "")
	ok, err := s.repository.DeleteIqiyiKeyword(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if !ok {
		s.writeIqiyiAdminFailure(w, "关键词不存在")
		return
	}
	s.writeJSON(w, http.StatusOK, success(nil))
}

func (s *Server) writeIqiyiAdminFailure(w http.ResponseWriter, message string) {
	s.writeJSON(w, http.StatusOK, result{Code: 1, Data: map[string]string{"desc": message}})
}
