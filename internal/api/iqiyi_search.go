package api

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var iqiyiPageID = regexp.MustCompile(`v_([a-zA-Z0-9]+)\.html`)
var iqiyiTitleTags = regexp.MustCompile(`<[^>]*>`)
var iqiyiMediaID = regexp.MustCompile(`^(movie_)?[a-zA-Z0-9]{1,19}$`)
var iqiyiTVID = regexp.MustCompile(`(?:[?;&]|^)tvid=([0-9]+)`)

func cleanIqiyiTitle(value string) string {
	return html.UnescapeString(iqiyiTitleTags.ReplaceAllString(value, ""))
}

func (i *Iqiyi) searchFetch(ctx context.Context, endpoint string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", iqiyiUserAgent)
	request.Header.Set("Referer", "https://www.iqiyi.com/")
	request.Header.Set("Origin", "https://www.iqiyi.com")
	client := *i.client
	// Episode continuation links may only stay within the configured service or
	// the two upstream metadata hosts, including after redirects.
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || !i.allowedMetadataURL(req.URL) {
			return errors.New("disallowed iqiyi metadata redirect")
		}
		return nil
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("iqiyi metadata HTTP %d", response.StatusCode)
	}
	return readLimited(response.Body, 8<<20)
}

func (i *Iqiyi) allowedMetadataURL(u *url.URL) bool {
	if u.User != nil || u.Fragment != "" {
		return false
	}
	for _, endpoint := range []string{i.settings.SearchAPIBase, i.settings.EpisodesAPIBase} {
		base, err := url.Parse(endpoint)
		if err == nil && u.Scheme == base.Scheme && u.Host == base.Host {
			return true
		}
	}
	return u.Scheme == "https" && (u.Host == "www.iqiyi.com" || u.Host == "mesh.if.iqiyi.com")
}

func (i *Iqiyi) Search(ctx context.Context, keyword string) ([]sourceAnime, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" || utf8.RuneCountInString(keyword) > 200 {
		return nil, errors.New("invalid search keyword")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	endpoint, err := url.Parse(i.settings.SearchAPIBase)
	if err != nil {
		return nil, err
	}
	q := endpoint.Query()
	for k, v := range map[string]string{
		"key": keyword, "current_page": "1", "mode": "1", "source": "input", "suggest": "",
		"pcv": "13.074.22699", "version": "13.074.22699", "pageNum": "1", "pageSize": "25",
		"pu": "", "u": "f6440fc5d919dca1aea12b6aff56e1c7", "scale": "200", "token": "",
		"userVip": "0", "conduit": "", "vipType": "-1", "os": "", "osShortName": "win10",
		"dataType": "", "appMode": "", "ad": `{"lm":3,"azd":1000000000951,"azt":733,"position":"feed"}`, "adExt": `{"r":"2.1.5-ares6-pure"}`,
	} {
		q.Set(k, v)
	}
	endpoint.RawQuery = q.Encode()
	var result []sourceAnime
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			if err := waitIqiyiRetry(ctx); err != nil {
				return nil, err
			}
		}
		raw, fetchErr := i.searchFetch(ctx, endpoint.String())
		err = fetchErr
		if err == nil {
			result, err = parseIqiyiSearch(raw)
		}
		if err == nil {
			return result, nil
		}
	}
	return nil, err
}

func parseIqiyiSearch(raw []byte) ([]sourceAnime, error) {
	type album struct {
		Title       string          `json:"title"`
		Channel     string          `json:"channel"`
		BtnText     string          `json:"btnText"`
		PageURL     string          `json:"pageUrl"`
		QipuID      json.RawMessage `json:"qipuId"`
		PlayQipuID  json.RawMessage `json:"playQipuId"`
		Superscript string          `json:"superscript"`
		Year        struct {
			Value string `json:"value"`
			Name  string `json:"name"`
		} `json:"year"`
	}
	var response struct {
		Code json.RawMessage `json:"code"`
		Data *struct {
			Templates *[]struct {
				Template int     `json:"template"`
				Album    *album  `json:"albumInfo"`
				Intent   []album `json:"intentAlbumInfos"`
			} `json:"templates"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, err
	}
	if jsonScalarString(response.Code) == "-1" || response.Data == nil || response.Data.Templates == nil {
		return nil, errors.New("iqiyi search unavailable or rate limited")
	}
	result := make([]sourceAnime, 0)
	seen := map[string]bool{}
	for _, template := range *response.Data.Templates {
		var albums []album
		if template.Template == 112 {
			albums = template.Intent
		} else if (template.Template == 101 || template.Template == 102 || template.Template == 103) && template.Album != nil {
			albums = []album{*template.Album}
		}
		for _, item := range albums {
			if item.Title == "" || item.BtnText == "外站付费播放" {
				continue
			}
			supported := false
			for _, name := range []string{"电影", "电视剧", "动漫", "综艺", "纪录片"} {
				supported = supported || strings.Contains(item.Channel, name)
			}
			if !supported {
				continue
			}
			id := ""
			if strings.Contains(item.Channel, "电影") {
				id = jsonScalarString(item.QipuID)
				if id == "" {
					id = jsonScalarString(item.PlayQipuID)
				}
				if _, err := normalizeAnimekoEpisodeID(id); err != nil {
					continue
				}
				id = "movie_" + id
			} else if match := iqiyiPageID.FindStringSubmatch(item.PageURL); len(match) == 2 {
				id = match[1]
			}
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			year := item.Year.Value
			if year == "" {
				year = item.Year.Name
			}
			if year == "" {
				year = item.Superscript
			}
			result = append(result, sourceAnime{AnimeID: id, Title: cleanIqiyiTitle(item.Title), TypeDescription: item.Channel, StartDate: year})
		}
	}
	return result, nil
}

func iqiyiEntityID(id string) (string, error) {
	if !iqiyiMediaID.MatchString(id) {
		return "", errors.New("invalid iqiyi media ID")
	}
	movie := strings.HasPrefix(id, "movie_")
	id = strings.TrimPrefix(id, "movie_")
	if movie || strings.Trim(id, "0123456789") == "" {
		number, err := strconv.ParseUint(id, 10, 64)
		if err != nil || number == 0 {
			return "", errors.New("invalid iqiyi numeric media ID")
		}
		return strconv.FormatUint(number, 10), nil
	}
	number, err := strconv.ParseUint(id, 36, 64)
	if err != nil {
		return "", err
	}
	number ^= 0x75706971676c
	if number < 900000 {
		number = 100 * (number + 900000)
	}
	return strconv.FormatUint(number, 10), nil
}
func iqiyiSearchSignature(q url.Values) string {
	keys := make([]string, 0, len(q))
	for k := range q {
		if k != "sign" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	values := make([]string, 0, len(keys))
	for _, k := range keys {
		values = append(values, k+"="+q.Get(k))
	}
	sum := md5.Sum([]byte(strings.Join(values, "&") + "&secret_key=howcuteitis"))
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}
func waitIqiyiRetry(ctx context.Context) error {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (i *Iqiyi) Episodes(ctx context.Context, mediaID string) ([]sourceEpisode, error) {
	entity, err := iqiyiEntityID(mediaID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	endpoint, err := url.Parse(i.settings.EpisodesAPIBase)
	if err != nil {
		return nil, err
	}
	q := endpoint.Query()
	for k, v := range map[string]string{
		"entity_id": entity, "device_id": "qd5fwuaj4hunxxdgzwkcqmefeb3ww5hx", "auth_cookie": "", "user_id": "0",
		"vip_type": "-1", "vip_status": "0", "conduit_id": "", "pcv": "13.082.22866", "app_version": "13.082.22866",
		"ext": "", "app_mode": "standard", "scale": "100", "timestamp": strconv.FormatInt(time.Now().UnixMilli(), 10),
		"src": "pca_tvg", "os": "", "ad_ext": `{"r":"2.2.0-ares6-pure"}`,
	} {
		q.Set(k, v)
	}
	q.Set("sign", iqiyiSearchSignature(q))
	endpoint.RawQuery = q.Encode()
	var response iqiyiEpisodeResponse
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			if err := waitIqiyiRetry(ctx); err != nil {
				return nil, err
			}
		}
		raw, fetchErr := i.searchFetch(ctx, endpoint.String())
		err = fetchErr
		if err == nil {
			err = json.Unmarshal(raw, &response)
		}
		if err == nil && response.Status != nil && *response.Status == 0 && response.Data != nil {
			break
		}
		if err == nil {
			err = errors.New("iqiyi episode metadata unavailable")
		}
	}
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(mediaID, "movie_") {
		id := entity
		for _, link := range []string{response.Data.Base.ShareURL, response.Data.Base.PageURL} {
			if match := iqiyiPageID.FindStringSubmatch(link); len(match) == 2 {
				id = match[1]
				break
			}
		}
		return []sourceEpisode{{EpisodeID: id, Title: "正片", Number: "1"}}, nil
	}
	if response.Data.Template == nil {
		return nil, errors.New("iqiyi episode template missing")
	}
	result := make([]sourceEpisode, 0)
	seen := map[string]bool{}
	urls := map[string]bool{}
	for _, tab := range response.Data.Template.Tabs {
		for _, block := range tab.Blocks {
			if block.Type != "album_episodes" && (block.Type != "video_list" || !strings.Contains(string(block.Tag), "episodes")) {
				continue
			}
			var groups []struct {
				Videos json.RawMessage `json:"videos"`
			}
			if err := json.Unmarshal(block.Data.Groups, &groups); err != nil {
				return nil, fmt.Errorf("decode iqiyi episode groups: %w", err)
			}
			for _, group := range groups {
				videos := group.Videos
				var link string
				if json.Unmarshal(videos, &link) == nil && link != "" {
					if urls[link] {
						continue
					}
					urls[link] = true
					if len(urls) > 20 {
						return nil, errors.New("too many iqiyi episode continuations")
					}
					u, err := url.Parse(link)
					if err != nil || !i.allowedMetadataURL(u) {
						return nil, errors.New("disallowed iqiyi episode continuation")
					}
					videos, err = i.searchFetch(ctx, link)
					if err != nil {
						return nil, err
					}
				}
				collectIqiyiEpisodes(videos, &result, seen, 0)
			}
		}
	}
	sort.SliceStable(result, func(a, b int) bool {
		x, _ := strconv.ParseFloat(result[a].Number, 64)
		y, _ := strconv.ParseFloat(result[b].Number, 64)
		return x < y
	})
	return result, nil
}

type iqiyiEpisodeResponse struct {
	Status *int `json:"status_code"`
	Data   *struct {
		Base struct {
			ShareURL string `json:"share_url"`
			PageURL  string `json:"page_url"`
		} `json:"base_data"`
		Template *struct {
			Tabs []struct {
				Blocks []struct {
					Type string          `json:"bk_type"`
					Tag  json.RawMessage `json:"tag"`
					Data struct {
						Groups json.RawMessage `json:"data"`
					} `json:"data"`
				} `json:"blocks"`
			} `json:"tabs"`
		} `json:"template"`
	} `json:"data"`
}

// New video_list arrays and legacy feature_paged maps share episode leaves.
func collectIqiyiEpisodes(raw json.RawMessage, result *[]sourceEpisode, seen map[string]bool, depth int) {
	if depth > 12 {
		return
	}
	var array []json.RawMessage
	if json.Unmarshal(raw, &array) == nil {
		for _, item := range array {
			collectIqiyiEpisodes(item, result, seen, depth+1)
		}
		return
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil {
		return
	}
	if jsonScalarString(object["content_type"]) == "1" {
		playURL := jsonScalarString(object["play_url"])
		match := iqiyiTVID.FindStringSubmatch(playURL)
		if len(match) == 2 {
			id := match[1]
			if _, err := normalizeAnimekoEpisodeID(id); err == nil && !seen[id] {
				title := jsonScalarString(object["short_display_name"])
				if title == "" {
					title = jsonScalarString(object["title"])
				}
				subtitle := jsonScalarString(object["subtitle"])
				if subtitle != "" && !strings.Contains(title, subtitle) {
					title += " " + subtitle
				}
				seen[id] = true
				*result = append(*result, sourceEpisode{EpisodeID: id, Title: cleanIqiyiTitle(title), Number: jsonScalarString(object["album_order"])})
			}
		}
		return
	}
	keys := make([]string, 0, len(object))
	for k := range object {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		collectIqiyiEpisodes(object[k], result, seen, depth+1)
	}
}

func (s *Server) searchIqiyi(w http.ResponseWriter, r *http.Request) {
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if keyword == "" || utf8.RuneCountInString(keyword) > 200 {
		s.writeIqiyiAdminFailure(w, "请输入 1 到 200 个字符的搜索关键词")
		return
	}
	data, err := s.iqiyi.Search(r.Context(), keyword)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}
func (s *Server) iqiyiEpisodes(w http.ResponseWriter, r *http.Request, path string) {
	id := strings.TrimSuffix(strings.TrimPrefix(path, "/api/admin/iqiyi/anime/"), "/episodes")
	if !iqiyiMediaID.MatchString(id) {
		s.writeIqiyiAdminFailure(w, "作品 ID 无效")
		return
	}
	data, err := s.iqiyi.Episodes(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}
