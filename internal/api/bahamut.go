package api

import (
	"context"
	"errors"
	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"github.com/longbridgeapp/opencc"
	"net/url"
	"sort"
	"sync"
)

var bahamutConverter = sync.OnceValues(func() (*opencc.OpenCC, error) { return opencc.New("s2t") })
var bahamutConvertMu sync.Mutex

func (i *Catalog) searchBahamut(ctx context.Context, keyword string) ([]sourceAnime, error) {
	converter, err := bahamutConverter()
	if err != nil {
		return nil, err
	}
	bahamutConvertMu.Lock()
	query, err := converter.Convert(keyword)
	bahamutConvertMu.Unlock()
	if err != nil {
		return nil, err
	}
	data, err := i.getJSON(ctx, catalogURL(i.settings.SearchAPIBase, url.Values{"kw": {query}}), nil)
	if err != nil {
		return nil, err
	}
	items, ok := data["anime"].([]any)
	if !ok {
		items, ok = valueAt(data, "data", "anime").([]any)
	}
	if !ok {
		return nil, errors.New("bahamut search response missing anime")
	}
	out := make([]sourceAnime, 0)
	seen := map[string]bool{}
	for _, raw := range items {
		m := object(raw)
		id := textValue(m["video_sn"])
		title := textValue(m["title"])
		if _, err := normalizeAnimekoEpisodeID(id); err != nil || title == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, sourceAnime{AnimeID: id, Title: title, TypeDescription: textValue(m["info"])})
	}
	return out, nil
}
func (i *Catalog) episodesBahamut(ctx context.Context, id string) ([]sourceEpisode, error) {
	id, err := normalizeAnimekoEpisodeID(id)
	if err != nil {
		return nil, err
	}
	data, err := i.getJSON(ctx, catalogURL(i.settings.EpisodesAPIBase, url.Values{"videoSn": {id}}), nil)
	if err != nil {
		return nil, err
	}
	groups := object(valueAt(data, "data", "anime", "episodes"))
	if groups == nil {
		return nil, errors.New("bahamut episodes response missing episodes")
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]sourceEpisode, 0)
	seen := map[string]bool{}
	for _, key := range keys {
		for _, raw := range array(groups[key]) {
			m := object(raw)
			id := textValue(m["videoSn"])
			if _, err := normalizeAnimekoEpisodeID(id); err != nil || seen[id] {
				continue
			}
			seen[id] = true
			num := textValue(m["episode"])
			out = append(out, sourceEpisode{EpisodeID: id, Title: "第" + num + "集", Number: num})
		}
	}
	return out, nil
}
func (i *Catalog) fetchBahamut(ctx context.Context, id string) ([]domain.DanmakuData, error) {
	data, err := i.getJSON(ctx, catalogURL(i.settings.APIBase, url.Values{"videoSn": {id}, "geo": {"TW,HK"}}), nil)
	if err != nil {
		return nil, err
	}
	rows, ok := valueAt(data, "data", "danmu").([]any)
	if !ok {
		return nil, errors.New("bahamut response missing danmaku")
	}
	result := make([]domain.DanmakuData, 0, len(rows))
	for _, raw := range rows {
		m := object(raw)
		mode := 1
		switch int(numberValue(m["position"])) {
		case 1:
			mode = 5
		case 2:
			mode = 4
		}
		if m["time"] == nil {
			continue
		}
		v, ok := catalogComment(numberValue(m["time"])/10, textValue(m["text"]), mode, catalogColor(m["color"], true))
		if ok {
			v.Author = textValue(m["userid"])
			result = append(result, v)
		}
	}
	if len(rows) > 0 && len(result) == 0 {
		return nil, errors.New("bahamut returned only invalid comments")
	}
	return result, nil
}
