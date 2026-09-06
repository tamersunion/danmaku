package api

import (
	"context"
	"git.hanada.info/tamersunion/danmaku/internal/config"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestBilibiliInputFormats(t *testing.T) {
	for _, item := range []struct {
		input, ref string
		p          int
	}{
		{"av79671692", "av79671692", 1}, {"BV1EJ411r7kH", "BV1EJ411r7kH", 1}, {"EP00123", "ep123", 1}, {"ss456", "ss456", 1},
		{"https://www.bilibili.com/video/BV1EJ411r7kH/?p=2&share_source=copy", "BV1EJ411r7kH", 2},
		{"m.bilibili.com/video/av79671692", "av79671692", 1},
		{"//www.bilibili.com/bangumi/play/ep123?from=search", "ep123", 1},
		{"https://www.bilibili.com/bangumi/play/ss456", "ss456", 1},
		{"https://player.bilibili.com/player.html?aid=79671692&page=2", "av79671692", 2},
		{"bilibili://video/79671692", "av79671692", 1}, {"bilibili://bangumi/season/456", "ss456", 1},
		{"bilibili://bangumi/episode/123", "ep123", 1},
		{"【分享标题】 https://www.bilibili.com/video/BV1EJ411r7kH/?p=3 （分享）", "BV1EJ411r7kH", 3},
	} {
		got, err := parseBilibiliInput(item.input)
		if err != nil || got.Reference != item.ref || got.Page != item.p {
			t.Errorf("%s => %#v %v", item.input, got, err)
		}
	}
	for _, value := range []string{"https://evil.test/video/BV1EJ411r7kH", "https://bilibili.com.evil.test/video/av1", "http://127.0.0.1/video/av1", "https://www.bilibili.com@127.0.0.1/video/av1", "file:///video/av1", "https://www.bilibili.com/video/av1?p=-1", "av0", "ep-1", "ssno"} {
		if _, err := parseBilibiliInput(value); err == nil {
			t.Errorf("accepted invalid input %s", value)
		}
	}
}

type biliTransportFunc func(*http.Request) (*http.Response, error)

func (f biliTransportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestBilibiliShortLinksOnlyFollowAllowedHosts(t *testing.T) {
	b := NewBilibili(&fakeRepository{}, config.BilibiliSettings{})
	calls := 0
	target := "https://www.bilibili.com/video/BV1EJ411r7kH?p=2"
	b.client.Transport = biliTransportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host != "b23.tv" || r.Header.Get("Cookie") != "" {
			t.Errorf("unexpected short link request %s", r.URL)
		}
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": {target}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})
	got, err := b.expandBilibiliInput(context.Background(), "https://b23.tv/test")
	if err != nil || got.Reference != "BV1EJ411r7kH" || got.Page != 2 || calls != 1 {
		t.Fatalf("short=%v err=%v calls=%d", got, err, calls)
	}
	target = "http://127.0.0.1/private"
	if _, err := b.expandBilibiliInput(context.Background(), "https://b23.tv/test"); err == nil {
		t.Fatal("unsafe redirect accepted")
	}
	if calls != 2 {
		t.Fatal("unsafe redirect was requested")
	}
}

func TestBilibiliResolvePartsSeasonsAndCreate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/x/web-interface/view":
			if r.URL.Query().Get("bvid") != "BV1EJ411r7kH" {
				t.Errorf("unexpected bvid %s", r.URL)
			}
			_, _ = w.Write([]byte(`{"code":0,"data":{"title":"video","pages":[{"cid":11,"page":1,"part":"one"},{"cid":22,"page":2,"part":"two"}]}}`))
		case "/pgc/view/web/season":
			_, _ = w.Write([]byte(`{"code":0,"result":{"season_id":456,"episodes":[{"id":123,"aid":79671692,"cid":11,"title":"1"},{"id":124,"bvid":"BV1EJ411r7kH","cid":22,"long_title":"second","title":"2"}]}}`))
		case "/x/v1/dm/list.so":
			_, _ = w.Write([]byte(`<i><d p="1,1,25,255,123,0,u,1">hello</d></i>`))
		default:
			t.Errorf("unexpected route %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	repo := &fakeRepository{}
	b := NewBilibili(repo, config.BilibiliSettings{APIBase: server.URL})
	for _, item := range []struct {
		input string
		count int
		cid   int64
	}{
		{"av79671692", 2, 11}, {"https://www.bilibili.com/video/BV1EJ411r7kH?p=2", 1, 22}, {"ss456", 2, 11}, {"ep124", 1, 22},
	} {
		data, err := b.ResolveInput(context.Background(), item.input)
		if err != nil || len(data) != item.count || data[0].CID != item.cid {
			t.Fatalf("%s => %v %v", item.input, data, err)
		}
	}
	pool, _, err := b.PreparePool(context.Background(), bilibiliQuery{BVID: "https://www.bilibili.com/video/BV1EJ411r7kH?p=2", Page: 1})
	if err != nil || pool.CID != 22 || pool.Page != 2 {
		t.Fatalf("link create=%v err=%v", pool, err)
	}
	second, _, err := b.PreparePool(context.Background(), bilibiliQuery{CID: 22})
	if err != nil || second.ID != pool.ID || len(repo.bilibiliPools) != 1 {
		t.Fatal("link duplicated CID pool")
	}
}

func TestBilibiliSearchWBISignature(t *testing.T) {
	q := url.Values{"foo": {"114"}, "bar": {"514"}, "baz": {"1919810"}}
	got := biliWBIQuery(q, "ea1db124af3c7062474693fa704f4ff8", 1702204169)
	if !strings.Contains(got, "wts=1702204169") || !strings.Contains(got, "w_rid=") {
		t.Fatal(got)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/x/web-interface/nav" {
			_, _ = w.Write([]byte(`{"code":-101,"data":{"wbi_img":{"img_url":"https://i0.hdslb.com/bfs/wbi/7cd084941338484aae1ad9425b84077c.png","sub_url":"https://i0.hdslb.com/bfs/wbi/4932caff0ff746eab6f01bf08b70ac45.png"}}}`))
			return
		}
		if r.URL.Query().Get("keyword") != "test" || r.URL.Query().Get("w_rid") == "" || r.Header.Get("Referer") == "" {
			t.Error("missing signed search parameters")
		}
		_, _ = w.Write([]byte(`{"code":0,"data":{"result":[{"bvid":"BV1EJ411r7kH","title":"<em>test</em> &amp; video"},{"season_id":123,"title":"show"}]}}`))
	}))
	defer upstream.Close()
	b := NewBilibili(&fakeRepository{}, config.BilibiliSettings{APIBase: upstream.URL})
	data, err := b.Search(context.Background(), "test", "video")
	if err != nil || len(data) != 2 || data[0].Title != "test & video" {
		t.Fatalf("search=%v err=%v", data, err)
	}
}

func TestBilibiliSearchLive(t *testing.T) {
	if os.Getenv("BILIBILI_SEARCH_LIVE_TEST") == "" {
		t.Skip("set BILIBILI_SEARCH_LIVE_TEST=1 for read-only upstream verification")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	b := NewBilibili(&fakeRepository{}, config.BilibiliSettings{})
	data, err := b.ResolveInput(ctx, "https://www.bilibili.com/video/av79671692")
	if err != nil || len(data) == 0 {
		t.Fatalf("resolve=%v err=%v", data, err)
	}
	t.Logf("AV resolved BVID=%s CID=%d", data[0].BVID, data[0].CID)
	for _, kind := range []string{"video", "media_bangumi", "media_ft"} {
		results, err := b.Search(ctx, "葬送的芙莉莲", kind)
		if err != nil {
			t.Errorf("search %s: %v", kind, err)
			continue
		}
		t.Logf("search %s results=%d", kind, len(results))
		if kind == "media_bangumi" && len(results) > 0 {
			episodes, err := b.ResolveInput(ctx, results[0].AnimeID)
			if err != nil || len(episodes) == 0 {
				t.Errorf("season resolve: %v", err)
			} else {
				t.Logf("season=%s episodes=%d", results[0].AnimeID, len(episodes))
			}
		}
	}
}
