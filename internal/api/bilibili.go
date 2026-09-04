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
	"net/http"
	"net/url"
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
	CID   int64
	AID   int
	BVID  string
	Page  int
	Dates []string
}

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
	return &Bilibili{repository: repository, settings: settings, client: &http.Client{Timeout: 60 * time.Second}, baseURL: "https://bilibili-api.hanada.info"}
}

func (b *Bilibili) Raw(ctx context.Context, query bilibiliQuery) ([]byte, error) {
	cid, err := b.resolveCID(ctx, query)
	if err != nil || cid == 0 {
		return nil, err
	}
	return b.fetch(ctx, fmt.Sprintf("%s/x/v1/dm/list.so?oid=%d", b.baseURL, cid), false, time.Duration(b.settings.DataCacheMinutes)*time.Minute)
}

func (b *Bilibili) Data(ctx context.Context, query bilibiliQuery) ([]domain.DanmakuData, error) {
	cid, err := b.resolveCID(ctx, query)
	if err != nil || cid == 0 {
		return []domain.DanmakuData{}, err
	}
	if len(query.Dates) == 0 || b.settings.Cookie == "" {
		raw, err := b.fetch(ctx, fmt.Sprintf("%s/x/v1/dm/list.so?oid=%d", b.baseURL, cid), false, time.Duration(b.settings.DataCacheMinutes)*time.Minute)
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
			raw, fetchErr := b.fetch(ctx, fmt.Sprintf("%s/x/v2/dm/history?type=1&oid=%d&date=%s", b.baseURL, cid, url.QueryEscape(date)), true, time.Duration(b.settings.DataCacheMinutes)*time.Minute)
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

func (b *Bilibili) resolveCID(ctx context.Context, query bilibiliQuery) (int64, error) {
	if query.CID != 0 {
		return query.CID, nil
	}
	if query.Page == 0 {
		query.Page = 1
	}
	pages, err := b.Pages(ctx, query.BVID, query.AID)
	if err != nil {
		return 0, err
	}
	for _, page := range pages {
		if page.Page == query.Page {
			return page.CID, nil
		}
	}
	return 0, nil
}

func (b *Bilibili) fetch(ctx context.Context, endpoint string, useCookie bool, lifetime time.Duration) ([]byte, error) {
	sum := md5.Sum([]byte(endpoint))
	key := hex.EncodeToString(sum[:])
	return b.repository.Cache(ctx, key, lifetime, func(ctx context.Context) ([]byte, error) {
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
	})
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
