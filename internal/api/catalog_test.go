package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"git.hanada.info/tamersunion/danmaku/internal/config"
	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCatalogIdentityAndRoutes(t *testing.T) {
	for _, tc := range []struct{ source, input, want string }{
		{"bahamut", "000123", "123"}, {"bahamut", "https://ani.gamer.com.tw/animeVideo.php?sn=123", "123"},
		{"tencent", "https://v.qq.com/x/cover/show/vid123.html", "vid123"}, {"tencent", "https://m.v.qq.com/play.html?vid=abc123", "abc123"},
		{"youku", "https://v.youku.com/v_show/id_Xabc==.html", "Xabc=="}, {"youku", "https://v.youku.com/video?vid=Xabc%3D%3D", "Xabc=="},
	} {
		c := NewCatalog(nil, tc.source, config.CatalogSettings{}, nil)
		v, e := c.normalizeID(tc.input)
		if e != nil || v != tc.want {
			t.Errorf("%v got=%q err=%v", tc, v, e)
		}
	}
	for _, source := range []string{"bahamut", "tencent", "youku"} {
		c := NewCatalog(nil, source, config.CatalogSettings{}, nil)
		for _, bad := range []string{"", "../id", "https://evil.test/video?id=1", "https://v.qq.com@127.0.0.1/id", "bad?id"} {
			if _, e := c.normalizeID(bad); e == nil {
				t.Errorf("accepted %s %s", source, bad)
			}
		}
		for _, prefix := range []string{"dplayer/v3/", "artplayer/v1/", "v1/"} {
			if catalogRoute("/api/danmaku/"+prefix+source) != source {
				t.Fatal("route missing")
			}
		}
		if catalogRoute("/api/danmaku/v1/"+source+"/xml") != source {
			t.Fatal("xml route missing")
		}
	}
}
func TestCatalogPassiveSnapshotsAndBindings(t *testing.T) {
	for _, source := range []string{"bahamut", "tencent", "youku"} {
		t.Run(source, func(t *testing.T) {
			entered := make(chan struct{}, 1)
			release := make(chan struct{})
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				select {
				case entered <- struct{}{}:
				default:
				}
				select {
				case <-release:
				case <-r.Context().Done():
					return
				}
				http.Error(w, "offline", 503)
			}))
			defer upstream.Close()
			repo := &fakeRepository{videos: []domain.Video{{ID: 1, Vid: "video", DefaultPool: true}}}
			server := testServer(t, repo)
			defer server.Close()
			i := server.catalogs[source]
			fake := &fakeCatalogRepository{fakeRepository: fakeRepository{videos: repo.videos}}
			i.repository = fake
			i.settings.APIBase = upstream.URL
			i.settings.VideoInfoAPIBase = upstream.URL
			path := "/api/danmaku/dplayer/v3/" + source + "?episodeId=123"
			cold := httptest.NewRecorder()
			done := make(chan struct{})
			go func() { server.Handler().ServeHTTP(cold, httptest.NewRequest("GET", path, nil)); close(done) }()
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("response blocked on upstream")
			}
			if cold.Body.String() != "{\"code\":0,\"data\":[]}\n" {
				t.Fatalf("cold %s", cold.Body)
			}
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("no passive refresh")
			}
			pool, _ := fake.CatalogPool(context.Background(), 1)
			text := "cached"
			_, _ = fake.MergeCatalogDanmaku(context.Background(), pool.ID, []domain.DanmakuData{{Time: 1.25, Text: &text}})
			for _, format := range []string{"dplayer/v3/" + source, "artplayer/v1/" + source, "v1/" + source, "v1/" + source + "/xml"} {
				response := httptest.NewRecorder()
				server.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/api/danmaku/"+format+"?vid=123&offset=2", nil))
				if !strings.Contains(response.Body.String(), "cached") || !strings.Contains(response.Body.String(), "3.25") {
					t.Fatalf("snapshot %s", response.Body)
				}
			}
			response := httptest.NewRecorder()
			link := "/api/admin/videos/1/" + source + "-bindings"
			server.serveVideoAdmin(response, httptest.NewRequest("POST", link, strings.NewReader(`{"poolId":1,"offset":2}`)), link)
			if !strings.Contains(response.Body.String(), `"code":0`) {
				t.Fatalf("bind %s", response.Body)
			}
			response = httptest.NewRecorder()
			server.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/api/danmaku/dplayer/v3?id=video", nil))
			if !strings.Contains(response.Body.String(), "cached") || !strings.Contains(response.Body.String(), "3.25") {
				t.Fatalf("merged %s", response.Body)
			}
			close(release)
			server.refresh.wg.Wait()
			data, _ := fake.CatalogPoolData(context.Background(), 1)
			if len(data) != 1 {
				t.Fatal("failed upstream cleared cache")
			}
			response = httptest.NewRecorder()
			server.serveCatalogAdmin(response, httptest.NewRequest("POST", "/", strings.NewReader(`{"keyword":"spam"}`)), "/api/admin/"+source+"/keywords", i)
			if len(fake.catalogKeywords) != 1 {
				t.Fatalf("keyword %s", response.Body)
			}
		})
	}
}
func TestTencentSegmentParsingAndFailure(t *testing.T) {
	fail := atomic.Bool{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/base/") {
			fmt.Fprint(w, `{"segment_index":{"b":{"segment_start":60000,"segment_name":"60/120"},"a":{"segment_start":0,"segment_name":"0/60"}}}`)
			return
		}
		if fail.Load() && strings.HasSuffix(r.URL.Path, "60/120") {
			http.Error(w, "error", 500)
			return
		}
		fmt.Fprint(w, `{"barrage_list":[{"time_offset":1250,"content":"hello","content_style":"{\"position\":2,\"color\":\"ff0000\"}"}]}`)
	}))
	defer upstream.Close()
	c := NewCatalog(nil, "tencent", config.CatalogSettings{APIBase: upstream.URL}, nil)
	data, err := c.fetchData(context.Background(), "vid")
	if err != nil || len(data) != 2 || data[0].Mode != 5 || data[0].Color != 0xff0000 || data[0].Time != 1.25 {
		t.Fatalf("data=%v err=%v", data, err)
	}
	fail.Store(true)
	if data, err = c.fetchData(context.Background(), "vid"); err == nil || data != nil {
		t.Fatal("partial sync accepted")
	}
	if _, err = fetchCatalogSegments(context.Background(), 2001, nil); err == nil {
		t.Fatal("unbounded segments")
	}
}
func TestYoukuSessionSigningAndParsing(t *testing.T) {
	var segments atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			if r.URL.Query().Get("video_id") != "Xabc==" {
				t.Error("lost VID padding")
			}
			fmt.Fprint(w, `{"duration":61}`)
		case "/cna":
			w.Header().Set("ETag", `"test-cna"`)
			fmt.Fprint(w, "ok")
		case "/session":
			http.SetCookie(w, &http.Cookie{Name: "_m_h5_tk", Value: "01234567890123456789012345678901_123"})
			http.SetCookie(w, &http.Cookie{Name: "_m_h5_tk_enc", Value: "enc"})
			fmt.Fprint(w, `{}`)
		case "/comments":
			segments.Add(1)
			_ = r.ParseForm()
			payload := r.PostForm.Get("data")
			var msg map[string]any
			_ = json.Unmarshal([]byte(payload), &msg)
			if r.URL.Query().Get("sign") != youkuMD5("01234567890123456789012345678901&"+r.URL.Query().Get("t")+"&24679788&"+payload) {
				t.Error("bad outer signature")
			}
			encoded := textValue(msg["msg"])
			decoded, e := base64.StdEncoding.DecodeString(encoded)
			if e != nil || !strings.Contains(string(decoded), "test-cna") || textValue(msg["sign"]) != youkuMD5(encoded+"MkmC9SoIw6xCkSKHhJ7b5D2r51kBiREr") {
				t.Error("bad inner signature")
			}
			if cookie, e := r.Cookie("_m_h5_tk_enc"); e != nil || cookie.Value != "enc" {
				t.Error("session cookies missing")
			}
			result := map[string]any{"code": "0", "data": map[string]any{"result": []any{map[string]any{"playat": 2500, "content": "hello", "propertis": `{"pos":2,"color":"255"}`}}}}
			raw, _ := json.Marshal(result)
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]string{"result": string(raw)}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	c := NewCatalog(nil, "youku", config.CatalogSettings{APIBase: upstream.URL + "/comments", VideoInfoAPIBase: upstream.URL + "/info", CNAAPIBase: upstream.URL + "/cna", SessionAPIBase: upstream.URL + "/session"}, nil)
	data, err := c.fetchData(context.Background(), "Xabc==")
	if err != nil || len(data) != 2 || data[0].Time != 2.5 || data[0].Color != 255 || data[0].Mode != 4 || segments.Load() != 2 {
		t.Fatalf("data=%v err=%v segments=%d", data, err, segments.Load())
	}
	for _, raw := range []string{`{}`, `{"ret":["FAIL_SYS_TOKEN_EXPIRED"]}`, `{"data":{"result":"{\"code\":\"-1\"}"}}`} {
		if _, e := parseYoukuResponse([]byte(raw)); e == nil {
			t.Fatal("accepted upstream failure")
		}
	}
}
func TestBahamutSearchAndEpisodes(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			if r.URL.Query().Get("kw") != "葬送的芙莉蓮" {
				t.Error("no traditional conversion")
			}
			fmt.Fprint(w, `{"anime":[{"video_sn":123,"title":"作品"},{"video_sn":123,"title":"重複"}]}`)
		case "/episodes":
			fmt.Fprint(w, `{"data":{"anime":{"episodes":{"0":[{"videoSn":123,"episode":"1"}],"1":[{"videoSn":456,"episode":"2"}]}}}}`)
		case "/comments":
			if r.URL.Query().Get("geo") != "TW,HK" {
				t.Error("geo")
			}
			fmt.Fprint(w, `{"data":{"danmu":[{"time":15,"text":"hello","position":1,"color":"#FF0000"}]}}`)
		}
	}))
	defer upstream.Close()
	c := NewCatalog(nil, "bahamut", config.CatalogSettings{SearchAPIBase: upstream.URL + "/search", EpisodesAPIBase: upstream.URL + "/episodes", APIBase: upstream.URL + "/comments"}, nil)
	data, e := c.Search(context.Background(), "葬送的芙莉莲")
	if e != nil || len(data) != 1 {
		t.Fatalf("search %v %v", data, e)
	}
	eps, e := c.Episodes(context.Background(), "123")
	if e != nil || len(eps) != 2 {
		t.Fatalf("episodes %v %v", eps, e)
	}
	comments, e := c.fetchData(context.Background(), "123")
	if e != nil || len(comments) != 1 || comments[0].Time != 1.5 || comments[0].Mode != 5 {
		t.Fatalf("comments %v %v", comments, e)
	}
}
func TestYoukuEpisodePagination(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		page := r.URL.Query().Get("page")
		if page == "1" {
			fmt.Fprint(w, `{"total":101,"videos":[{"id":"Xfirst==","title":"1"}]}`)
		} else {
			fmt.Fprint(w, `{"total":101,"videos":[{"id":"Xfirst==","title":"1"},{"id":"Xlast==","title":"101"}]}`)
		}
	}))
	defer upstream.Close()
	c := NewCatalog(nil, "youku", config.CatalogSettings{EpisodesAPIBase: upstream.URL}, nil)
	v, e := c.Episodes(context.Background(), "show")
	if e != nil || len(v) != 2 || calls != 2 {
		t.Fatalf("episodes=%v err=%v calls=%d", v, e, calls)
	}
}

type catalogMetadataCache struct {
	fakeRepository
	values map[string][]byte
}

func (f *catalogMetadataCache) Cache(ctx context.Context, key string, ttl time.Duration, fetch func(context.Context) ([]byte, error)) ([]byte, error) {
	if ttl > 0 && f.values[key] != nil {
		return f.values[key], nil
	}
	raw, err := fetch(ctx)
	if err == nil {
		if f.values == nil {
			f.values = map[string][]byte{}
		}
		f.values[key] = raw
	}
	return raw, err
}
func TestTencentEpisodeTabsAndChapterPagination(t *testing.T) {
	for _, mode := range []string{"tabs", "chapters"} {
		t.Run(mode, func(t *testing.T) {
			var pages []string
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request map[string]any
				_ = json.NewDecoder(r.Body).Decode(&request)
				if r.URL.Path == "/search" {
					_ = json.NewEncoder(w).Encode(map[string]any{"ret": 0, "data": map[string]any{"normalList": map[string]any{"itemList": []any{map[string]any{"doc": map[string]string{"id": "show"}, "videoInfo": map[string]any{"title": "作品", "year": 2026, "typeName": "动漫", "episodeSites": []any{map[string]any{"enName": "qq", "chapterInfo": map[string]any{"chapters": []any{map[string]string{"title": "正片"}, map[string]string{"title": "花絮"}}}}}}}}}}})
					return
				}
				pc := textValue(valueAt(request, "page_params", "page_context"))
				q, _ := url.ParseQuery(pc)
				params := map[string]any{}
				items := []any{}
				if q.Get("req_type") == "0" {
					if mode == "tabs" {
						params["tabs"] = `[{"page_context":"page=1&chapter_name=正片"},{"page_context":"page=2&chapter_name=正片"},{"page_context":"page=extra&chapter_name=花絮"}]`
					}
				} else {
					page := q.Get("page")
					if mode == "chapters" {
						if q.Get("chapter_name") != "正片" {
							t.Error("non-formal chapter requested")
						}
						page = q.Get("page_num")
						params["has_next"] = page == "0"
					}
					pages = append(pages, page)
					items = []any{map[string]any{"item_params": map[string]string{"vid": "vid" + page, "title": "正片", "episode_on_chapter": page}}, map[string]any{"item_params": map[string]string{"vid": "trailer", "is_trailer": "1"}}}
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"ret": 0, "data": map[string]any{"module_list_datas": []any{map[string]any{"module_datas": []any{map[string]any{"module_params": params, "item_data_lists": map[string]any{"item_datas": items}}}}}}})
			}))
			defer upstream.Close()
			c := NewCatalog(&catalogMetadataCache{}, "tencent", config.CatalogSettings{SearchAPIBase: upstream.URL + "/search", EpisodesAPIBase: upstream.URL + "/episodes"}, nil)
			if mode == "chapters" {
				if _, e := c.Search(context.Background(), "作品"); e != nil {
					t.Fatal(e)
				}
			}
			result, e := c.Episodes(context.Background(), "show")
			if e != nil || len(result) != 2 || len(pages) != 2 {
				t.Fatalf("result=%v pages=%v err=%v", result, pages, e)
			}
		})
	}
}
