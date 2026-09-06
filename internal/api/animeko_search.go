package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"
)

type sourceAnime struct {
	AnimeID         string `json:"animeId"`
	Title           string `json:"title"`
	TypeDescription string `json:"typeDescription"`
	StartDate       string `json:"startDate"`
}
type sourceEpisode struct {
	EpisodeID string `json:"episodeId"`
	Title     string `json:"title"`
	Number    string `json:"number"`
}

func (i *Animeko) Search(ctx context.Context, keyword string) ([]sourceAnime, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" || utf8.RuneCountInString(keyword) > 200 {
		return nil, errors.New("invalid search keyword")
	}
	body, _ := json.Marshal(map[string]any{"keyword": keyword, "filter": map[string]any{"type": []int{2}}})
	raw, err := i.fetch(ctx, http.MethodPost, strings.TrimRight(i.settings.BangumiAPIBase, "/")+"/v0/search/subjects?limit=60&offset=0", strings.NewReader(string(body)), 4<<20)
	if err != nil {
		return nil, err
	}
	var response struct {
		Data *[]struct {
			ID     json.RawMessage `json:"id"`
			Name   string          `json:"name"`
			NameCN string          `json:"name_cn"`
			Date   string          `json:"date"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, err
	}
	if response.Data == nil {
		return nil, errors.New("invalid Bangumi search response")
	}
	result := make([]sourceAnime, 0, len(*response.Data))
	seen := map[string]bool{}
	for _, item := range *response.Data {
		id, err := normalizeAnimekoEpisodeID(jsonScalarString(item.ID))
		if err != nil || seen[id] {
			continue
		}
		seen[id] = true
		title := item.NameCN
		if title == "" {
			title = item.Name
		}
		result = append(result, sourceAnime{AnimeID: id, Title: title, StartDate: item.Date, TypeDescription: "动画"})
	}
	return result, nil
}

func (i *Animeko) Episodes(ctx context.Context, subjectID string) ([]sourceEpisode, error) {
	id, err := normalizeAnimekoEpisodeID(subjectID)
	if err != nil {
		return nil, err
	}
	result := make([]sourceEpisode, 0)
	seen := map[string]bool{}
	for offset := 0; offset < 10000; offset += 200 {
		endpoint := fmt.Sprintf("%s/v0/episodes?subject_id=%s&type=0&limit=200&offset=%d", strings.TrimRight(i.settings.BangumiAPIBase, "/"), id, offset)
		raw, err := i.fetch(ctx, http.MethodGet, endpoint, nil, 4<<20)
		if err != nil {
			return nil, err
		}
		var response struct {
			Total int `json:"total"`
			Data  *[]struct {
				ID     json.RawMessage `json:"id"`
				Name   string          `json:"name"`
				NameCN string          `json:"name_cn"`
				Sort   json.RawMessage `json:"sort"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &response); err != nil {
			return nil, err
		}
		if response.Data == nil {
			return nil, errors.New("invalid Bangumi episodes response")
		}
		for _, item := range *response.Data {
			id, err := normalizeAnimekoEpisodeID(jsonScalarString(item.ID))
			if err != nil || seen[id] {
				continue
			}
			seen[id] = true
			title := item.NameCN
			if title == "" {
				title = item.Name
			}
			result = append(result, sourceEpisode{EpisodeID: id, Title: title, Number: jsonScalarString(item.Sort)})
		}
		if len(*response.Data) < 200 || offset+len(*response.Data) >= response.Total {
			return result, nil
		}
	}
	return nil, errors.New("Bangumi episode list exceeds safety limit")
}

func (s *Server) searchAnimeko(w http.ResponseWriter, r *http.Request) {
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if keyword == "" || utf8.RuneCountInString(keyword) > 200 {
		s.writeAnimekoAdminFailure(w, "请输入 1 到 200 个字符的搜索关键词")
		return
	}
	data, err := s.animeko.Search(r.Context(), keyword)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}
func (s *Server) animekoEpisodes(w http.ResponseWriter, r *http.Request, path string) {
	id := strings.TrimSuffix(strings.TrimPrefix(path, "/api/admin/animeko/anime/"), "/episodes")
	if _, err := normalizeAnimekoEpisodeID(id); err != nil {
		s.writeAnimekoAdminFailure(w, "作品 ID 无效")
		return
	}
	data, err := s.animeko.Episodes(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}
