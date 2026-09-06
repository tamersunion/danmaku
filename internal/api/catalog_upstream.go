package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"git.hanada.info/tamersunion/danmaku/internal/domain"
)

var catalogIdentifier = regexp.MustCompile(`^[A-Za-z0-9_=-]{1,128}$`)

func (i *Catalog) normalizeID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if strings.Contains(value, "://") {
		u, e := url.Parse(value)
		if e != nil || u.User != nil {
			return "", errors.New("invalid source URL")
		}
		if u.Scheme != "https" && u.Scheme != "http" {
			return "", errors.New("invalid source URL")
		}
		switch i.source {
		case "bahamut":
			if u.Hostname() != "ani.gamer.com.tw" {
				return "", errors.New("invalid bahamut host")
			}
			value = u.Query().Get("sn")
		case "tencent":
			if u.Hostname() != "v.qq.com" && u.Hostname() != "m.v.qq.com" {
				return "", errors.New("invalid tencent host")
			}
			value = u.Query().Get("vid")
			if value == "" {
				parts := strings.Split(strings.TrimSuffix(u.Path, ".html"), "/")
				value = parts[len(parts)-1]
			}
		case "youku":
			if u.Hostname() != "v.youku.com" && u.Hostname() != "m.youku.com" {
				return "", errors.New("invalid youku host")
			}
			value = u.Query().Get("vid")
			if value == "" {
				value = strings.TrimPrefix(strings.TrimSuffix(u.Path, ".html"), "/v_show/id_")
			}
		}
	}
	if i.source == "bahamut" {
		return normalizeAnimekoEpisodeID(value)
	}
	if !catalogIdentifier.MatchString(value) {
		return "", errors.New("invalid provider video ID")
	}
	return value, nil
}
func catalogRequestID(r *http.Request, source string) string {
	for _, key := range []string{"episodeId", "vid", "videoSn", "sn"} {
		if v := foldQuery(r, key); v != "" {
			return v
		}
	}
	return ""
}
func catalogRoute(path string) string {
	for _, source := range []string{"bahamut", "tencent", "youku"} {
		for _, base := range []string{"/api/danmaku/dplayer/v3/", "/api/danmaku/artplayer/v1/", "/api/danmaku/v1/"} {
			if path == base+source || (base == "/api/danmaku/v1/" && path == base+source+"/xml") {
				return source
			}
		}
	}
	return ""
}
func catalogAdminSource(path string) string {
	for _, source := range []string{"bahamut", "tencent", "youku"} {
		if strings.HasPrefix(path, "/api/admin/"+source+"/") {
			return source
		}
	}
	return ""
}
func catalogURL(base string, q url.Values) string {
	if len(q) == 0 {
		return base
	}
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return base + sep + q.Encode()
}
func (i *Catalog) request(ctx context.Context, method, endpoint string, body []byte, headers http.Header) ([]byte, http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", iqiyiUserAgent)
	req.Header.Set("Accept", "application/json")
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	switch i.source {
	case "bahamut":
		req.Header.Set("User-Agent", "Anime/2.29.2 (7N5749MM3F.tw.com.gamer.anime; build:972; iOS 26.0.0) Alamofire/5.6.4")
	case "tencent":
		req.Header.Set("Origin", "https://v.qq.com")
		req.Header.Set("Referer", "https://v.qq.com/")
	case "youku":
		req.Header.Set("Referer", "https://v.youku.com/")
	}
	if i.settings.Cookie != "" {
		req.Header.Set("Cookie", i.settings.Cookie)
	}
	for k, v := range headers {
		req.Header[k] = v
	}
	res, err := i.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("%s upstream request failed", i.source)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, res.Header, fmt.Errorf("%s upstream HTTP %d", i.source, res.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, (32<<20)+1))
	if err != nil {
		return nil, nil, err
	}
	if len(raw) > 32<<20 {
		return nil, nil, errors.New("upstream response too large")
	}
	return raw, res.Header, nil
}
func decodeCatalogJSON(raw []byte) (map[string]any, error) {
	var data map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&data); err != nil || data == nil {
		return nil, errors.New("invalid upstream JSON")
	}
	return data, nil
}
func (i *Catalog) getJSON(ctx context.Context, endpoint string, body any) (map[string]any, error) {
	method := http.MethodGet
	var raw []byte
	var err error
	if body != nil {
		method = http.MethodPost
		raw, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	raw, _, err = i.request(ctx, method, endpoint, raw, nil)
	if err != nil {
		return nil, err
	}
	return decodeCatalogJSON(raw)
}
func object(v any) map[string]any { m, _ := v.(map[string]any); return m }
func array(v any) []any           { a, _ := v.([]any); return a }
func valueAt(v any, keys ...string) any {
	for _, key := range keys {
		v = object(v)[key]
	}
	return v
}
func textValue(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return string(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		return ""
	}
}
func numberValue(v any) float64 { n, _ := strconv.ParseFloat(textValue(v), 64); return n }
func walkCatalog(v any, depth int, fn func(map[string]any)) {
	if depth > 32 {
		return
	}
	switch x := v.(type) {
	case map[string]any:
		fn(x)
		for _, v := range x {
			walkCatalog(v, depth+1, fn)
		}
	case []any:
		for _, v := range x {
			walkCatalog(v, depth+1, fn)
		}
	}
}
func catalogComment(seconds float64, text string, mode, color int) (domain.DanmakuData, bool) {
	if text == "" || seconds < 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds > 7*24*3600 {
		return domain.DanmakuData{}, false
	}
	return domain.DanmakuData{Time: float32(seconds), Text: &text, Mode: mode, Size: 25, Color: color & 0xffffff}, true
}
func catalogColor(v any, hex bool) int {
	s := textValue(v)
	if s == "" {
		return 0xffffff
	}
	if strings.HasPrefix(s, "#") {
		hex = true
		s = strings.TrimPrefix(s, "#")
	}
	base := 10
	if hex {
		base = 16
	}
	n, e := strconv.ParseUint(s, base, 32)
	if e != nil {
		return 0xffffff
	}
	return int(n) & 0xffffff
}
func (i *Catalog) fetchData(ctx context.Context, id string) ([]domain.DanmakuData, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	switch i.source {
	case "bahamut":
		return i.fetchBahamut(ctx, id)
	case "tencent":
		return i.fetchTencent(ctx, id)
	case "youku":
		return i.fetchYouku(ctx, id)
	}
	return nil, errors.New("unknown catalog source")
}
func (i *Catalog) Search(ctx context.Context, keyword string) ([]sourceAnime, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" || utf8.RuneCountInString(keyword) > 200 {
		return nil, errors.New("请输入 1 到 200 个字符的关键词")
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	switch i.source {
	case "bahamut":
		return i.searchBahamut(ctx, keyword)
	case "tencent":
		return i.searchTencent(ctx, keyword)
	case "youku":
		return i.searchYouku(ctx, keyword)
	}
	return nil, errors.New("unknown catalog source")
}
func (i *Catalog) Episodes(ctx context.Context, id string) ([]sourceEpisode, error) {
	if !catalogIdentifier.MatchString(id) {
		return nil, errors.New("invalid series ID")
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	switch i.source {
	case "bahamut":
		return i.episodesBahamut(ctx, id)
	case "tencent":
		return i.episodesTencent(ctx, id)
	case "youku":
		return i.episodesYouku(ctx, id)
	}
	return nil, errors.New("unknown catalog source")
}

// Each upstream job uses at most four concurrent segment requests. A failed
// segment fails the sync as a whole; it never marks an incomplete fetch fresh.
func fetchCatalogSegments(ctx context.Context, count int, load func(context.Context, int) ([]domain.DanmakuData, error)) ([]domain.DanmakuData, error) {
	if count < 0 || count > 2000 {
		return nil, errors.New("upstream segment count exceeds limit")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	data := make([][]domain.DanmakuData, count)
	jobs := make(chan int)
	var wg sync.WaitGroup
	var once sync.Once
	var first error
	var total atomic.Int64
	for n := 0; n < min(count, 4); n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := range jobs {
				if ctx.Err() != nil {
					continue
				}
				v, e := load(ctx, n)
				if e == nil && total.Add(int64(len(v))) > 1000000 {
					e = errors.New("upstream comment count exceeds limit")
				}
				if e != nil {
					once.Do(func() { first = e; cancel() })
				} else {
					data[n] = v
				}
			}
		}()
	}
	for n := 0; n < count; n++ {
		select {
		case jobs <- n:
		case <-ctx.Done():
		}
	}
	close(jobs)
	wg.Wait()
	if first != nil {
		return nil, first
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	result := make([]domain.DanmakuData, 0)
	for _, v := range data {
		if len(result)+len(v) > 1000000 {
			return nil, errors.New("upstream comment count exceeds limit")
		}
		result = append(result, v...)
	}
	return result, nil
}
