package api

import (
	"context"
	"encoding/json"
	"git.hanada.info/tamersunion/danmaku/internal/config"
	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

const animekoFixture = `{"danmakuList":[{"id":"a","senderId":"user","danmakuInfo":{"playTime":1250,"color":-1,"text":"hello","location":"NORMAL"}},{"id":"b","senderId":"u","danmakuInfo":{"playTime":2000,"color":-65536,"text":"top","location":"TOP"}},{"id":"c","danmakuInfo":{"playTime":3000,"color":255,"text":"bottom","location":"BOTTOM"}}]}`

func TestAnimekoParser(t *testing.T) {
	data, err := parseAnimekoComments([]byte(animekoFixture))
	if err != nil || len(data) != 3 {
		t.Fatalf("parse=%v %v", data, err)
	}
	if data[0].Time != 1.25 || data[0].Color != 16777215 || data[0].Author != "user" || data[1].Color != 16711680 || data[1].Mode != 5 || data[2].Mode != 4 {
		t.Fatalf("mapping=%v", data)
	}
	for _, raw := range []string{`{}`, `{"danmakuList":null}`, `{"danmakuList":{}}`, `{"danmakuList":[{"danmakuInfo":{"playTime":-1,"color":255,"text":"x","location":"NORMAL"}}]}`} {
		if _, err := parseAnimekoComments([]byte(raw)); err == nil {
			t.Errorf("accepted malformed %s", raw)
		}
	}
	if data, err := parseAnimekoComments([]byte(`{"danmakuList":[]}`)); err != nil || len(data) != 0 {
		t.Fatal("valid empty rejected")
	}
}

func TestAnimekoPassiveRefreshAndAdminVideoLink(t *testing.T) {
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/danmaku/123" || r.URL.RawQuery != "" {
			t.Errorf("endpoint=%s", r.URL)
		}
		select {
		case entered <- struct{}{}:
		default:
		}
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		_, _ = w.Write([]byte(animekoFixture))
	}))
	defer upstream.Close()
	repo := &fakeAnimekoRepository{fakeRepository: fakeRepository{videos: []domain.Video{{ID: 1, Vid: "video", DefaultPool: true}}}}
	server := testServer(t, repo)
	defer server.Close()
	server.animeko.settings.APIBase = upstream.URL
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		server.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/api/danmaku/dplayer/v3/animeko?episodeId=00123", nil))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cold request waited on upstream")
	}
	if response.Body.String() != "{\"code\":0,\"data\":[]}\n" {
		t.Fatalf("cold=%s", response.Body)
	}
	<-entered
	close(release)
	server.refresh.wg.Wait()
	for _, format := range []string{"dplayer/v3", "artplayer/v1", "v1", "v1/animeko/xml"} {
		path := "/api/danmaku/" + format + "/animeko?episodeId=123&offset=2"
		if strings.HasSuffix(format, "xml") {
			path = "/api/danmaku/v1/animeko/xml?episodeId=123&offset=2"
		}
		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		server.refresh.wg.Wait()
		if response.Code != 200 || !strings.Contains(response.Body.String(), "hello") || !strings.Contains(response.Body.String(), "3.25") {
			t.Fatalf("format=%s response=%s", format, response.Body)
		}
	}
	response = httptest.NewRecorder()
	server.serveVideoAdmin(response, httptest.NewRequest("POST", "/api/admin/videos/1/animeko-bindings", strings.NewReader(`{"poolId":1,"offset":2}`)), "/api/admin/videos/1/animeko-bindings")
	if response.Code != 200 || len(repo.animekoBindings) != 1 {
		t.Fatalf("binding=%s", response.Body)
	}
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/api/danmaku/dplayer/v3?id=video", nil))
	server.refresh.wg.Wait()
	if !strings.Contains(response.Body.String(), "3.25") || !strings.Contains(response.Body.String(), "hello") {
		t.Fatalf("merged=%s", response.Body)
	}
	response = httptest.NewRecorder()
	server.serveAnimekoAdmin(response, httptest.NewRequest("POST", "/api/admin/animeko/keywords", strings.NewReader(`{"keyword":"spam"}`)), "/api/admin/animeko/keywords")
	if !strings.Contains(response.Body.String(), "\"code\":0") {
		t.Fatalf("keyword=%s", response.Body)
	}
	for _, id := range []string{"0", "-1", "bad", "1/2"} {
		if _, err := server.animeko.DataWithOffset(context.Background(), id, 0); err == nil {
			t.Errorf("accepted ID %s", id)
		}
	}
}

func TestAnimekoSearchAndEpisodePagination(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "search") {
			var body struct {
				Keyword string `json:"keyword"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if r.Method != "POST" || body.Keyword != "test" {
				t.Errorf("search=%s %#v", r.Method, body)
			}
			_, _ = w.Write([]byte(`{"data":[{"id":42,"name":"original","name_cn":"作品","date":"2026-01-01"}]}`))
			return
		}
		calls++
		if r.URL.Query().Get("subject_id") != "42" || r.URL.Query().Get("type") != "0" {
			t.Errorf("query=%s", r.URL)
		}
		data := []map[string]any{}
		if calls == 1 {
			for n := 1; n <= 200; n++ {
				data = append(data, map[string]any{"id": n, "sort": n, "name": "episode"})
			}
		} else {
			data = append(data, map[string]any{"id": 201, "sort": 201, "name": "last"})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"total": 201, "data": data})
	}))
	defer upstream.Close()
	client := NewAnimeko(&fakeAnimekoRepository{}, config.AnimekoSettings{BangumiAPIBase: upstream.URL})
	result, err := client.Search(context.Background(), "test")
	if err != nil || len(result) != 1 || result[0].AnimeID != "42" || result[0].Title != "作品" {
		t.Fatalf("search=%v %v", result, err)
	}
	episodes, err := client.Episodes(context.Background(), "42")
	if err != nil || len(episodes) != 201 || calls != 2 {
		t.Fatalf("episodes=%d calls=%d err=%v", len(episodes), calls, err)
	}
}

func TestAnimekoLive(t *testing.T) {
	if os.Getenv("ANIMEKO_LIVE_TEST") == "" {
		t.Skip("set ANIMEKO_LIVE_TEST=1 for read-only upstream verification")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	client := NewAnimeko(&fakeAnimekoRepository{}, config.AnimekoSettings{})
	results, err := client.Search(ctx, "葬送的芙莉莲")
	if err != nil || len(results) == 0 {
		t.Fatalf("search=%v err=%v", results, err)
	}
	episodes, err := client.Episodes(ctx, results[0].AnimeID)
	if err != nil || len(episodes) == 0 {
		t.Fatalf("episodes=%v err=%v", episodes, err)
	}
	data, err := client.fetchData(ctx, episodes[0].EpisodeID)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("subject=%s episodes=%d firstEpisode=%s comments=%d", results[0].AnimeID, len(episodes), episodes[0].EpisodeID, len(data))
}
