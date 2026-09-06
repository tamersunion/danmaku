package api

import (
	"context"
	"encoding/json"
	"git.hanada.info/tamersunion/danmaku/internal/config"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestIqiyiSearchCards(t *testing.T) {
	raw := `{"code":"0","data":{"templates":[{"template":112,"intentAlbumInfos":[{"title":"<b>作品</b> &amp; test","channel":"动漫","pageUrl":"https://www.iqiyi.com/v_19rrnq1r2s.html"},{"title":"电影","channel":"电影","qipuId":123}]},{"template":101,"albumInfo":{"title":"duplicate","channel":"动漫","pageUrl":"https://www.iqiyi.com/v_19rrnq1r2s.html"}},{"template":102,"albumInfo":{"title":"external","channel":"动漫","btnText":"外站付费播放","pageUrl":"https://www.iqiyi.com/v_abc.html"}}]}}`
	data, err := parseIqiyiSearch([]byte(raw))
	if err != nil || len(data) != 2 || data[0].Title != "作品 & test" || data[1].AnimeID != "movie_123" {
		t.Fatalf("search=%v err=%v", data, err)
	}
	if _, err := parseIqiyiSearch([]byte(`{"code":"-1"}`)); err == nil {
		t.Fatal("rate limit disguised as empty result")
	}
	if data, err := parseIqiyiSearch([]byte(`{"code":"0","data":{"templates":[]}}`)); err != nil || len(data) != 0 {
		t.Fatal("valid empty rejected")
	}
}

func TestIqiyiEpisodeFormatsAndSigning(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/base" {
			q := r.URL.Query()
			if q.Get("entity_id") != "123" || q.Get("sign") != iqiyiSearchSignature(q) || r.Header.Get("Referer") == "" {
				t.Errorf("invalid signed request %s", r.URL)
			}
			_, _ = w.Write([]byte(`{"status_code":0,"data":{"template":{"tabs":[{"blocks":[{"bk_type":"video_list","tag":["episodes"],"data":{"data":[{"videos":[{"data":[{"content_type":1,"play_url":"qips://ht=0;tvid=102;vid=test;","short_display_name":"second","album_order":2},{"content_type":2,"play_url":"https://www.iqiyi.com/?tvid=999","title":"trailer"}]}]}]}},{"bk_type":"video_list","tag":["recommend"],"data":{"data":{"irrelevant":true}}},{"bk_type":"album_episodes","data":{"data":[{"videos":{"feature_paged":{"1":[{"content_type":1,"play_url":"https://www.iqiyi.com/?tvid=101","title":"first","subtitle":"subtitle","album_order":1},{"content_type":1,"play_url":"https://www.iqiyi.com/?tvid=102","title":"duplicate","album_order":2}]}}}]}}]}]}}}`))
		}
	}))
	defer upstream.Close()
	client := NewIqiyi(&fakeRepository{}, config.IqiyiSettings{EpisodesAPIBase: upstream.URL + "/base"})
	data, err := client.Episodes(context.Background(), "123")
	if err != nil || len(data) != 2 || data[0].EpisodeID != "101" || data[0].Title != "first subtitle" || data[1].EpisodeID != "102" {
		t.Fatalf("episodes=%v err=%v", data, err)
	}
	for _, raw := range []string{"http://127.0.0.1/private", "https://evil.test/path", "file:///tmp/data", "https://www.iqiyi.com@evil.test/path"} {
		u, _ := url.Parse(raw)
		if client.allowedMetadataURL(u) {
			t.Errorf("unsafe continuation %s", raw)
		}
	}
	if _, err := iqiyiEntityID("bad/path"); err == nil {
		t.Fatal("invalid identity accepted")
	}
	entity, err := iqiyiEntityID("19rrnq1r2s")
	if err != nil || entity == "" {
		t.Fatal("base36 conversion failed")
	}
}

func TestIqiyiEpisodeContinuationAndMovie(t *testing.T) {
	var base string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/continuation" {
			_, _ = w.Write([]byte(`{"feature_paged":{"1":[{"content_type":1,"play_url":"https://www.iqiyi.com/?tvid=42","title":"episode","album_order":1}]}}`))
			return
		}
		if r.URL.Query().Get("entity_id") == "99" {
			_, _ = w.Write([]byte(`{"status_code":0,"data":{"base_data":{"share_url":"https://www.iqiyi.com/v_19rrnq1r2s.html"}}}`))
			return
		}
		response := map[string]any{"status_code": 0, "data": map[string]any{"template": map[string]any{"tabs": []any{map[string]any{"blocks": []any{map[string]any{"bk_type": "album_episodes", "data": map[string]any{"data": []any{map[string]any{"videos": base + "/continuation"}}}}}}}}}}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer upstream.Close()
	base = upstream.URL
	client := NewIqiyi(&fakeRepository{}, config.IqiyiSettings{EpisodesAPIBase: base + "/base"})
	data, err := client.Episodes(context.Background(), "123")
	if err != nil || len(data) != 1 || data[0].EpisodeID != "42" {
		t.Fatalf("continuation=%v %v", data, err)
	}
	data, err = client.Episodes(context.Background(), "movie_99")
	if err != nil || len(data) != 1 || data[0].EpisodeID != "19rrnq1r2s" {
		t.Fatalf("movie=%v %v", data, err)
	}
}

func TestIqiyiInvalidNumericMediaID(t *testing.T) {
	for _, id := range []string{"0", "000", "movie_0", "movie_ab", "18446744073709551616"} {
		if _, err := iqiyiEntityID(id); err == nil {
			t.Errorf("accepted invalid media ID %s", id)
		}
	}
}

func TestIqiyiSearchLive(t *testing.T) {
	if os.Getenv("IQIYI_SEARCH_LIVE_TEST") == "" {
		t.Skip("set IQIYI_SEARCH_LIVE_TEST=1 for read-only upstream verification")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	client := NewIqiyi(&fakeRepository{}, config.IqiyiSettings{})
	results, err := client.Search(ctx, "苍兰诀")
	if err != nil || len(results) == 0 {
		t.Fatalf("search=%v err=%v", results, err)
	}
	var selected sourceAnime
	for _, item := range results {
		if strings.Contains(item.TypeDescription, "电视剧") {
			selected = item
			break
		}
	}
	if selected.AnimeID == "" {
		selected = results[0]
	}
	episodes, err := client.Episodes(ctx, selected.AnimeID)
	if err != nil || len(episodes) == 0 {
		t.Fatalf("episodes=%v err=%v", episodes, err)
	}
	t.Logf("title=%s media=%s episodes=%d firstVID=%s", selected.Title, selected.AnimeID, len(episodes), episodes[0].EpisodeID)
	// A selected episode must actually be usable by the existing danmaku fetcher.
	data, err := client.fetchData(ctx, episodes[0].EpisodeID)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("first episode comments=%d", len(data))
}
