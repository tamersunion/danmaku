package api

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func youkuQuery() url.Values {
	return url.Values{"client_id": {"53e6cc67237fc59a"}, "package": {"com.huawei.hwvplayer.youku"}, "ext": {"show"}}
}
func (i *Catalog) searchYouku(ctx context.Context, keyword string) ([]sourceAnime, error) {
	data, err := i.getJSON(ctx, catalogURL(i.settings.SearchAPIBase, url.Values{"keyword": {keyword}, "userAgent": {iqiyiUserAgent}, "site": {"1"}, "categories": {"0"}, "ftype": {"0"}, "ob": {"0"}, "pg": {"1"}}), nil)
	if err != nil {
		return nil, err
	}
	rows, ok := data["pageComponentList"].([]any)
	if !ok {
		return nil, errors.New("youku search response missing components")
	}
	out := []sourceAnime{}
	seen := map[string]bool{}
	for _, row := range rows {
		m := object(object(row)["commonData"])
		if numberValue(m["isYouku"]) != 1 && numberValue(m["hasYouku"]) != 1 {
			continue
		}
		title := cleanIqiyiTitle(textValue(valueAt(m, "titleDTO", "displayName")))
		id := textValue(m["showId"])
		if title == "" || !catalogIdentifier.MatchString(id) || seen[id] {
			continue
		}
		skip := false
		for _, word := range []string{"抢先看", "非正片", "解读", "揭秘", "赏析"} {
			if strings.Contains(title, word) {
				skip = true
			}
		}
		if skip {
			continue
		}
		seen[id] = true
		out = append(out, sourceAnime{AnimeID: id, Title: title, TypeDescription: textValue(m["cats"])})
	}
	return out, nil
}
func (i *Catalog) episodesYouku(ctx context.Context, id string) ([]sourceEpisode, error) {
	out := []sourceEpisode{}
	seen := map[string]bool{}
	for page := 1; page <= 100; page++ {
		q := youkuQuery()
		q.Set("show_id", id)
		q.Set("page", strconv.Itoa(page))
		q.Set("count", "100")
		data, err := i.getJSON(ctx, catalogURL(i.settings.EpisodesAPIBase, q), nil)
		if err != nil {
			return nil, err
		}
		rows, ok := data["videos"].([]any)
		if !ok {
			return nil, errors.New("youku episodes response missing videos")
		}
		added := 0
		for _, row := range rows {
			m := object(row)
			vid := textValue(m["id"])
			if vid == "" {
				vid = textValue(m["vid"])
			}
			if !catalogIdentifier.MatchString(vid) || seen[vid] {
				continue
			}
			seen[vid] = true
			added++
			out = append(out, sourceEpisode{EpisodeID: vid, Title: textValue(m["title"]), Number: textValue(m["seq"])})
		}
		total := int(numberValue(data["total"]))
		if total > 10000 {
			return nil, errors.New("youku episode count exceeds limit")
		}
		if page*100 >= total || len(rows) == 0 {
			return out, nil
		}
		if added == 0 {
			return nil, errors.New("youku repeated episode page")
		}
	}
	return nil, errors.New("youku episode pages exceed limit")
}
func youkuMD5(value string) string { sum := md5.Sum([]byte(value)); return hex.EncodeToString(sum[:]) }
func youkuSignedRequest(id, cna, token string, segment int, now int64) (url.Values, []byte, error) {
	msg := map[string]any{"ctime": now, "ctype": 10004, "cver": "v1.0", "guid": cna, "mat": segment, "mcount": 1, "pid": 0, "sver": "3.1.0", "type": 1, "vid": id}
	raw, err := json.Marshal(msg)
	if err != nil {
		return nil, nil, err
	}
	encoded := base64.StdEncoding.EncodeToString(raw)
	msg["msg"] = encoded
	msg["sign"] = youkuMD5(encoded + "MkmC9SoIw6xCkSKHhJ7b5D2r51kBiREr")
	raw, err = json.Marshal(msg)
	if err != nil {
		return nil, nil, err
	}
	t := strconv.FormatInt(now, 10)
	q := url.Values{"jsv": {"2.5.6"}, "appKey": {"24679788"}, "t": {t}, "sign": {youkuMD5(strings.SplitN(token, "_", 2)[0] + "&" + t + "&24679788&" + string(raw))}, "api": {"mopen.youku.danmu.list"}, "v": {"1.0"}, "type": {"originaljson"}, "dataType": {"jsonp"}, "timeout": {"20000"}, "jsonpIncPrefix": {"utility"}}
	return q, []byte(url.Values{"data": {string(raw)}}.Encode()), nil
}
func (i *Catalog) fetchYouku(ctx context.Context, id string) ([]domain.DanmakuData, error) {
	q := youkuQuery()
	q.Set("video_id", id)
	info, err := i.getJSON(ctx, catalogURL(i.settings.VideoInfoAPIBase, q), nil)
	if err != nil {
		return nil, err
	}
	duration := numberValue(info["duration"])
	if info["duration"] == nil || duration <= 0 || math.IsNaN(duration) || math.IsInf(duration, 0) || duration > 24*3600 {
		return nil, errors.New("youku video duration unavailable or exceeds limit")
	}
	_, headers, err := i.request(ctx, http.MethodGet, i.settings.CNAAPIBase, nil, http.Header{"Cookie": {""}})
	if err != nil {
		return nil, err
	}
	cna := strings.Trim(headers.Get("Etag"), "\"")
	if cna == "" {
		for _, c := range (&http.Response{Header: headers}).Cookies() {
			if c.Name == "cna" {
				cna = c.Value
			}
		}
	}
	if cna == "" {
		return nil, errors.New("youku session missing cna")
	}
	_, headers, err = i.request(ctx, http.MethodGet, catalogURL(i.settings.SessionAPIBase, url.Values{"jsv": {"2.5.1"}, "appKey": {"24679788"}}), nil, nil)
	if err != nil {
		return nil, err
	}
	token, enc := "", ""
	for _, c := range (&http.Response{Header: headers}).Cookies() {
		switch c.Name {
		case "_m_h5_tk":
			token = c.Value
		case "_m_h5_tk_enc":
			enc = c.Value
		}
	}
	if token == "" || enc == "" {
		return nil, errors.New("youku session token unavailable")
	}
	return fetchCatalogSegments(ctx, int(duration/60)+1, func(ctx context.Context, n int) ([]domain.DanmakuData, error) {
		q, body, err := youkuSignedRequest(id, cna, token, n, time.Now().UnixMilli())
		if err != nil {
			return nil, err
		}
		raw, _, err := i.request(ctx, http.MethodPost, catalogURL(i.settings.APIBase, q), body, http.Header{"Content-Type": {"application/x-www-form-urlencoded"}, "Cookie": {"_m_h5_tk=" + token + "; _m_h5_tk_enc=" + enc + ";"}})
		if err != nil {
			return nil, err
		}
		return parseYoukuResponse(raw)
	})
}
func parseYoukuResponse(raw []byte) ([]domain.DanmakuData, error) {
	data, err := decodeCatalogJSON(raw)
	if err != nil {
		return nil, err
	}
	rawResult := textValue(valueAt(data, "data", "result"))
	if rawResult == "" {
		return nil, errors.New("youku upstream rejected the signed request")
	}
	result, err := decodeCatalogJSON([]byte(rawResult))
	if err != nil {
		return nil, err
	}
	if textValue(result["code"]) == "-1" {
		return nil, errors.New("youku danmaku request failed")
	}
	rows, ok := valueAt(result, "data", "result").([]any)
	if !ok {
		return nil, errors.New("youku response missing comments")
	}
	out := make([]domain.DanmakuData, 0, len(rows))
	for _, row := range rows {
		m := object(row)
		if m["playat"] == nil {
			continue
		}
		var prop map[string]any
		_ = json.Unmarshal([]byte(textValue(m["propertis"])), &prop)
		mode := 1
		switch int(numberValue(prop["pos"])) {
		case 1:
			mode = 5
		case 2:
			mode = 4
		}
		v, ok := catalogComment(numberValue(m["playat"])/1000, textValue(m["content"]), mode, catalogColor(prop["color"], false))
		if ok {
			out = append(out, v)
		}
	}
	if len(rows) > 0 && len(out) == 0 {
		return nil, errors.New("youku returned only invalid comments")
	}
	return out, nil
}
