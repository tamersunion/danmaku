package api

import (
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"git.hanada.info/tamersunion/danmaku/internal/store"
)

func (s *Server) serveCatalogAdmin(w http.ResponseWriter, r *http.Request, path string, i *Catalog) {
	if i.repository == nil {
		http.NotFound(w, r)
		return
	}
	switch {
	case path == "/api/admin/"+i.source+"/search" && r.Method == http.MethodGet:
		s.searchCatalog(w, r, i)
	case strings.HasPrefix(path, "/api/admin/"+i.source+"/anime/") && strings.HasSuffix(path, "/episodes") && r.Method == http.MethodGet:
		s.catalogEpisodes(w, r, path, i)
	case path == "/api/admin/"+i.source+"/pools" && r.Method == http.MethodGet:
		s.listCatalogPools(w, r, i)
	case path == "/api/admin/"+i.source+"/pools" && r.Method == http.MethodPost:
		s.createCatalogPool(w, r, i)
	case strings.HasPrefix(path, "/api/admin/"+i.source+"/pools/") && strings.HasSuffix(path, "/sync") && r.Method == http.MethodPost:
		s.syncCatalogPool(w, r, path, i)
	case strings.HasPrefix(path, "/api/admin/"+i.source+"/pools/") && strings.HasSuffix(path, "/danmaku") && r.Method == http.MethodGet:
		s.listCatalogPoolDanmaku(w, r, path, i)
	case strings.HasPrefix(path, "/api/admin/"+i.source+"/danmaku/") && strings.HasSuffix(path, "/blocked") && r.Method == http.MethodPatch:
		s.blockCatalogDanmaku(w, r, path, i)
	case path == "/api/admin/"+i.source+"/keywords" && r.Method == http.MethodGet:
		s.listCatalogKeywords(w, r, i)
	case path == "/api/admin/"+i.source+"/keywords" && r.Method == http.MethodPost:
		s.createCatalogKeyword(w, r, i)
	case strings.HasPrefix(path, "/api/admin/"+i.source+"/keywords/") && r.Method == http.MethodDelete:
		s.deleteCatalogKeyword(w, r, path, i)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) listCatalogPools(w http.ResponseWriter, r *http.Request, i *Catalog) {
	query := r.URL.Query()
	data, err := i.repository.CatalogPools(r.Context(), store.CatalogPoolFilter{
		Page: queryInt(query.Get("page"), 1), Size: queryInt(query.Get("size"), 20), Query: query.Get("query"),
	})
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) createCatalogPool(w http.ResponseWriter, r *http.Request, i *Catalog) {
	var request struct {
		EpisodeID string `json:"episodeId"`
	}
	if !s.decodeJSON(w, r, &request) {
		return
	}
	request.EpisodeID = strings.TrimSpace(request.EpisodeID)
	if _, err := i.normalizeID(request.EpisodeID); err != nil {
		s.writeCatalogAdminFailure(w, "请输入有效的视频标识或对应平台的视频链接")
		return
	}
	pool, inserted, err := i.PreparePool(r.Context(), request.EpisodeID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if pool == nil {
		s.writeCatalogAdminFailure(w, "未找到对应的第三方弹幕池")
		return
	}
	s.writeJSON(w, http.StatusOK, success(map[string]any{"pool": pool, "inserted": inserted}))
}

func (s *Server) syncCatalogPool(w http.ResponseWriter, r *http.Request, path string, i *Catalog) {
	id := pathID(path, "/api/admin/"+i.source+"/pools/", "/sync")
	if id < 1 {
		s.writeCatalogAdminFailure(w, "弹幕池 ID 无效")
		return
	}
	pool, inserted, err := i.SyncPool(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if pool == nil {
		s.writeCatalogAdminFailure(w, "弹幕池不存在")
		return
	}
	s.writeJSON(w, http.StatusOK, success(map[string]any{"pool": pool, "inserted": inserted}))
}

func (s *Server) listCatalogPoolDanmaku(w http.ResponseWriter, r *http.Request, path string, i *Catalog) {
	poolID := pathID(path, "/api/admin/"+i.source+"/pools/", "/danmaku")
	if poolID < 1 {
		s.writeCatalogAdminFailure(w, "弹幕池 ID 无效")
		return
	}
	query := r.URL.Query()
	var blocked *bool
	if raw := query.Get("blocked"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			s.writeCatalogAdminFailure(w, "屏蔽状态无效")
			return
		}
		blocked = &value
	}
	data, err := i.repository.CatalogDanmaku(r.Context(), store.CatalogDanmakuFilter{
		Page: queryInt(query.Get("page"), 1), Size: queryInt(query.Get("size"), 20), PoolID: poolID,
		Query: query.Get("query"), Blocked: blocked,
	})
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) blockCatalogDanmaku(w http.ResponseWriter, r *http.Request, path string, i *Catalog) {
	id := pathID(path, "/api/admin/"+i.source+"/danmaku/", "/blocked")
	if id < 1 {
		s.writeCatalogAdminFailure(w, "弹幕 ID 无效")
		return
	}
	var request struct {
		Blocked bool `json:"blocked"`
	}
	if !s.decodeJSON(w, r, &request) {
		return
	}
	ok, err := i.repository.SetCatalogDanmakuBlocked(r.Context(), int64(id), request.Blocked)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if !ok {
		s.writeCatalogAdminFailure(w, "弹幕不存在")
		return
	}
	s.writeJSON(w, http.StatusOK, success(nil))
}

func (s *Server) listCatalogKeywords(w http.ResponseWriter, r *http.Request, i *Catalog) {
	data, err := i.repository.CatalogKeywords(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) createCatalogKeyword(w http.ResponseWriter, r *http.Request, i *Catalog) {
	var request struct {
		PoolID  *int   `json:"poolId"`
		Keyword string `json:"keyword"`
	}
	if !s.decodeJSON(w, r, &request) {
		return
	}
	request.Keyword = strings.TrimSpace(request.Keyword)
	if request.Keyword == "" || utf8.RuneCountInString(request.Keyword) > 200 {
		s.writeCatalogAdminFailure(w, "关键词长度应为 1 到 200 个字符")
		return
	}
	data, err := i.repository.CreateCatalogKeyword(r.Context(), request.PoolID, request.Keyword)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) deleteCatalogKeyword(w http.ResponseWriter, r *http.Request, path string, i *Catalog) {
	id := pathID(path, "/api/admin/"+i.source+"/keywords/", "")
	ok, err := i.repository.DeleteCatalogKeyword(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if !ok {
		s.writeCatalogAdminFailure(w, "关键词不存在")
		return
	}
	s.writeJSON(w, http.StatusOK, success(nil))
}

func (s *Server) writeCatalogAdminFailure(w http.ResponseWriter, message string) {
	s.writeJSON(w, http.StatusOK, result{Code: 1, Data: map[string]string{"desc": message}})
}
func (s *Server) searchCatalog(w http.ResponseWriter, r *http.Request, i *Catalog) {
	data, err := i.Search(r.Context(), r.URL.Query().Get("keyword"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}
func (s *Server) catalogEpisodes(w http.ResponseWriter, r *http.Request, path string, i *Catalog) {
	id := strings.TrimSuffix(strings.TrimPrefix(path, "/api/admin/"+i.source+"/anime/"), "/episodes")
	data, err := i.Episodes(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}
