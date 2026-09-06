package api

import (
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"git.hanada.info/tamersunion/danmaku/internal/store"
)

func (s *Server) serveDandanplayAdmin(w http.ResponseWriter, r *http.Request, path string) {
	if s.dandanplay.repository == nil {
		http.NotFound(w, r)
		return
	}
	switch {
	case path == "/api/admin/dandanplay/search" && r.Method == http.MethodGet:
		s.searchDandanplay(w, r)
	case strings.HasPrefix(path, "/api/admin/dandanplay/anime/") && strings.HasSuffix(path, "/episodes") && r.Method == http.MethodGet:
		s.dandanplayEpisodes(w, r, path)
	case path == "/api/admin/dandanplay/pools" && r.Method == http.MethodGet:
		s.listDandanplayPools(w, r)
	case path == "/api/admin/dandanplay/pools" && r.Method == http.MethodPost:
		s.createDandanplayPool(w, r)
	case strings.HasPrefix(path, "/api/admin/dandanplay/pools/") && strings.HasSuffix(path, "/sync") && r.Method == http.MethodPost:
		s.syncDandanplayPool(w, r, path)
	case strings.HasPrefix(path, "/api/admin/dandanplay/pools/") && strings.HasSuffix(path, "/danmaku") && r.Method == http.MethodGet:
		s.listDandanplayPoolDanmaku(w, r, path)
	case strings.HasPrefix(path, "/api/admin/dandanplay/danmaku/") && strings.HasSuffix(path, "/blocked") && r.Method == http.MethodPatch:
		s.blockDandanplayDanmaku(w, r, path)
	case path == "/api/admin/dandanplay/keywords" && r.Method == http.MethodGet:
		s.listDandanplayKeywords(w, r)
	case path == "/api/admin/dandanplay/keywords" && r.Method == http.MethodPost:
		s.createDandanplayKeyword(w, r)
	case strings.HasPrefix(path, "/api/admin/dandanplay/keywords/") && r.Method == http.MethodDelete:
		s.deleteDandanplayKeyword(w, r, path)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) listDandanplayPools(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	data, err := s.dandanplay.repository.DandanplayPools(r.Context(), store.DandanplayPoolFilter{
		Page: queryInt(query.Get("page"), 1), Size: queryInt(query.Get("size"), 20), Query: query.Get("query"),
	})
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) createDandanplayPool(w http.ResponseWriter, r *http.Request) {
	var request struct {
		EpisodeID string `json:"episodeId"`
	}
	if !s.decodeJSON(w, r, &request) {
		return
	}
	request.EpisodeID = strings.TrimSpace(request.EpisodeID)
	if _, err := normalizeDandanplayEpisodeID(request.EpisodeID); err != nil {
		s.writeDandanplayAdminFailure(w, "请输入有效的弹弹play 剧集 ID（正整数）")
		return
	}
	pool, inserted, err := s.dandanplay.PreparePool(r.Context(), request.EpisodeID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if pool == nil {
		s.writeDandanplayAdminFailure(w, "未找到对应的弹弹play弹幕池")
		return
	}
	s.writeJSON(w, http.StatusOK, success(map[string]any{"pool": pool, "inserted": inserted}))
}

func (s *Server) syncDandanplayPool(w http.ResponseWriter, r *http.Request, path string) {
	id := pathID(path, "/api/admin/dandanplay/pools/", "/sync")
	if id < 1 {
		s.writeDandanplayAdminFailure(w, "弹幕池 ID 无效")
		return
	}
	pool, inserted, err := s.dandanplay.SyncPool(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if pool == nil {
		s.writeDandanplayAdminFailure(w, "弹幕池不存在")
		return
	}
	s.writeJSON(w, http.StatusOK, success(map[string]any{"pool": pool, "inserted": inserted}))
}

func (s *Server) listDandanplayPoolDanmaku(w http.ResponseWriter, r *http.Request, path string) {
	poolID := pathID(path, "/api/admin/dandanplay/pools/", "/danmaku")
	if poolID < 1 {
		s.writeDandanplayAdminFailure(w, "弹幕池 ID 无效")
		return
	}
	query := r.URL.Query()
	var blocked *bool
	if raw := query.Get("blocked"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			s.writeDandanplayAdminFailure(w, "屏蔽状态无效")
			return
		}
		blocked = &value
	}
	data, err := s.dandanplay.repository.DandanplayDanmaku(r.Context(), store.DandanplayDanmakuFilter{
		Page: queryInt(query.Get("page"), 1), Size: queryInt(query.Get("size"), 20), PoolID: poolID,
		Query: query.Get("query"), Blocked: blocked,
	})
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) blockDandanplayDanmaku(w http.ResponseWriter, r *http.Request, path string) {
	id := pathID(path, "/api/admin/dandanplay/danmaku/", "/blocked")
	if id < 1 {
		s.writeDandanplayAdminFailure(w, "弹幕 ID 无效")
		return
	}
	var request struct {
		Blocked bool `json:"blocked"`
	}
	if !s.decodeJSON(w, r, &request) {
		return
	}
	ok, err := s.dandanplay.repository.SetDandanplayDanmakuBlocked(r.Context(), int64(id), request.Blocked)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if !ok {
		s.writeDandanplayAdminFailure(w, "弹幕不存在")
		return
	}
	s.writeJSON(w, http.StatusOK, success(nil))
}

func (s *Server) listDandanplayKeywords(w http.ResponseWriter, r *http.Request) {
	data, err := s.dandanplay.repository.DandanplayKeywords(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) createDandanplayKeyword(w http.ResponseWriter, r *http.Request) {
	var request struct {
		PoolID  *int   `json:"poolId"`
		Keyword string `json:"keyword"`
	}
	if !s.decodeJSON(w, r, &request) {
		return
	}
	request.Keyword = strings.TrimSpace(request.Keyword)
	if request.Keyword == "" || utf8.RuneCountInString(request.Keyword) > 200 {
		s.writeDandanplayAdminFailure(w, "关键词长度应为 1 到 200 个字符")
		return
	}
	data, err := s.dandanplay.repository.CreateDandanplayKeyword(r.Context(), request.PoolID, request.Keyword)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) deleteDandanplayKeyword(w http.ResponseWriter, r *http.Request, path string) {
	id := pathID(path, "/api/admin/dandanplay/keywords/", "")
	ok, err := s.dandanplay.repository.DeleteDandanplayKeyword(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if !ok {
		s.writeDandanplayAdminFailure(w, "关键词不存在")
		return
	}
	s.writeJSON(w, http.StatusOK, success(nil))
}

func (s *Server) writeDandanplayAdminFailure(w http.ResponseWriter, message string) {
	s.writeJSON(w, http.StatusOK, result{Code: 1, Data: map[string]string{"desc": message}})
}
