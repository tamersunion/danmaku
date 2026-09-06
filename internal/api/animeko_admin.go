package api

import (
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"git.hanada.info/tamersunion/danmaku/internal/store"
)

func (s *Server) serveAnimekoAdmin(w http.ResponseWriter, r *http.Request, path string) {
	if s.animeko.repository == nil {
		http.NotFound(w, r)
		return
	}
	switch {
	case path == "/api/admin/animeko/search" && r.Method == http.MethodGet:
		s.searchAnimeko(w, r)
	case strings.HasPrefix(path, "/api/admin/animeko/anime/") && strings.HasSuffix(path, "/episodes") && r.Method == http.MethodGet:
		s.animekoEpisodes(w, r, path)
	case path == "/api/admin/animeko/pools" && r.Method == http.MethodGet:
		s.listAnimekoPools(w, r)
	case path == "/api/admin/animeko/pools" && r.Method == http.MethodPost:
		s.createAnimekoPool(w, r)
	case strings.HasPrefix(path, "/api/admin/animeko/pools/") && strings.HasSuffix(path, "/sync") && r.Method == http.MethodPost:
		s.syncAnimekoPool(w, r, path)
	case strings.HasPrefix(path, "/api/admin/animeko/pools/") && strings.HasSuffix(path, "/danmaku") && r.Method == http.MethodGet:
		s.listAnimekoPoolDanmaku(w, r, path)
	case strings.HasPrefix(path, "/api/admin/animeko/danmaku/") && strings.HasSuffix(path, "/blocked") && r.Method == http.MethodPatch:
		s.blockAnimekoDanmaku(w, r, path)
	case path == "/api/admin/animeko/keywords" && r.Method == http.MethodGet:
		s.listAnimekoKeywords(w, r)
	case path == "/api/admin/animeko/keywords" && r.Method == http.MethodPost:
		s.createAnimekoKeyword(w, r)
	case strings.HasPrefix(path, "/api/admin/animeko/keywords/") && r.Method == http.MethodDelete:
		s.deleteAnimekoKeyword(w, r, path)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) listAnimekoPools(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	data, err := s.animeko.repository.AnimekoPools(r.Context(), store.AnimekoPoolFilter{
		Page: queryInt(query.Get("page"), 1), Size: queryInt(query.Get("size"), 20), Query: query.Get("query"),
	})
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) createAnimekoPool(w http.ResponseWriter, r *http.Request) {
	var request struct {
		EpisodeID string `json:"episodeId"`
	}
	if !s.decodeJSON(w, r, &request) {
		return
	}
	request.EpisodeID = strings.TrimSpace(request.EpisodeID)
	if _, err := normalizeAnimekoEpisodeID(request.EpisodeID); err != nil {
		s.writeAnimekoAdminFailure(w, "请输入有效的 Animeko 剧集 ID（正整数）")
		return
	}
	pool, inserted, err := s.animeko.PreparePool(r.Context(), request.EpisodeID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if pool == nil {
		s.writeAnimekoAdminFailure(w, "未找到对应的 Animeko弹幕池")
		return
	}
	s.writeJSON(w, http.StatusOK, success(map[string]any{"pool": pool, "inserted": inserted}))
}

func (s *Server) syncAnimekoPool(w http.ResponseWriter, r *http.Request, path string) {
	id := pathID(path, "/api/admin/animeko/pools/", "/sync")
	if id < 1 {
		s.writeAnimekoAdminFailure(w, "弹幕池 ID 无效")
		return
	}
	pool, inserted, err := s.animeko.SyncPool(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if pool == nil {
		s.writeAnimekoAdminFailure(w, "弹幕池不存在")
		return
	}
	s.writeJSON(w, http.StatusOK, success(map[string]any{"pool": pool, "inserted": inserted}))
}

func (s *Server) listAnimekoPoolDanmaku(w http.ResponseWriter, r *http.Request, path string) {
	poolID := pathID(path, "/api/admin/animeko/pools/", "/danmaku")
	if poolID < 1 {
		s.writeAnimekoAdminFailure(w, "弹幕池 ID 无效")
		return
	}
	query := r.URL.Query()
	var blocked *bool
	if raw := query.Get("blocked"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			s.writeAnimekoAdminFailure(w, "屏蔽状态无效")
			return
		}
		blocked = &value
	}
	data, err := s.animeko.repository.AnimekoDanmaku(r.Context(), store.AnimekoDanmakuFilter{
		Page: queryInt(query.Get("page"), 1), Size: queryInt(query.Get("size"), 20), PoolID: poolID,
		Query: query.Get("query"), Blocked: blocked,
	})
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) blockAnimekoDanmaku(w http.ResponseWriter, r *http.Request, path string) {
	id := pathID(path, "/api/admin/animeko/danmaku/", "/blocked")
	if id < 1 {
		s.writeAnimekoAdminFailure(w, "弹幕 ID 无效")
		return
	}
	var request struct {
		Blocked bool `json:"blocked"`
	}
	if !s.decodeJSON(w, r, &request) {
		return
	}
	ok, err := s.animeko.repository.SetAnimekoDanmakuBlocked(r.Context(), int64(id), request.Blocked)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if !ok {
		s.writeAnimekoAdminFailure(w, "弹幕不存在")
		return
	}
	s.writeJSON(w, http.StatusOK, success(nil))
}

func (s *Server) listAnimekoKeywords(w http.ResponseWriter, r *http.Request) {
	data, err := s.animeko.repository.AnimekoKeywords(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) createAnimekoKeyword(w http.ResponseWriter, r *http.Request) {
	var request struct {
		PoolID  *int   `json:"poolId"`
		Keyword string `json:"keyword"`
	}
	if !s.decodeJSON(w, r, &request) {
		return
	}
	request.Keyword = strings.TrimSpace(request.Keyword)
	if request.Keyword == "" || utf8.RuneCountInString(request.Keyword) > 200 {
		s.writeAnimekoAdminFailure(w, "关键词长度应为 1 到 200 个字符")
		return
	}
	data, err := s.animeko.repository.CreateAnimekoKeyword(r.Context(), request.PoolID, request.Keyword)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) deleteAnimekoKeyword(w http.ResponseWriter, r *http.Request, path string) {
	id := pathID(path, "/api/admin/animeko/keywords/", "")
	ok, err := s.animeko.repository.DeleteAnimekoKeyword(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if !ok {
		s.writeAnimekoAdminFailure(w, "关键词不存在")
		return
	}
	s.writeJSON(w, http.StatusOK, success(nil))
}

func (s *Server) writeAnimekoAdminFailure(w http.ResponseWriter, message string) {
	s.writeJSON(w, http.StatusOK, result{Code: 1, Data: map[string]string{"desc": message}})
}
