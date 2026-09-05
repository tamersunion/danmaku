package api

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"git.hanada.info/tamersunion/danmaku/internal/store"
)

func (s *Server) serveVideoAdmin(w http.ResponseWriter, r *http.Request, path string) {
	const base = "/api/admin/videos"
	suffix := strings.Trim(strings.TrimPrefix(path, base), "/")
	if suffix == "" {
		switch r.Method {
		case http.MethodGet:
			s.listVideos(w, r)
		case http.MethodPost:
			s.createVideo(w, r)
		default:
			http.NotFound(w, r)
		}
		return
	}
	parts := strings.Split(suffix, "/")
	videoID, _ := strconv.Atoi(parts[0])
	if videoID < 1 {
		s.writeVideoAdminFailure(w, "视频 ID 无效")
		return
	}
	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		s.getVideo(w, r, videoID)
	case len(parts) == 1 && r.Method == http.MethodPut:
		s.updateVideo(w, r, videoID)
	case len(parts) == 1 && r.Method == http.MethodDelete:
		s.deleteVideo(w, r, videoID)
	case len(parts) == 2 && parts[1] == "status" && r.Method == http.MethodPatch:
		s.setVideoStatus(w, r, videoID)
	case len(parts) == 2 && parts[1] == "bilibili-bindings" && r.Method == http.MethodPost:
		s.upsertVideoBilibiliBinding(w, r, videoID)
	case len(parts) == 3 && parts[1] == "bilibili-bindings" && r.Method == http.MethodDelete:
		bindingID, _ := strconv.Atoi(parts[2])
		s.deleteVideoBilibiliBinding(w, r, videoID, bindingID)
	case len(parts) == 2 && parts[1] == "iqiyi-bindings" && r.Method == http.MethodPost:
		s.upsertVideoIqiyiBinding(w, r, videoID)
	case len(parts) == 3 && parts[1] == "iqiyi-bindings" && r.Method == http.MethodDelete:
		bindingID, _ := strconv.Atoi(parts[2])
		s.deleteVideoIqiyiBinding(w, r, videoID, bindingID)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) listVideos(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	var deleted *bool
	if raw := query.Get("deleted"); raw != "" && raw != "all" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			s.writeVideoAdminFailure(w, "视频状态无效")
			return
		}
		deleted = &value
	}
	data, err := s.repository.Videos(r.Context(), store.VideoFilter{
		Page: queryInt(query.Get("page"), 1), Size: queryInt(query.Get("size"), 20),
		Query: query.Get("query"), IsDeleted: deleted,
	})
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) createVideo(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Vid  string `json:"vid"`
		Name string `json:"name"`
	}
	if !s.decodeJSON(w, r, &request) {
		return
	}
	request.Vid, request.Name = strings.TrimSpace(request.Vid), strings.TrimSpace(request.Name)
	if request.Vid == "" || utf8.RuneCountInString(request.Vid) > 36 || utf8.RuneCountInString(request.Name) > 200 {
		s.writeVideoAdminFailure(w, "视频 ID 长度应为 1 到 36 个字符，名称不能超过 200 个字符")
		return
	}
	data, err := s.repository.CreateVideo(r.Context(), request.Vid, request.Name)
	if errors.Is(err, store.ErrVideoExists) {
		s.writeVideoAdminFailure(w, "该视频 ID 已存在")
		return
	}
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) getVideo(w http.ResponseWriter, r *http.Request, id int) {
	data, err := s.repository.Video(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if data == nil {
		s.writeVideoAdminFailure(w, "视频不存在")
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) updateVideo(w http.ResponseWriter, r *http.Request, id int) {
	var request struct {
		Name string `json:"name"`
	}
	if !s.decodeJSON(w, r, &request) {
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	if utf8.RuneCountInString(request.Name) > 200 {
		s.writeVideoAdminFailure(w, "视频名称不能超过 200 个字符")
		return
	}
	data, err := s.repository.UpdateVideo(r.Context(), id, request.Name)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if data == nil {
		s.writeVideoAdminFailure(w, "视频不存在")
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) deleteVideo(w http.ResponseWriter, r *http.Request, id int) {
	ok, err := s.repository.SetVideoDeleted(r.Context(), id, true)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if !ok {
		s.writeVideoAdminFailure(w, "视频不存在")
		return
	}
	s.writeJSON(w, http.StatusOK, success(nil))
}

func (s *Server) setVideoStatus(w http.ResponseWriter, r *http.Request, id int) {
	var request struct {
		Deleted bool `json:"deleted"`
	}
	if !s.decodeJSON(w, r, &request) {
		return
	}
	ok, err := s.repository.SetVideoDeleted(r.Context(), id, request.Deleted)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if !ok {
		s.writeVideoAdminFailure(w, "视频不存在")
		return
	}
	s.writeJSON(w, http.StatusOK, success(nil))
}

func (s *Server) upsertVideoBilibiliBinding(w http.ResponseWriter, r *http.Request, videoID int) {
	var request struct {
		PoolID int     `json:"poolId"`
		Offset float64 `json:"offset"`
	}
	if !s.decodeJSON(w, r, &request) {
		return
	}
	if request.PoolID < 1 || math.IsNaN(request.Offset) || math.IsInf(request.Offset, 0) || math.Abs(request.Offset) > math.MaxFloat32 {
		s.writeVideoAdminFailure(w, "请输入有效的弹幕池和偏移量")
		return
	}
	data, err := s.repository.UpsertVideoBilibiliBinding(r.Context(), videoID, request.PoolID, request.Offset)
	if errors.Is(err, store.ErrVideoDeleted) {
		s.writeVideoAdminFailure(w, "已删除的视频不能修改弹幕池关联")
		return
	}
	if err != nil {
		s.writeError(w, err)
		return
	}
	if data == nil {
		s.writeVideoAdminFailure(w, "视频不存在")
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) deleteVideoBilibiliBinding(w http.ResponseWriter, r *http.Request, videoID, bindingID int) {
	if bindingID < 1 {
		s.writeVideoAdminFailure(w, "弹幕池关联 ID 无效")
		return
	}
	ok, err := s.repository.DeleteVideoBilibiliBinding(r.Context(), videoID, bindingID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if !ok {
		s.writeVideoAdminFailure(w, "弹幕池关联不存在")
		return
	}
	s.writeJSON(w, http.StatusOK, success(nil))
}

func (s *Server) upsertVideoIqiyiBinding(w http.ResponseWriter, r *http.Request, videoID int) {
	var request struct {
		PoolID int     `json:"poolId"`
		Offset float64 `json:"offset"`
	}
	if !s.decodeJSON(w, r, &request) {
		return
	}
	if request.PoolID < 1 || math.IsNaN(request.Offset) || math.IsInf(request.Offset, 0) || math.Abs(request.Offset) > math.MaxFloat32 {
		s.writeVideoAdminFailure(w, "请输入有效的弹幕池和偏移量")
		return
	}
	data, err := s.repository.UpsertVideoIqiyiBinding(r.Context(), videoID, request.PoolID, request.Offset)
	if errors.Is(err, store.ErrVideoDeleted) {
		s.writeVideoAdminFailure(w, "已删除的视频不能修改弹幕池关联")
		return
	}
	if err != nil {
		s.writeError(w, err)
		return
	}
	if data == nil {
		s.writeVideoAdminFailure(w, "视频不存在")
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) deleteVideoIqiyiBinding(w http.ResponseWriter, r *http.Request, videoID, bindingID int) {
	if bindingID < 1 {
		s.writeVideoAdminFailure(w, "弹幕池关联 ID 无效")
		return
	}
	ok, err := s.repository.DeleteVideoIqiyiBinding(r.Context(), videoID, bindingID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if !ok {
		s.writeVideoAdminFailure(w, "弹幕池关联不存在")
		return
	}
	s.writeJSON(w, http.StatusOK, success(nil))
}

func (s *Server) writeVideoAdminFailure(w http.ResponseWriter, message string) {
	s.writeJSON(w, http.StatusOK, result{Code: 1, Data: map[string]string{"desc": message}})
}
