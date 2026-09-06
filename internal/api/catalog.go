package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"git.hanada.info/tamersunion/danmaku/internal/config"
	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"git.hanada.info/tamersunion/danmaku/internal/store"
)

func (s *Server) serveCatalogData(w http.ResponseWriter, r *http.Request, path string, i *Catalog) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id, err := i.normalizeID(catalogRequestID(r, i.source))
	if err != nil {
		s.writeJSON(w, http.StatusBadRequest, failure())
		return
	}
	data, err := i.DataWithOffset(r.Context(), id, queryFloat(foldQuery(r, "offset"), 0))
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

type Catalog struct {
	refresh    *backgroundRefresh
	repository store.CatalogRepository
	settings   config.CatalogSettings
	source     string
	client     *http.Client
	cache      store.Repository
}

func NewCatalog(repository store.Repository, source string, settings config.CatalogSettings, refresh *backgroundRefresh) *Catalog {
	settings.ApplyDefaults(source)
	if settings.SyncIntervalSeconds <= 0 {
		settings.SyncIntervalSeconds = 600
	}
	var repo store.CatalogRepository
	if factory, ok := repository.(interface {
		Catalog(string) store.CatalogRepository
	}); ok {
		repo = factory.Catalog(source)
	}
	return &Catalog{repository: repo, source: source, settings: settings, refresh: refresh, cache: repository, client: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (i *Catalog) DataWithOffset(ctx context.Context, episodeID string, offset float64) ([]domain.DanmakuData, error) {
	episodeID, err := i.normalizeID(episodeID)
	if err != nil {
		return nil, err
	}
	if i.repository == nil {
		return nil, errors.New("catalog repository unavailable")
	}
	pool, err := i.repository.EnsureCatalogPool(ctx, episodeID)
	if err == nil {
		defer i.refreshPool(episodeID)
	}
	if err != nil {
		return nil, err
	}
	data, err := i.repository.CatalogPoolData(ctx, pool.ID)
	return offsetDanmaku(data, offset), err
}

func (i *Catalog) ensurePool(ctx context.Context, vid string, force, staleOnError bool) (*domain.CatalogPool, int, error) {
	var err error
	vid, err = i.normalizeID(vid)
	if err != nil {
		return nil, 0, err
	}
	if i.repository == nil {
		return nil, 0, errors.New("catalog storage unavailable")
	}
	pool, err := i.repository.EnsureCatalogPool(ctx, vid)
	if err != nil {
		return nil, 0, err
	}
	claimed, err := i.repository.ClaimCatalogPoolSync(ctx, pool.ID, time.Duration(i.settings.SyncIntervalSeconds)*time.Second, force)
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
	inserted, err := i.repository.MergeCatalogDanmaku(ctx, pool.ID, data)
	if err != nil {
		return pool, 0, err
	}
	refreshed, err := i.repository.CatalogPool(ctx, pool.ID)
	return refreshed, inserted, err
}

func (i *Catalog) PreparePool(ctx context.Context, vid string) (*domain.CatalogPool, int, error) {
	return i.ensurePool(ctx, vid, true, false)
}

func (i *Catalog) SyncPool(ctx context.Context, id int) (*domain.CatalogPool, int, error) {
	pool, err := i.repository.CatalogPool(ctx, id)
	if err != nil || pool == nil {
		return pool, 0, err
	}
	return i.ensurePool(ctx, pool.EpisodeID, true, false)
}

func (i *Catalog) refreshPool(episodeID string) {
	i.refresh.schedule(i.source+":"+episodeID, func(ctx context.Context) error {
		_, _, err := i.ensurePool(ctx, episodeID, false, false)
		return err
	})
}
