package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"git.hanada.info/tamersunion/danmaku/internal/config"
	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"git.hanada.info/tamersunion/danmaku/internal/store"
)

func (s *Server) serveAnimekoData(w http.ResponseWriter, r *http.Request, path string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id, err := normalizeAnimekoEpisodeID(foldQuery(r, "episodeId"))
	if err != nil {
		s.writeJSON(w, http.StatusBadRequest, failure())
		return
	}
	data, err := s.animeko.DataWithOffset(r.Context(), id, queryFloat(foldQuery(r, "offset"), 0))
	if err != nil {
		s.writeError(w, err)
		return
	}
	switch {
	case strings.Contains(path, "/dplayer/"):
		s.writeJSON(w, http.StatusOK, success(exportDPlayerRows(data)))
	case strings.Contains(path, "/artplayer/"):
		s.writeJSON(w, http.StatusOK, map[string]any{"danmuku": exportArtPlayerData(data)})
	case strings.HasSuffix(path, "/xml"):
		s.writeXML(w, data)
	default:
		s.writeJSON(w, http.StatusOK, success(data))
	}
}

// Animeko stores only comments returned by the self-hosted Animeko source.
type Animeko struct {
	refresh    *backgroundRefresh
	repository store.AnimekoRepository
	settings   config.AnimekoSettings
	client     *http.Client
}

func NewAnimeko(repository store.Repository, settings config.AnimekoSettings) *Animeko {
	if settings.BangumiAPIBase == "" {
		settings.BangumiAPIBase = config.DefaultAnimekoBangumiAPIBase
	}
	if settings.APIBase == "" {
		settings.APIBase = config.DefaultAnimekoAPIBase
	}
	if settings.SyncIntervalSeconds <= 0 {
		settings.SyncIntervalSeconds = config.DefaultAnimekoSyncIntervalSeconds
	}
	repo, _ := repository.(store.AnimekoRepository)
	return &Animeko{refresh: newBackgroundRefresh(context.Background(), nil), repository: repo, settings: settings, client: &http.Client{Timeout: 60 * time.Second}}
}

func normalizeAnimekoEpisodeID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 19 {
		return "", errors.New("invalid animeko episode ID")
	}
	for _, c := range value {
		if c < '0' || c > '9' {
			return "", errors.New("invalid animeko episode ID")
		}
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return "", errors.New("invalid animeko episode ID")
	}
	return strconv.FormatInt(id, 10), nil
}

func (i *Animeko) DataWithOffset(ctx context.Context, episodeID string, offset float64) ([]domain.DanmakuData, error) {
	episodeID, err := normalizeAnimekoEpisodeID(episodeID)
	if err != nil {
		return nil, err
	}
	if i.repository == nil {
		return nil, errors.New("animeko repository unavailable")
	}
	pool, err := i.repository.EnsureAnimekoPool(ctx, episodeID)
	if err == nil {
		defer i.refreshPool(episodeID)
	}
	if err != nil {
		return nil, err
	}
	data, err := i.repository.AnimekoPoolData(ctx, pool.ID)
	return offsetDanmaku(data, offset), err
}

func (i *Animeko) fetchData(ctx context.Context, episodeID string) ([]domain.DanmakuData, error) {
	raw, err := i.fetch(ctx, http.MethodGet, strings.TrimRight(i.settings.APIBase, "/")+"/v1/danmaku/"+episodeID, nil, 32<<20)
	if err != nil {
		return nil, err
	}
	return parseAnimekoComments(raw)
}

func (i *Animeko) fetch(ctx context.Context, method, endpoint string, body io.Reader, limit int64) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "tamersunion/danmaku (animeko integration)")
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := i.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("animeko upstream HTTP %d", response.StatusCode)
	}
	return readLimited(response.Body, limit)
}

func parseAnimekoComments(raw []byte) ([]domain.DanmakuData, error) {
	var response struct {
		List *[]struct {
			ID       string `json:"id"`
			SenderID string `json:"senderId"`
			Info     *struct {
				PlayTime *float64 `json:"playTime"`
				Color    *int64   `json:"color"`
				Text     *string  `json:"text"`
				Location string   `json:"location"`
			} `json:"danmakuInfo"`
		} `json:"danmakuList"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, fmt.Errorf("decode animeko: %w", err)
	}
	if response.List == nil {
		return nil, errors.New("animeko response missing danmakuList")
	}
	data := make([]domain.DanmakuData, 0, len(*response.List))
	for _, item := range *response.List {
		info := item.Info
		if info == nil || info.PlayTime == nil || info.Color == nil || info.Text == nil || strings.TrimSpace(*info.Text) == "" {
			continue
		}
		seconds := *info.PlayTime / 1000
		if seconds < 0 || seconds > math.MaxFloat32 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
			continue
		}
		// Upstream color is a signed ARGB Int; output keeps only RGB.
		if *info.Color < math.MinInt32 || *info.Color > math.MaxUint32 {
			continue
		}
		mode := 1
		switch info.Location {
		case "NORMAL":
		case "TOP":
			mode = 5
		case "BOTTOM":
			mode = 4
		default:
			continue
		}
		data = append(data, domain.DanmakuData{Time: float32(seconds), Mode: mode, Size: 25, Color: int(uint32(*info.Color) & 0xffffff), Author: item.SenderID, Text: info.Text})
	}
	if len(*response.List) > 0 && len(data) == 0 {
		return nil, errors.New("animeko response contains no valid comments")
	}
	return data, nil
}

func (i *Animeko) ensurePool(ctx context.Context, vid string, force, staleOnError bool) (*domain.AnimekoPool, int, error) {
	var err error
	vid, err = normalizeAnimekoEpisodeID(vid)
	if err != nil {
		return nil, 0, err
	}
	if i.repository == nil {
		return nil, 0, errors.New("animeko storage unavailable")
	}
	pool, err := i.repository.EnsureAnimekoPool(ctx, vid)
	if err != nil {
		return nil, 0, err
	}
	claimed, err := i.repository.ClaimAnimekoPoolSync(ctx, pool.ID, time.Duration(i.settings.SyncIntervalSeconds)*time.Second, force)
	if err != nil || !claimed {
		return pool, 0, err
	}
	data, err := i.fetchData(ctx, vid)
	if err != nil {
		if staleOnError {
			return pool, 0, nil
		}
		return pool, 0, err
	}
	inserted, err := i.repository.MergeAnimekoDanmaku(ctx, pool.ID, data)
	if err != nil {
		return pool, 0, err
	}
	refreshed, err := i.repository.AnimekoPool(ctx, pool.ID)
	return refreshed, inserted, err
}

func (i *Animeko) PreparePool(ctx context.Context, vid string) (*domain.AnimekoPool, int, error) {
	return i.ensurePool(ctx, vid, true, false)
}

func (i *Animeko) SyncPool(ctx context.Context, id int) (*domain.AnimekoPool, int, error) {
	pool, err := i.repository.AnimekoPool(ctx, id)
	if err != nil || pool == nil {
		return pool, 0, err
	}
	return i.ensurePool(ctx, pool.EpisodeID, true, false)
}

func (i *Animeko) refreshPool(episodeID string) {
	i.refresh.schedule("animeko:"+episodeID, func(ctx context.Context) error {
		_, _, err := i.ensurePool(ctx, episodeID, false, false)
		return err
	})
}
