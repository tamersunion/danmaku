package api

import (
	"errors"
	"git.hanada.info/tamersunion/danmaku/internal/store"
	"math"
	"net/http"
)

func (s *Server) upsertVideoCatalogBinding(w http.ResponseWriter, r *http.Request, videoID int, i *Catalog) {
	if i.repository == nil {
		http.NotFound(w, r)
		return
	}
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
	data, err := i.repository.UpsertVideoCatalogBinding(r.Context(), videoID, request.PoolID, request.Offset)
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

func (s *Server) deleteVideoCatalogBinding(w http.ResponseWriter, r *http.Request, videoID, bindingID int, i *Catalog) {
	if i.repository == nil {
		http.NotFound(w, r)
		return
	}
	if bindingID < 1 {
		s.writeVideoAdminFailure(w, "弹幕池关联 ID 无效")
		return
	}
	ok, err := i.repository.DeleteVideoCatalogBinding(r.Context(), videoID, bindingID)
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
