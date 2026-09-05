package api

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"git.hanada.info/tamersunion/danmaku/internal/store"
)

func (s *Server) serveBilibiliAdmin(w http.ResponseWriter, r *http.Request, path string) {
	switch {
	case path == "/api/admin/bilibili/pools" && r.Method == http.MethodGet:
		s.listBilibiliPools(w, r)
	case path == "/api/admin/bilibili/pools" && r.Method == http.MethodPost:
		s.createBilibiliPool(w, r)
	case strings.HasPrefix(path, "/api/admin/bilibili/pools/") && strings.HasSuffix(path, "/sync") && r.Method == http.MethodPost:
		s.syncBilibiliPool(w, r, path)
	case strings.HasPrefix(path, "/api/admin/bilibili/pools/") && strings.HasSuffix(path, "/danmaku") && r.Method == http.MethodGet:
		s.listBilibiliPoolDanmaku(w, r, path)
	case strings.HasPrefix(path, "/api/admin/bilibili/danmaku/") && strings.HasSuffix(path, "/blocked") && r.Method == http.MethodPatch:
		s.blockBilibiliDanmaku(w, r, path)
	case path == "/api/admin/bilibili/keywords" && r.Method == http.MethodGet:
		s.listBilibiliKeywords(w, r)
	case path == "/api/admin/bilibili/keywords" && r.Method == http.MethodPost:
		s.createBilibiliKeyword(w, r)
	case strings.HasPrefix(path, "/api/admin/bilibili/keywords/") && r.Method == http.MethodDelete:
		s.deleteBilibiliKeyword(w, r, path)
	case path == "/api/admin/bilibili/bindings" && r.Method == http.MethodGet:
		s.listBilibiliBindings(w, r)
	case path == "/api/admin/bilibili/bindings" && r.Method == http.MethodPost:
		s.upsertBilibiliBinding(w, r)
	case strings.HasPrefix(path, "/api/admin/bilibili/bindings/") && r.Method == http.MethodDelete:
		s.deleteBilibiliBinding(w, r, path)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) listBilibiliPools(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	data, err := s.repository.BilibiliPools(r.Context(), store.BilibiliPoolFilter{
		Page: queryInt(query.Get("page"), 1), Size: queryInt(query.Get("size"), 20), Query: query.Get("query"),
	})
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) createBilibiliPool(w http.ResponseWriter, r *http.Request) {
	var request struct {
		BVID string `json:"bvid"`
		AID  int    `json:"aid"`
		CID  int64  `json:"cid"`
		Page int    `json:"p"`
	}
	if !s.decodeJSON(w, r, &request) {
		return
	}
	request.BVID = strings.TrimSpace(request.BVID)
	if request.Page == 0 {
		request.Page = 1
	}
	identifierCount := 0
	if request.BVID != "" {
		identifierCount++
	}
	if request.AID != 0 {
		identifierCount++
	}
	if request.CID != 0 {
		identifierCount++
	}
	if identifierCount != 1 || len(request.BVID) > 32 || request.AID < 0 || request.CID < 0 || request.Page < 1 {
		s.writeBilibiliAdminFailure(w, "请仅输入一个有效的 BVID、AID 或 CID，并检查分 P")
		return
	}
	pool, inserted, err := s.bilibili.PreparePool(r.Context(), bilibiliQuery{
		BVID: request.BVID,
		AID:  request.AID,
		CID:  request.CID,
		Page: request.Page,
	})
	if err != nil {
		s.writeError(w, err)
		return
	}
	if pool == nil {
		s.writeBilibiliAdminFailure(w, "未找到对应的 Bilibili 弹幕池")
		return
	}
	s.writeJSON(w, http.StatusOK, success(map[string]any{"pool": pool, "inserted": inserted}))
}

func (s *Server) syncBilibiliPool(w http.ResponseWriter, r *http.Request, path string) {
	id := pathID(path, "/api/admin/bilibili/pools/", "/sync")
	if id < 1 {
		s.writeBilibiliAdminFailure(w, "弹幕池 ID 无效")
		return
	}
	pool, inserted, err := s.bilibili.SyncPool(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(map[string]any{"pool": pool, "inserted": inserted}))
}

func (s *Server) listBilibiliPoolDanmaku(w http.ResponseWriter, r *http.Request, path string) {
	poolID := pathID(path, "/api/admin/bilibili/pools/", "/danmaku")
	if poolID < 1 {
		s.writeBilibiliAdminFailure(w, "弹幕池 ID 无效")
		return
	}
	query := r.URL.Query()
	var blocked *bool
	if raw := query.Get("blocked"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			s.writeBilibiliAdminFailure(w, "屏蔽状态无效")
			return
		}
		blocked = &value
	}
	data, err := s.repository.BilibiliDanmaku(r.Context(), store.BilibiliDanmakuFilter{
		Page: queryInt(query.Get("page"), 1), Size: queryInt(query.Get("size"), 20), PoolID: poolID,
		Query: query.Get("query"), Blocked: blocked,
	})
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) blockBilibiliDanmaku(w http.ResponseWriter, r *http.Request, path string) {
	id := pathID(path, "/api/admin/bilibili/danmaku/", "/blocked")
	if id < 1 {
		s.writeBilibiliAdminFailure(w, "弹幕 ID 无效")
		return
	}
	var request struct {
		Blocked bool `json:"blocked"`
	}
	if !s.decodeJSON(w, r, &request) {
		return
	}
	ok, err := s.repository.SetBilibiliDanmakuBlocked(r.Context(), int64(id), request.Blocked)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if !ok {
		s.writeBilibiliAdminFailure(w, "弹幕不存在")
		return
	}
	s.writeJSON(w, http.StatusOK, success(nil))
}

func (s *Server) listBilibiliKeywords(w http.ResponseWriter, r *http.Request) {
	data, err := s.repository.BilibiliKeywords(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) createBilibiliKeyword(w http.ResponseWriter, r *http.Request) {
	var request struct {
		PoolID  *int   `json:"poolId"`
		Keyword string `json:"keyword"`
	}
	if !s.decodeJSON(w, r, &request) {
		return
	}
	request.Keyword = strings.TrimSpace(request.Keyword)
	if request.Keyword == "" || utf8.RuneCountInString(request.Keyword) > 200 {
		s.writeBilibiliAdminFailure(w, "关键词长度应为 1 到 200 个字符")
		return
	}
	data, err := s.repository.CreateBilibiliKeyword(r.Context(), request.PoolID, request.Keyword)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) deleteBilibiliKeyword(w http.ResponseWriter, r *http.Request, path string) {
	id := pathID(path, "/api/admin/bilibili/keywords/", "")
	ok, err := s.repository.DeleteBilibiliKeyword(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if !ok {
		s.writeBilibiliAdminFailure(w, "关键词不存在")
		return
	}
	s.writeJSON(w, http.StatusOK, success(nil))
}

func (s *Server) listBilibiliBindings(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	data, err := s.repository.BilibiliBindings(r.Context(), store.BilibiliBindingFilter{
		Page: queryInt(query.Get("page"), 1), Size: queryInt(query.Get("size"), 20), Query: query.Get("query"),
	})
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) upsertBilibiliBinding(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Vid    string  `json:"vid"`
		PoolID int     `json:"poolId"`
		Offset float64 `json:"offset"`
	}
	if !s.decodeJSON(w, r, &request) {
		return
	}
	request.Vid = strings.TrimSpace(request.Vid)
	if request.Vid == "" || utf8.RuneCountInString(request.Vid) > 36 || request.PoolID < 1 || math.IsNaN(request.Offset) || math.IsInf(request.Offset, 0) || math.Abs(request.Offset) > math.MaxFloat32 {
		s.writeBilibiliAdminFailure(w, "请输入有效的视频 ID、弹幕池和偏移量")
		return
	}
	data, err := s.repository.UpsertBilibiliBinding(r.Context(), request.Vid, request.PoolID, request.Offset)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) deleteBilibiliBinding(w http.ResponseWriter, r *http.Request, path string) {
	id := pathID(path, "/api/admin/bilibili/bindings/", "")
	ok, err := s.repository.DeleteBilibiliBinding(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if !ok {
		s.writeBilibiliAdminFailure(w, "关联不存在")
		return
	}
	s.writeJSON(w, http.StatusOK, success(nil))
}

func pathID(path, prefix, suffix string) int {
	raw := strings.TrimPrefix(path, prefix)
	if suffix != "" {
		raw = strings.TrimSuffix(raw, suffix)
	}
	value, _ := strconv.Atoi(strings.Trim(raw, "/"))
	return value
}

func (s *Server) writeBilibiliAdminFailure(w http.ResponseWriter, message string) {
	s.writeJSON(w, http.StatusOK, result{Code: 1, Data: map[string]string{"desc": message}})
}
