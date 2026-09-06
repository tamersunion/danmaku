package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"git.hanada.info/tamersunion/danmaku/internal/config"
	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"git.hanada.info/tamersunion/danmaku/internal/store"
)

func (s *Server) serveDandanplayData(w http.ResponseWriter, r *http.Request, path string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id, err := normalizeDandanplayEpisodeID(foldQuery(r, "episodeId"))
	if err != nil {
		s.writeJSON(w, http.StatusBadRequest, failure())
		return
	}
	withRelated := true
	if raw := foldQuery(r, "withRelated"); raw != "" {
		withRelated, err = strconv.ParseBool(raw)
		if err != nil {
			s.writeJSON(w, http.StatusBadRequest, failure())
			return
		}
	}
	data, err := s.dandanplay.DataWithOffset(r.Context(), id, queryFloat(foldQuery(r, "offset"), 0), withRelated)
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

// APIBase is a gateway endpoint accepting the official API route in ?path=.
type Dandanplay struct {
	repository store.DandanplayRepository
	settings   config.DandanplaySettings
	client     *http.Client
}

func NewDandanplay(repository store.Repository, settings config.DandanplaySettings) *Dandanplay {
	if settings.APIBase == "" {
		settings.APIBase = config.DefaultDandanplayAPIBase
	}
	if settings.SyncIntervalSeconds <= 0 {
		settings.SyncIntervalSeconds = config.DefaultDandanplaySyncIntervalSeconds
	}
	repo, _ := repository.(store.DandanplayRepository)
	return &Dandanplay{repository: repo, settings: settings, client: &http.Client{Timeout: 60 * time.Second}}
}

func normalizeDandanplayEpisodeID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 19 {
		return "", errors.New("invalid dandanplay episode ID")
	}
	for _, c := range value {
		if c < '0' || c > '9' {
			return "", errors.New("invalid dandanplay episode ID")
		}
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return "", errors.New("invalid dandanplay episode ID")
	}
	return strconv.FormatInt(id, 10), nil
}

func (i *Dandanplay) DataWithOffset(ctx context.Context, episodeID string, offset float64, withRelated bool) ([]domain.DanmakuData, error) {
	pool, _, err := i.ensurePool(ctx, episodeID, false, true, withRelated)
	if err != nil {
		return nil, err
	}
	data, err := i.repository.DandanplayPoolData(ctx, pool.ID)
	return offsetDanmaku(data, offset), err
}

func (i *Dandanplay) fetchData(ctx context.Context, episodeID string, withRelated bool) ([]domain.DanmakuData, error) {
	raw, err := i.fetchGateway(ctx, "/v2/comment/"+episodeID+"?from=0&withRelated="+strconv.FormatBool(withRelated)+"&chConvert=0", 32<<20)
	if err != nil {
		return nil, err
	}
	return parseDandanplayComments(raw)
}

func (i *Dandanplay) fetchGateway(ctx context.Context, path string, limit int64) ([]byte, error) {
	endpoint, err := url.Parse(i.settings.APIBase)
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	query.Set("path", path)
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "danmaku")
	request.Header.Set("Accept", "application/json")
	response, err := i.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("dandanplay upstream HTTP %d", response.StatusCode)
	}
	return readLimited(response.Body, limit)
}

func parseDandanplayComments(raw []byte) ([]domain.DanmakuData, error) {
	var response struct {
		Success  *bool           `json:"success"`
		Comments json.RawMessage `json:"comments"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, fmt.Errorf("decode dandanplay comments: %w", err)
	}
	if response.Success != nil && !*response.Success {
		return nil, errors.New("dandanplay upstream rejected request")
	}
	if len(response.Comments) == 0 || string(response.Comments) == "null" {
		return nil, errors.New("dandanplay upstream missing comments")
	}
	var comments []struct {
		P string `json:"p"`
		M string `json:"m"`
	}
	if err := json.Unmarshal(response.Comments, &comments); err != nil {
		return nil, err
	}
	result := make([]domain.DanmakuData, 0, len(comments))
	for _, comment := range comments {
		fields := strings.Split(comment.P, ",")
		if len(fields) < 4 || comment.M == "" {
			continue
		}
		seconds, err := strconv.ParseFloat(fields[0], 32)
		if err != nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 0 {
			continue
		}
		mode, err := strconv.Atoi(fields[1])
		if err != nil || (mode != 1 && mode != 4 && mode != 5) {
			continue
		}
		color, err := strconv.ParseInt(fields[2], 10, 32)
		if err != nil || color < 0 || color > 0xffffff {
			continue
		}
		item := domain.NewDanmakuData()
		item.Time = float32(seconds)
		item.Mode = mode
		item.Color = int(color)
		item.Author = fields[3]
		item.Timestamp = 0
		item.Text = stringPointer(comment.M)
		result = append(result, item)
	}
	if len(comments) > 0 && len(result) == 0 {
		return nil, errors.New("dandanplay upstream contains no valid comments")
	}
	return result, nil
}

func (i *Dandanplay) ensurePool(ctx context.Context, vid string, force, staleOnError, withRelated bool) (*domain.DandanplayPool, int, error) {
	var err error
	vid, err = normalizeDandanplayEpisodeID(vid)
	if err != nil {
		return nil, 0, err
	}
	if i.repository == nil {
		return nil, 0, errors.New("dandanplay storage unavailable")
	}
	pool, err := i.repository.EnsureDandanplayPool(ctx, vid, withRelated)
	if err != nil {
		return nil, 0, err
	}
	claimed, err := i.repository.ClaimDandanplayPoolSync(ctx, pool.ID, time.Duration(i.settings.SyncIntervalSeconds)*time.Second, force)
	if err != nil || !claimed {
		return pool, 0, err
	}
	data, err := i.fetchData(ctx, vid, withRelated)
	if err != nil {
		if staleOnError {
			return pool, 0, nil
		}
		return pool, 0, err
	}
	inserted, err := i.repository.MergeDandanplayDanmaku(ctx, pool.ID, data)
	if err != nil {
		return pool, 0, err
	}
	refreshed, err := i.repository.DandanplayPool(ctx, pool.ID)
	return refreshed, inserted, err
}

func (i *Dandanplay) PreparePool(ctx context.Context, vid string, withRelated bool) (*domain.DandanplayPool, int, error) {
	return i.ensurePool(ctx, vid, true, false, withRelated)
}

func (i *Dandanplay) SyncPool(ctx context.Context, id int) (*domain.DandanplayPool, int, error) {
	pool, err := i.repository.DandanplayPool(ctx, id)
	if err != nil || pool == nil {
		return pool, 0, err
	}
	return i.ensurePool(ctx, pool.EpisodeID, true, false, pool.WithRelated)
}
