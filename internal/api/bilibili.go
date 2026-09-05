package api

import (
	"bytes"
	"compress/flate"
	"compress/zlib"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"git.hanada.info/tamersunion/danmaku/internal/config"
	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"git.hanada.info/tamersunion/danmaku/internal/store"
)

type Bilibili struct {
	repository store.Repository
	settings   config.BilibiliSettings
	client     *http.Client
	baseURL    string
}

type bilibiliQuery struct {
	CID    int64
	AID    int
	BVID   string
	Page   int
	Dates  []string
	Offset float64
}

const bilibiliSyncInterval = 10 * time.Minute

type bilibiliPageResponse struct {
	Code int            `json:"code"`
	Data []bilibiliPage `json:"data"`
}

type bilibiliPage struct {
	CID       int64             `json:"cid"`
	Page      int               `json:"page"`
	From      string            `json:"from"`
	Part      string            `json:"part"`
	Duration  int               `json:"duration"`
	Dimension bilibiliDimension `json:"dimension"`
}

type bilibiliDimension struct {
	Width  int `json:"width"`
	Height int `json:"height"`
	Rotate int `json:"rotate"`
}

type bilibiliArchiveResponse struct {
	Code int `json:"code"`
	Data struct {
		AID  int    `json:"aid"`
		BVID string `json:"bvid"`
	} `json:"data"`
}

func NewBilibili(repository store.Repository, settings config.BilibiliSettings) *Bilibili {
	baseURL := strings.TrimRight(strings.TrimSpace(settings.APIBase), "/")
	if baseURL == "" {
		baseURL = config.DefaultBilibiliAPIBase
	}
	return &Bilibili{repository: repository, settings: settings, client: &http.Client{Timeout: 60 * time.Second}, baseURL: baseURL}
}

func (b *Bilibili) Data(ctx context.Context, query bilibiliQuery) ([]domain.DanmakuData, error) {
	pool, _, err := b.ensurePool(ctx, query, false, true)
	if err != nil || pool == nil {
		return []domain.DanmakuData{}, err
	}
	data, err := b.repository.BilibiliPoolData(ctx, pool.ID)
	if err != nil {
		return nil, err
	}
	return offsetDanmaku(data, query.Offset), nil
}

func (b *Bilibili) fetchDanmaku(ctx context.Context, query bilibiliQuery, cid int64) ([]domain.DanmakuData, error) {
	if len(query.Dates) == 0 || b.settings.Cookie == "" {
		raw, err := b.fetch(ctx, fmt.Sprintf("%s/x/v1/dm/list.so?oid=%d", b.baseURL, cid), false, 0)
		if err != nil {
			return nil, err
		}
		return parseBilibiliXML(raw)
	}
	results := make([][]domain.DanmakuData, len(query.Dates))
	errorsByIndex := make([]error, len(query.Dates))
	var wait sync.WaitGroup
	for index, date := range query.Dates {
		index, date := index, date
		wait.Add(1)
		go func() {
			defer wait.Done()
			raw, fetchErr := b.fetch(ctx, fmt.Sprintf("%s/x/v2/dm/history?type=1&oid=%d&date=%s", b.baseURL, cid, url.QueryEscape(date)), true, 0)
			if fetchErr != nil {
				errorsByIndex[index] = fetchErr
				return
			}
			results[index], errorsByIndex[index] = parseBilibiliXML(raw)
		}()
	}
	wait.Wait()
	combined := make([]domain.DanmakuData, 0)
	for index := range results {
		if errorsByIndex[index] != nil {
			return nil, errorsByIndex[index]
		}
		combined = append(combined, results[index]...)
	}
	return combined, nil
}

func (b *Bilibili) ensurePool(ctx context.Context, query bilibiliQuery, force, staleOnError bool) (*domain.BilibiliPool, int, error) {
	bvid, page, cid, err := b.resolvePool(ctx, query)
	if err != nil || cid == 0 {
		return nil, 0, err
	}
	pool, err := b.repository.EnsureBilibiliPool(ctx, bvid, page, cid)
	if err != nil {
		return nil, 0, err
	}
	claimed, err := b.repository.ClaimBilibiliPoolSync(ctx, pool.ID, bilibiliSyncInterval, force)
	if err != nil || !claimed {
		return pool, 0, err
	}
	data, err := b.fetchDanmaku(ctx, query, cid)
	if err != nil {
		if staleOnError {
			return pool, 0, nil
		}
		return pool, 0, err
	}
	inserted, err := b.repository.MergeBilibiliDanmaku(ctx, pool.ID, data)
	if err != nil {
		return pool, 0, err
	}
	refreshed, err := b.repository.BilibiliPool(ctx, pool.ID)
	return refreshed, inserted, err
}

func (b *Bilibili) PreparePool(ctx context.Context, query bilibiliQuery) (*domain.BilibiliPool, int, error) {
	query.BVID = strings.TrimSpace(query.BVID)
	return b.ensurePool(ctx, query, true, false)
}

func (b *Bilibili) SyncPool(ctx context.Context, id int) (*domain.BilibiliPool, int, error) {
	pool, err := b.repository.BilibiliPool(ctx, id)
	if err != nil {
		return nil, 0, err
	}
	return b.ensurePool(ctx, bilibiliQuery{BVID: pool.BVID, Page: pool.Page, CID: pool.CID}, true, false)
}

func (b *Bilibili) BoundData(ctx context.Context, vid string) ([]domain.DanmakuData, error) {
	bindings, err := b.repository.BilibiliBindingsByVID(ctx, vid)
	if err != nil {
		return nil, err
	}
	result := make([]domain.DanmakuData, 0)
	for _, binding := range bindings {
		pool, _, err := b.ensurePool(ctx, bilibiliQuery{BVID: binding.BVID, Page: binding.Page, CID: binding.CID}, false, true)
		if err != nil {
			return nil, err
		}
		data, err := b.repository.BilibiliPoolData(ctx, pool.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, offsetDanmaku(data, binding.Offset)...)
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Time < result[j].Time })
	return result, nil
}

func (b *Bilibili) Archive(ctx context.Context, bvid string, aid int) (bilibiliArchiveResponse, error) {
	if bvid != "" && aid != 0 {
		var response bilibiliArchiveResponse
		response.Code, response.Data.AID, response.Data.BVID = 0, aid, bvid
		return response, nil
	}
	query := ""
	if bvid != "" {
		query = "bvid=" + url.QueryEscape(bvid)
	} else {
		query = "aid=" + strconv.Itoa(aid)
	}
	raw, err := b.fetch(ctx, b.baseURL+"/x/web-interface/archive/stat?"+query, false, time.Duration(b.settings.CIDCacheMinutes)*time.Minute)
	if err != nil {
		return bilibiliArchiveResponse{}, err
	}
	var response bilibiliArchiveResponse
	if len(raw) == 0 || json.Unmarshal(raw, &response) != nil || response.Code != 0 {
		return bilibiliArchiveResponse{Code: 1}, nil
	}
	return response, nil
}

func (b *Bilibili) Pages(ctx context.Context, bvid string, aid int) ([]bilibiliPage, error) {
	query := ""
	if bvid != "" {
		query = "bvid=" + url.QueryEscape(bvid)
	} else {
		query = "aid=" + strconv.Itoa(aid)
	}
	raw, err := b.fetch(ctx, b.baseURL+"/x/player/pagelist?"+query, false, time.Duration(b.settings.CIDCacheMinutes)*time.Minute)
	if err != nil {
		return nil, err
	}
	var response bilibiliPageResponse
	if len(raw) == 0 || json.Unmarshal(raw, &response) != nil || response.Code != 0 {
		return nil, nil
	}
	return response.Data, nil
}

func (b *Bilibili) resolvePool(ctx context.Context, query bilibiliQuery) (string, int, int64, error) {
	if query.Page == 0 {
		query.Page = 1
	}
	bvid := strings.TrimSpace(query.BVID)
	if query.CID != 0 {
		return bvid, query.Page, query.CID, nil
	}
	if bvid == "" && query.AID != 0 {
		archive, err := b.Archive(ctx, "", query.AID)
		if err != nil {
			return "", 0, 0, err
		}
		if archive.Code != 0 {
			return "", 0, 0, nil
		}
		bvid = archive.Data.BVID
	}
	if bvid == "" {
		return "", 0, 0, nil
	}
	existing, err := b.repository.BilibiliPoolByKey(ctx, bvid, query.Page)
	if err != nil {
		return "", 0, 0, err
	}
	if existing != nil {
		return existing.BVID, existing.Page, existing.CID, nil
	}
	pages, err := b.Pages(ctx, bvid, 0)
	if err != nil {
		return "", 0, 0, err
	}
	for _, page := range pages {
		if page.Page == query.Page {
			return bvid, query.Page, page.CID, nil
		}
	}
	return "", 0, 0, nil
}

func (b *Bilibili) fetch(ctx context.Context, endpoint string, useCookie bool, lifetime time.Duration) ([]byte, error) {
	sum := md5.Sum([]byte(endpoint))
	key := hex.EncodeToString(sum[:])
	return b.repository.Cache(ctx, key, lifetime, func(ctx context.Context) ([]byte, error) {
		return b.fetchUpstream(ctx, endpoint, useCookie)
	})
}

func (b *Bilibili) fetchUpstream(ctx context.Context, endpoint string, useCookie bool) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if useCookie && b.settings.Cookie != "" {
		request.Header.Set("Cookie", b.settings.Cookie)
	}
	response, err := b.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return []byte{}, nil
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(strings.TrimSpace(response.Header.Get("Content-Encoding")), "deflate") {
		return inflate(raw)
	}
	return raw, nil
}

func inflate(raw []byte) ([]byte, error) {
	reader, err := zlib.NewReader(bytes.NewReader(raw))
	if err != nil {
		reader = flate.NewReader(bytes.NewReader(raw))
	}
	defer reader.Close()
	return io.ReadAll(io.LimitReader(reader, 64<<20))
}

func parseBilibiliXML(raw []byte) ([]domain.DanmakuData, error) {
	if len(raw) == 0 {
		return []domain.DanmakuData{}, nil
	}
	var document xmlDocument
	if err := xml.NewDecoder(bytes.NewReader(raw)).Decode(&document); err != nil {
		return nil, err
	}
	result := make([]domain.DanmakuData, 0, len(document.Items))
	for _, item := range document.Items {
		parts := strings.Split(item.Parameters, ",")
		if len(parts) < 7 {
			continue
		}
		data := domain.NewDanmakuData()
		value, _ := strconv.ParseFloat(parts[0], 32)
		data.Time = float32(value)
		data.Mode, _ = strconv.Atoi(parts[1])
		data.Size, _ = strconv.Atoi(parts[2])
		data.Color, _ = strconv.Atoi(parts[3])
		data.Timestamp, _ = strconv.ParseInt(parts[4], 10, 64)
		data.Pool, _ = strconv.Atoi(parts[5])
		data.Author = parts[6]
		data.Text = stringPointer(item.Text)
		result = append(result, data)
	}
	return result, nil
}

func bilibiliQueryFromRequest(r *http.Request) bilibiliQuery {
	return bilibiliQuery{
		CID: queryInt64(foldQuery(r, "cid")), AID: queryInt(foldQuery(r, "aid"), 0),
		BVID: foldQuery(r, "bvid"), Page: queryInt(foldQuery(r, "p"), 0), Dates: foldQueryValues(r, "date"),
		Offset: queryFloat(foldQuery(r, "offset"), 0),
	}
}

func foldQuery(r *http.Request, key string) string {
	values := foldQueryValues(r, key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func foldQueryValues(r *http.Request, key string) []string {
	for candidate, values := range r.URL.Query() {
		if strings.EqualFold(candidate, key) {
			return values
		}
	}
	return nil
}

func queryInt64(raw string) int64 {
	value, _ := strconv.ParseInt(raw, 10, 64)
	return value
}

func queryFloat(raw string, fallback float64) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > math.MaxFloat32 {
		return fallback
	}
	return value
}

func offsetDanmaku(data []domain.DanmakuData, offset float64) []domain.DanmakuData {
	result := make([]domain.DanmakuData, len(data))
	copy(result, data)
	if offset != 0 {
		for index := range result {
			result[index].Time += float32(offset)
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Time < result[j].Time })
	return result
}

func (s *Server) queryAID(w http.ResponseWriter, r *http.Request) {
	bvid := foldQuery(r, "bvid")
	aid := queryInt(foldQuery(r, "aid"), 0)
	archive, err := s.bilibili.Archive(r.Context(), bvid, aid)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if archive.Code != 0 {
		s.writeJSON(w, http.StatusOK, failure())
		return
	}
	pages, err := s.bilibili.Pages(r.Context(), archive.Data.BVID, 0)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(map[string]any{"aid": archive.Data.AID, "bvid": archive.Data.BVID, "pageList": pages}))
}
