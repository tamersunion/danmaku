package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

type dandanplayAnime struct {
	AnimeID         string `json:"animeId"`
	Title           string `json:"title"`
	TypeDescription string `json:"typeDescription"`
	StartDate       string `json:"startDate"`
}

type dandanplayEpisode struct {
	EpisodeID string `json:"episodeId"`
	Title     string `json:"title"`
	Number    string `json:"number"`
}

var dandanplaySeasonPattern = regexp.MustCompile(`(?i)(?:第\s*[0-9一二三四五六七八九十百千万]+\s*[季期部])|(?:\bS(?:eason)?\s*\d+\b)|(?:\bPart\s*\d+\b)`)

func (i *Dandanplay) Search(ctx context.Context, keyword string) ([]dandanplayAnime, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" || utf8.RuneCountInString(keyword) > 200 {
		return nil, errors.New("invalid search keyword")
	}
	items, err := i.searchAnime(ctx, keyword)
	if err != nil || len(items) > 0 {
		return items, err
	}
	// Match the reference's bounded season-stripping fallback. The user chooses
	// the exact work/season instead of silently matching an episode.
	fallback := strings.TrimSpace(dandanplaySeasonPattern.ReplaceAllString(keyword, ""))
	if fallback != "" && fallback != keyword {
		return i.searchAnime(ctx, fallback)
	}
	return items, nil
}

func (i *Dandanplay) searchAnime(ctx context.Context, keyword string) ([]dandanplayAnime, error) {
	raw, err := i.fetchGateway(ctx, "/v2/search/anime?"+url.Values{"keyword": {keyword}}.Encode(), 4<<20)
	if err != nil {
		return nil, err
	}
	var response struct {
		Success *bool `json:"success"`
		Animes  *[]struct {
			AnimeID         json.RawMessage `json:"animeId"`
			AnimeTitle      string          `json:"animeTitle"`
			TypeDescription string          `json:"typeDescription"`
			StartDate       string          `json:"startDate"`
		} `json:"animes"`
	}
	if err = json.Unmarshal(raw, &response); err != nil {
		return nil, err
	}
	if response.Success != nil && !*response.Success || response.Animes == nil {
		return nil, errors.New("invalid dandanplay search response")
	}
	result := make([]dandanplayAnime, 0, len(*response.Animes))
	seen := map[string]bool{}
	for _, item := range *response.Animes {
		id, err := normalizeDandanplayEpisodeID(jsonScalarString(item.AnimeID))
		if err != nil || item.AnimeTitle == "" || seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, dandanplayAnime{AnimeID: id, Title: item.AnimeTitle, TypeDescription: item.TypeDescription, StartDate: item.StartDate})
	}
	return result, nil
}

func (i *Dandanplay) Episodes(ctx context.Context, animeID string) ([]dandanplayEpisode, error) {
	id, err := normalizeDandanplayEpisodeID(animeID)
	if err != nil {
		return nil, err
	}
	raw, err := i.fetchGateway(ctx, "/v2/bangumi/"+id, 4<<20)
	if err != nil {
		return nil, err
	}
	var response struct {
		Success *bool `json:"success"`
		Bangumi *struct {
			Episodes *[]struct {
				EpisodeID     json.RawMessage `json:"episodeId"`
				EpisodeTitle  string          `json:"episodeTitle"`
				EpisodeNumber json.RawMessage `json:"episodeNumber"`
			} `json:"episodes"`
		} `json:"bangumi"`
	}
	if err = json.Unmarshal(raw, &response); err != nil {
		return nil, err
	}
	if response.Success != nil && !*response.Success || response.Bangumi == nil || response.Bangumi.Episodes == nil {
		return nil, errors.New("invalid dandanplay episodes response")
	}
	result := make([]dandanplayEpisode, 0, len(*response.Bangumi.Episodes))
	seen := map[string]bool{}
	for _, item := range *response.Bangumi.Episodes {
		id, err := normalizeDandanplayEpisodeID(jsonScalarString(item.EpisodeID))
		if err != nil || seen[id] {
			continue
		}
		seen[id] = true
		number := strings.Trim(string(item.EpisodeNumber), "\"")
		if number == "null" {
			number = ""
		}
		if len(item.EpisodeNumber) > 0 && item.EpisodeNumber[0] == '"' {
			_ = json.Unmarshal(item.EpisodeNumber, &number)
		}
		result = append(result, dandanplayEpisode{EpisodeID: id, Title: item.EpisodeTitle, Number: number})
	}
	return result, nil
}

func (s *Server) searchDandanplay(w http.ResponseWriter, r *http.Request) {
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if keyword == "" || utf8.RuneCountInString(keyword) > 200 {
		s.writeDandanplayAdminFailure(w, "请输入 1 到 200 个字符的搜索关键词")
		return
	}
	data, err := s.dandanplay.Search(r.Context(), keyword)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) dandanplayEpisodes(w http.ResponseWriter, r *http.Request, path string) {
	id := strings.TrimSuffix(strings.TrimPrefix(path, "/api/admin/dandanplay/anime/"), "/episodes")
	if _, err := normalizeDandanplayEpisodeID(id); err != nil {
		s.writeDandanplayAdminFailure(w, "作品 ID 无效")
		return
	}
	data, err := s.dandanplay.Episodes(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}
