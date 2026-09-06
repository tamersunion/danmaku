package api

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"git.hanada.info/tamersunion/danmaku/internal/domain"
)

var biliReference = regexp.MustCompile(`(?i)^(BV[0-9a-z]{10}|av[0-9]+|ep[0-9]+|ss[0-9]+)$`)
var biliShareURL = regexp.MustCompile(`(?i)(?:https?://|bilibili://|//)?(?:[a-z0-9-]+\.)?(?:bilibili\.com|b23\.tv|bili2233\.cn)/[^\s<>"，。]+|bilibili://[^\s<>"，。]+`)

type bilibiliInput struct {
	Reference    string
	Page         int
	ExplicitPage bool
	ShortURL     string
}
type bilibiliSelection struct {
	Key       string `json:"key"`
	Title     string `json:"title"`
	BVID      string `json:"bvid"`
	AID       int64  `json:"aid"`
	CID       int64  `json:"cid"`
	Page      int    `json:"p"`
	Reference string `json:"reference"`
}

func normalizeBilibiliReference(value string) (string, error) {
	value = domain.CanonicalBVID(value)
	if !biliReference.MatchString(value) {
		return "", errors.New("unsupported bilibili identifier")
	}
	prefix := strings.ToLower(value[:2])
	if prefix == "bv" {
		value = "BV" + value[2:]
		if _, ok := domain.BVIDToAID(value); !ok {
			return "", errors.New("invalid BVID")
		}
		return value, nil
	}
	id, err := strconv.ParseInt(value[2:], 10, 64)
	if err != nil || id <= 0 {
		return "", errors.New("invalid bilibili identifier")
	}
	return prefix + strconv.FormatInt(id, 10), nil
}
func parseBilibiliInput(value string) (bilibiliInput, error) {
	value = strings.TrimSpace(value)
	result := bilibiliInput{Page: 1}
	if len(value) > 4096 {
		return result, errors.New("bilibili input too long")
	}
	if ref, err := normalizeBilibiliReference(value); err == nil {
		result.Reference = ref
		return result, nil
	}
	// Shared titles can surround a link. Only extract when the input is not
	// already a URL, so an arbitrary hostname cannot hide an allowed URL inside.
	if !strings.Contains(value, "://") || strings.ContainsAny(value, " \n\r\t") {
		if match := biliShareURL.FindString(value); match != "" {
			value = strings.TrimRight(match, "）)]}")
		}
	}
	if strings.HasPrefix(value, "//") {
		value = "https:" + value
	} else if !strings.Contains(value, "://") {
		value = "https://" + value
	}
	u, err := url.Parse(value)
	if err != nil || u.User != nil || (u.Port() != "" && u.Port() != "443" && u.Port() != "80") {
		return result, errors.New("invalid bilibili URL")
	}
	if u.Scheme != "https" && u.Scheme != "http" && u.Scheme != "bilibili" {
		return result, errors.New("unsupported bilibili URL scheme")
	}
	host := strings.ToLower(u.Hostname())
	if u.Scheme != "bilibili" {
		switch host {
		case "b23.tv", "www.b23.tv", "bili2233.cn", "www.bili2233.cn":
			result.ShortURL = u.String()
			return result, nil
		case "bilibili.com", "www.bilibili.com", "m.bilibili.com", "player.bilibili.com":
		default:
			return result, errors.New("not a supported bilibili host")
		}
	}
	q := u.Query()
	rawPage := q.Get("p")
	if rawPage == "" {
		rawPage = q.Get("page")
	}
	if raw := rawPage; raw != "" {
		result.Page, err = strconv.Atoi(raw)
		if err != nil || result.Page < 1 {
			return result, errors.New("invalid bilibili part")
		}
		result.ExplicitPage = true
	}
	for _, field := range []struct{ Key, Prefix string }{{"bvid", ""}, {"aid", "av"}, {"avid", "av"}, {"ep_id", "ep"}, {"season_id", "ss"}} {
		if raw := q.Get(field.Key); raw != "" {
			result.Reference, err = normalizeBilibiliReference(field.Prefix + raw)
			return result, err
		}
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if u.Scheme == "bilibili" {
		parts = append([]string{host}, parts...)
	}
	for _, part := range parts {
		if ref, err := normalizeBilibiliReference(part); err == nil {
			result.Reference = ref
			return result, nil
		}
	}
	if u.Scheme == "bilibili" && len(parts) >= 2 {
		prefix := ""
		switch parts[len(parts)-2] {
		case "video":
			prefix = "av"
		case "season":
			prefix = "ss"
		case "episode":
			prefix = "ep"
		}
		if prefix != "" {
			result.Reference, err = normalizeBilibiliReference(prefix + parts[len(parts)-1])
			return result, err
		}
	}
	return result, errors.New("URL contains no AV/BV/EP/SS identifier")
}

func (b *Bilibili) expandBilibiliInput(ctx context.Context, value string) (bilibiliInput, error) {
	for attempt := 0; attempt < 5; attempt++ {
		input, err := parseBilibiliInput(value)
		if err != nil || input.ShortURL == "" {
			return input, err
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, input.ShortURL, nil)
		if err != nil {
			return input, err
		}
		request.Header.Set("User-Agent", iqiyiUserAgent)
		client := *b.client
		client.Timeout = 10 * time.Second
		client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		response, err := client.Do(request)
		if err != nil {
			return input, err
		}
		response.Body.Close()
		if response.StatusCode < 300 || response.StatusCode >= 400 {
			return input, errors.New("bilibili short link returned no redirect")
		}
		location, err := response.Location()
		if err != nil {
			return input, err
		}
		value = location.String()
	}
	return bilibiliInput{}, errors.New("too many bilibili short link redirects")
}
func (b *Bilibili) metadata(ctx context.Context, route string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(b.baseURL, "/")+route, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", iqiyiUserAgent)
	request.Header.Set("Referer", "https://www.bilibili.com/")
	if b.settings.Cookie != "" {
		request.Header.Set("Cookie", b.settings.Cookie)
	}
	response, err := b.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("bilibili metadata HTTP %d", response.StatusCode)
	}
	return readLimited(response.Body, 8<<20)
}

func (b *Bilibili) ResolveInput(ctx context.Context, value string) ([]bilibiliSelection, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	input, err := b.expandBilibiliInput(ctx, value)
	if err != nil {
		return nil, err
	}
	ref := input.Reference
	if strings.HasPrefix(ref, "av") {
		aid, _ := strconv.ParseInt(ref[2:], 10, 64)
		var ok bool
		ref, ok = domain.AIDToBVID(aid)
		if !ok {
			return nil, errors.New("invalid AID")
		}
	}
	if strings.HasPrefix(ref, "BV") {
		raw, err := b.metadata(ctx, "/x/web-interface/view?bvid="+url.QueryEscape(ref))
		if err != nil {
			return nil, err
		}
		var response struct {
			Code int `json:"code"`
			Data *struct {
				BVID  string         `json:"bvid"`
				AID   int64          `json:"aid"`
				Title string         `json:"title"`
				Pages []bilibiliPage `json:"pages"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &response); err != nil {
			return nil, err
		}
		if response.Code != 0 || response.Data == nil {
			return nil, errors.New("bilibili video unavailable")
		}
		data := response.Data
		aid, _ := domain.BVIDToAID(ref)
		result := make([]bilibiliSelection, 0)
		for _, p := range data.Pages {
			if p.CID < 1 || p.Page < 1 || (input.ExplicitPage && p.Page != input.Page) {
				continue
			}
			result = append(result, bilibiliSelection{Key: strconv.FormatInt(p.CID, 10), Title: strings.TrimSpace(data.Title + " · " + p.Part), BVID: ref, AID: aid, CID: p.CID, Page: p.Page, Reference: fmt.Sprintf("https://www.bilibili.com/video/%s?p=%d", ref, p.Page)})
		}
		if len(result) == 0 {
			return nil, errors.New("bilibili video has no matching part")
		}
		return result, nil
	}
	param := "season_id"
	if strings.HasPrefix(ref, "ep") {
		param = "ep_id"
	}
	raw, err := b.metadata(ctx, "/pgc/view/web/season?"+param+"="+ref[2:])
	if err != nil {
		return nil, err
	}
	result, seasonID, err := parseBilibiliSeason(raw, ref)
	if err == nil && len(result) > 0 {
		return result, nil
	}
	if seasonID == 0 && strings.HasPrefix(ref, "ss") {
		seasonID, _ = strconv.ParseInt(ref[2:], 10, 64)
	}
	if seasonID > 0 {
		raw, fetchErr := b.metadata(ctx, fmt.Sprintf("/pgc/web/season/section?season_id=%d", seasonID))
		if fetchErr != nil {
			return nil, fetchErr
		}
		result, _, err = parseBilibiliSeason(raw, ref)
	}
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, errors.New("bilibili season has no accessible episodes")
	}
	return result, nil
}

func parseBilibiliSeason(raw []byte, ref string) ([]bilibiliSelection, int64, error) {
	type episode struct {
		ID        int64  `json:"id"`
		AID       int64  `json:"aid"`
		CID       int64  `json:"cid"`
		BVID      string `json:"bvid"`
		Title     string `json:"title"`
		LongTitle string `json:"long_title"`
		ShowTitle string `json:"show_title"`
	}
	type section struct {
		Episodes []episode `json:"episodes"`
	}
	var response struct {
		Code   int `json:"code"`
		Result *struct {
			SeasonID int64     `json:"season_id"`
			Episodes []episode `json:"episodes"`
			Main     section   `json:"main_section"`
			Sections []section `json:"section"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, 0, err
	}
	if response.Code != 0 || response.Result == nil {
		return nil, 0, errors.New("bilibili season unavailable")
	}
	source := response.Result
	episodes := append(source.Episodes, source.Main.Episodes...)
	for _, section := range source.Sections {
		episodes = append(episodes, section.Episodes...)
	}
	result := make([]bilibiliSelection, 0)
	seen := map[int64]bool{}
	for _, ep := range episodes {
		if ep.CID < 1 || seen[ep.CID] || (strings.HasPrefix(ref, "ep") && strconv.FormatInt(ep.ID, 10) != ref[2:]) {
			continue
		}
		bvid := ep.BVID
		if bvid == "" {
			bvid, _ = domain.AIDToBVID(ep.AID)
		}
		aid, ok := domain.BVIDToAID(bvid)
		if !ok {
			continue
		}
		title := ep.ShowTitle
		if title == "" {
			title = strings.TrimSpace(ep.Title + " " + ep.LongTitle)
		}
		seen[ep.CID] = true
		result = append(result, bilibiliSelection{Key: strconv.FormatInt(ep.CID, 10), Title: cleanIqiyiTitle(title), BVID: bvid, AID: aid, CID: ep.CID, Page: 1, Reference: "ep" + strconv.FormatInt(ep.ID, 10)})
	}
	return result, source.SeasonID, nil
}

func biliWBIQuery(q url.Values, key string, now int64) string {
	q.Set("wts", strconv.FormatInt(now, 10))
	for name, values := range q {
		for n, v := range values {
			values[n] = strings.Map(func(r rune) rune {
				if strings.ContainsRune("!'()*", r) {
					return -1
				}
				return r
			}, v)
		}
		q[name] = values
	}
	encoded := strings.ReplaceAll(q.Encode(), "+", "%20")
	sum := md5.Sum([]byte(encoded + key))
	return encoded + "&w_rid=" + hex.EncodeToString(sum[:])
}
func (b *Bilibili) Search(ctx context.Context, keyword, kind string) ([]sourceAnime, error) {
	if kind != "video" && kind != "media_bangumi" && kind != "media_ft" {
		return nil, errors.New("invalid bilibili search type")
	}
	keyword = strings.TrimSpace(keyword)
	if keyword == "" || utf8.RuneCountInString(keyword) > 200 {
		return nil, errors.New("invalid search keyword")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	raw, err := b.repository.Cache(ctx, "bilibili-wbi:"+b.baseURL, time.Hour, func(ctx context.Context) ([]byte, error) { return b.metadata(ctx, "/x/web-interface/nav") })
	if err != nil {
		return nil, err
	}
	var nav struct {
		Data struct {
			Image struct {
				ImageURL string `json:"img_url"`
				SubURL   string `json:"sub_url"`
			} `json:"wbi_img"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &nav); err != nil {
		return nil, err
	}
	stem := func(value string) string {
		u, err := url.Parse(value)
		if err != nil {
			return ""
		}
		return strings.TrimSuffix(path.Base(u.Path), path.Ext(u.Path))
	}
	combined := stem(nav.Data.Image.ImageURL) + stem(nav.Data.Image.SubURL)
	if len(combined) != 64 {
		return nil, errors.New("bilibili WBI key unavailable")
	}
	table := []int{46, 47, 18, 2, 53, 8, 23, 32, 15, 50, 10, 31, 58, 3, 45, 35, 27, 43, 5, 49, 33, 9, 42, 19, 29, 28, 14, 39, 12, 38, 41, 13}
	var key strings.Builder
	for _, n := range table {
		key.WriteByte(combined[n])
	}
	signed := biliWBIQuery(url.Values{"keyword": {keyword}, "search_type": {kind}, "page": {"1"}, "page_size": {"20"}}, key.String(), time.Now().Unix())
	raw, err = b.metadata(ctx, "/x/web-interface/wbi/search/type?"+signed)
	if err != nil {
		return nil, err
	}
	var response struct {
		Code int `json:"code"`
		Data *struct {
			NumResults *int `json:"numResults"`
			Result     *[]struct {
				BVID     string `json:"bvid"`
				SeasonID int64  `json:"season_id"`
				Title    string `json:"title"`
				Type     string `json:"season_type_name"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, err
	}
	if response.Code != 0 || response.Data == nil {
		return nil, errors.New("bilibili search unavailable or rate limited")
	}
	if response.Data.Result == nil {
		if response.Data.NumResults != nil && *response.Data.NumResults == 0 {
			return []sourceAnime{}, nil
		}
		return nil, errors.New("bilibili search result missing")
	}
	result := make([]sourceAnime, 0)
	seen := map[string]bool{}
	for _, item := range *response.Data.Result {
		id := item.BVID
		if item.SeasonID > 0 {
			id = "ss" + strconv.FormatInt(item.SeasonID, 10)
		}
		if _, err := normalizeBilibiliReference(id); err != nil || seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, sourceAnime{AnimeID: id, Title: cleanIqiyiTitle(item.Title), TypeDescription: item.Type})
	}
	return result, nil
}

func (s *Server) searchBilibili(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("type")
	if kind == "" {
		kind = "video"
	}
	data, err := s.bilibili.Search(r.Context(), r.URL.Query().Get("keyword"), kind)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}
func (s *Server) resolveBilibili(w http.ResponseWriter, r *http.Request) {
	data, err := s.bilibili.ResolveInput(r.Context(), r.URL.Query().Get("input"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}
