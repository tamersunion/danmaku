package api

import (
	"context"
	"encoding/json"
	"errors"
	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

func tencentChapterOrder(title string) int {
	year := 0
	for n := 0; n+4 <= len(title); n++ {
		value, err := strconv.Atoi(title[n : n+4])
		if err == nil && value >= 1900 && value < 2100 {
			year = value
			break
		}
	}
	season := 0
	for index, name := range []string{"春", "夏", "秋", "冬"} {
		if strings.Contains(title, name) {
			season = index + 1
			break
		}
	}
	return year*10 + season
}

func tencentOK(data map[string]any) error {
	if data["ret"] == nil || numberValue(data["ret"]) != 0 || object(data["data"]) == nil {
		return errors.New("tencent upstream returned an error")
	}
	return nil
}
func (i *Catalog) searchTencent(ctx context.Context, keyword string) ([]sourceAnime, error) {
	payload := map[string]any{"version": "25071701", "clientType": 1, "filterValue": "", "uuid": "0379274D-05A0-4EB6-A89C-878C9A460426", "query": keyword, "retry": 0, "pagenum": 0, "isPrefetch": true, "pagesize": 30, "queryFrom": 0, "searchDatakey": "", "transInfo": "", "isneedQc": true, "preQid": "", "adClientInfo": "", "extraInfo": map[string]string{"multi_terminal_pc": "1", "themeType": "1", "sugRelatedIds": "{}", "appVersion": ""}}
	data, err := i.getJSON(ctx, catalogURL(i.settings.SearchAPIBase, url.Values{"vplatform": {"2"}}), payload)
	if err != nil {
		return nil, err
	}
	if err = tencentOK(data); err != nil {
		return nil, err
	}
	boxes := array(valueAt(data, "data", "areaBoxList"))
	var items []any
	for _, box := range boxes {
		if textValue(object(box)["boxId"]) == "MainNeed" {
			items = array(object(box)["itemList"])
			break
		}
	}
	if len(items) == 0 {
		items = array(valueAt(data, "data", "normalList", "itemList"))
	}
	for _, box := range boxes {
		m := object(box)
		titles := array(valueAt(m, "boxTitle", "boxTitles"))
		related := false
		for _, title := range titles {
			if textValue(title) == "相关影视" {
				related = true
			}
		}
		if related {
			for _, item := range array(m["itemList"]) {
				if strings.Contains(cleanIqiyiTitle(textValue(valueAt(item, "videoInfo", "title"))), keyword) {
					items = append(items, item)
				}
			}
		}
	}
	out := make([]sourceAnime, 0)
	seen := map[string]bool{}
	for _, item := range items {
		m := object(item)
		v := object(m["videoInfo"])
		id := textValue(valueAt(m, "doc", "id"))
		title := cleanIqiyiTitle(textValue(v["title"]))
		kind := textValue(v["typeName"])
		if !catalogIdentifier.MatchString(id) || title == "" || seen[id] || numberValue(v["year"]) == 0 || textValue(v["subTitle"]) == "全网搜" || numberValue(v["playFlag"]) == 2 {
			continue
		}
		switch kind {
		case "电视剧", "动漫", "电影", "纪录片", "综艺", "综艺节目":
		default:
			continue
		}
		sites := append(append([]any{}, array(v["playSites"])...), array(v["episodeSites"])...)
		allowed := len(sites) == 0
		chapters := []string{}
		for _, site := range sites {
			if textValue(object(site)["enName"]) == "qq" {
				allowed = true
				for _, ch := range array(valueAt(site, "chapterInfo", "chapters")) {
					title := textValue(object(ch)["title"])
					if title != "" {
						chapters = append(chapters, title)
					}
				}
			}
		}
		if !allowed {
			continue
		}
		seen[id] = true
		out = append(out, sourceAnime{AnimeID: id, Title: title, TypeDescription: kind, StartDate: textValue(v["year"])})
		if i.cache != nil && len(chapters) > 0 {
			raw, _ := json.Marshal(chapters)
			_, _ = i.cache.Cache(ctx, "tencent-chapters:"+i.settings.SearchAPIBase+":"+id, 0, func(context.Context) ([]byte, error) { return raw, nil })
		}
	}
	return out, nil
}
func (i *Catalog) tencentEpisodePage(ctx context.Context, id, pc string) (map[string]any, error) {
	payload := map[string]any{"has_cache": 1, "page_params": map[string]string{"req_from": "web_vsite", "page_id": "vsite_episode_list", "page_type": "detail_operation", "id_type": "1", "page_size": "", "cid": id, "vid": "", "lid": "", "page_num": "", "page_context": pc, "detail_page_type": "1"}}
	data, err := i.getJSON(ctx, catalogURL(i.settings.EpisodesAPIBase, url.Values{"video_appid": {"3000010"}, "vversion_name": {"8.2.96"}, "vversion_platform": {"2"}}), payload)
	if err != nil {
		return nil, err
	}
	return data, tencentOK(data)
}
func tencentEpisodeItems(data map[string]any) ([]sourceEpisode, []string, bool) {
	out := []sourceEpisode{}
	tabs := []string{}
	hasNext := false
	for _, list := range array(valueAt(data, "data", "module_list_datas")) {
		for _, mod := range array(object(list)["module_datas"]) {
			params := object(object(mod)["module_params"])
			if params["has_next"] == true || textValue(params["has_next"]) == "true" {
				hasNext = true
			}
			var parsed []map[string]any
			if raw := textValue(params["tabs"]); raw != "" {
				_ = json.Unmarshal([]byte(raw), &parsed)
			}
			for _, tab := range parsed {
				pc := textValue(tab["page_context"])
				if pc != "" {
					q, _ := url.ParseQuery(pc)
					ch := q.Get("chapter_name")
					if ch == "" || ch == "正片" {
						tabs = append(tabs, pc)
					}
				}
			}
			for _, item := range array(valueAt(mod, "item_data_lists", "item_datas")) {
				v := object(object(item)["item_params"])
				id := textValue(v["vid"])
				if !catalogIdentifier.MatchString(id) || textValue(v["is_trailer"]) == "1" {
					continue
				}
				title := textValue(v["union_title"])
				if title == "" {
					title = textValue(v["title"])
				}
				out = append(out, sourceEpisode{EpisodeID: id, Title: title, Number: textValue(v["episode_on_chapter"])})
			}
		}
	}
	return out, tabs, hasNext
}
func (i *Catalog) episodesTencent(ctx context.Context, id string) ([]sourceEpisode, error) {
	data, err := i.tencentEpisodePage(ctx, id, "cid="+id+"&detail_page_type=1&req_from=web_vsite&req_from_second_type=&req_type=0")
	if err != nil {
		return nil, err
	}
	initial, tabs, _ := tencentEpisodeItems(data)
	result := []sourceEpisode{}
	seen := map[string]bool{}
	appendItems := func(items []sourceEpisode) {
		for _, item := range items {
			if !seen[item.EpisodeID] {
				seen[item.EpisodeID] = true
				result = append(result, item)
			}
		}
	}
	if len(tabs) > 100 {
		return nil, errors.New("tencent episode pages exceed limit")
	}
	if len(tabs) > 0 {
		for _, pc := range tabs {
			data, err := i.tencentEpisodePage(ctx, id, pc)
			if err != nil {
				return nil, err
			}
			items, _, _ := tencentEpisodeItems(data)
			appendItems(items)
		}
	} else {
		var chapters []string
		if i.cache != nil {
			raw, _ := i.cache.Cache(ctx, "tencent-chapters:"+i.settings.SearchAPIBase+":"+id, time.Hour, func(context.Context) ([]byte, error) {
				return nil, errors.New("chapter metadata unavailable; search the series first")
			})
			_ = json.Unmarshal(raw, &chapters)
		}
		if len(chapters) > 50 {
			return nil, errors.New("tencent chapter count exceeds limit")
		}
		for _, ch := range chapters {
			if ch == "正片" {
				chapters = []string{"正片"}
				break
			}
		}
		uniqueChapters := make([]string, 0, len(chapters))
		seenChapters := map[string]bool{}
		for _, chapter := range chapters {
			if !seenChapters[chapter] {
				seenChapters[chapter] = true
				uniqueChapters = append(uniqueChapters, chapter)
			}
		}
		chapters = uniqueChapters
		sort.SliceStable(chapters, func(a, b int) bool { return tencentChapterOrder(chapters[a]) < tencentChapterOrder(chapters[b]) })
		if len(chapters) == 0 {
			appendItems(initial)
		}
		for _, ch := range chapters {
			season := []sourceEpisode{}
			localSeen := map[string]bool{}
			for page := 0; page < 100; page++ {
				q := url.Values{"lid": {""}, "cid": {id}, "page_num": {strconv.Itoa(page)}, "page_size": {"30"}, "id_type": {"1"}, "req_type": {"6"}, "req_from": {"web_vsite"}, "req_from_second_type": {""}, "detail_page_type": {"1"}, "year": {""}, "tab_type": {"4"}, "chapter_name": {ch}, "is_nocopyright": {"false"}}
				data, err := i.tencentEpisodePage(ctx, id, q.Encode())
				if err != nil {
					return nil, err
				}
				items, _, next := tencentEpisodeItems(data)
				added := 0
				for _, item := range items {
					if !localSeen[item.EpisodeID] {
						localSeen[item.EpisodeID] = true
						season = append(season, item)
						added++
					}
				}
				if !next || added == 0 {
					break
				}
				if page == 99 {
					return nil, errors.New("tencent chapter pages exceed limit")
				}
			}
			sort.SliceStable(season, func(a, b int) bool { return numberValue(season[a].Number) < numberValue(season[b].Number) })
			appendItems(season)
		}
	}
	return result, nil
}
func (i *Catalog) fetchTencent(ctx context.Context, id string) ([]domain.DanmakuData, error) {
	base := strings.TrimRight(i.settings.APIBase, "/")
	data, err := i.getJSON(ctx, base+"/base/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, err
	}
	index, ok := data["segment_index"].(map[string]any)
	if !ok {
		return nil, errors.New("tencent response missing segment index")
	}
	type segment struct {
		name  string
		start float64
	}
	segments := []segment{}
	seen := map[string]bool{}
	for _, raw := range index {
		m := object(raw)
		name := textValue(m["segment_name"])
		if name == "" || strings.Contains(name, "..") || strings.ContainsAny(name, "?#:\\") {
			return nil, errors.New("invalid tencent segment name")
		}
		valid := true
		for _, part := range strings.Split(name, "/") {
			if !catalogIdentifier.MatchString(part) {
				valid = false
			}
		}
		if !valid {
			return nil, errors.New("invalid tencent segment name")
		}
		if !seen[name] {
			seen[name] = true
			segments = append(segments, segment{name, numberValue(m["segment_start"])})
		}
	}
	sort.Slice(segments, func(a, b int) bool { return segments[a].start < segments[b].start })
	return fetchCatalogSegments(ctx, len(segments), func(ctx context.Context, n int) ([]domain.DanmakuData, error) {
		data, err := i.getJSON(ctx, base+"/segment/"+url.PathEscape(id)+"/"+segments[n].name, nil)
		if err != nil {
			return nil, err
		}
		rows, ok := data["barrage_list"].([]any)
		if !ok {
			return nil, errors.New("tencent segment missing barrage list")
		}
		return parseTencentComments(rows)
	})
}
func parseTencentComments(rows []any) ([]domain.DanmakuData, error) {
	out := make([]domain.DanmakuData, 0, len(rows))
	for _, raw := range rows {
		m := object(raw)
		if m["time_offset"] == nil {
			continue
		}
		style := object(m["content_style"])
		if style == nil {
			_ = json.Unmarshal([]byte(textValue(m["content_style"])), &style)
		}
		mode := 1
		switch int(numberValue(style["position"])) {
		case 2:
			mode = 5
		case 3:
			mode = 4
		}
		color := catalogColor(style["color"], true)
		if colors := array(style["gradient_colors"]); len(colors) > 0 {
			color = catalogColor(colors[0], true)
		}
		if v, ok := catalogComment(numberValue(m["time_offset"])/1000, textValue(m["content"]), mode, color); ok {
			out = append(out, v)
		}
	}
	if len(rows) > 0 && len(out) == 0 {
		return nil, errors.New("tencent returned only invalid comments")
	}
	return out, nil
}
