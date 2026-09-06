package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"git.hanada.info/tamersunion/danmaku/internal/config"
)

func TestDandanplayLive(t *testing.T) {
	if os.Getenv("DANMAKU_DANDANPLAY_LIVE") != "1" {
		t.Skip("set DANMAKU_DANDANPLAY_LIVE=1 to test the public gateway")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	client := NewDandanplay(&fakeDandanplayRepository{}, config.DandanplaySettings{})
	animes, err := client.Search(ctx, "Naruto")
	if err != nil || len(animes) == 0 {
		t.Fatalf("live search count=%d err=%v", len(animes), err)
	}
	episodes, err := client.Episodes(ctx, animes[0].AnimeID)
	if err != nil || len(episodes) == 0 {
		t.Fatalf("live episodes count=%d err=%v", len(episodes), err)
	}
	for _, withRelated := range []bool{true, false} {
		data, err := client.DataWithOffset(ctx, episodes[0].EpisodeID, 0, withRelated)
		if err != nil || len(data) == 0 {
			t.Fatalf("live comments mode=%t count=%d err=%v", withRelated, len(data), err)
		}
		t.Logf("search=%d anime=%s episodes=%d episode=%s withRelated=%t comments=%d", len(animes), animes[0].AnimeID, len(episodes), episodes[0].EpisodeID, withRelated, len(data))
	}
}

func TestDandanplaySearchAndEpisodes(t *testing.T) {
	calls := []string{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gateway" || r.URL.Query().Get("custom") != "value" {
			t.Errorf("not using configured gateway: %s", r.URL)
		}
		path := r.URL.Query().Get("path")
		calls = append(calls, path)
		switch {
		case strings.HasPrefix(path, "/v2/search/anime?"):
			parsed, err := url.Parse(path)
			if err != nil {
				t.Fatal(err)
			}
			keyword := parsed.Query().Get("keyword")
			if keyword != "作品 & ? # 第二季" && keyword != "作品 & ? #" {
				t.Errorf("keyword escaped incorrectly: %q", keyword)
			}
			if strings.HasSuffix(keyword, "第二季") {
				_, _ = io.WriteString(w, `{"success":true,"animes":[]}`)
				return
			}
			_, _ = io.WriteString(w, `{"animes":[{"animeId":42,"animeTitle":"作品 第二季","typeDescription":"TV动画","startDate":"2026-01-01"},{"animeId":"42","animeTitle":"duplicate"},{"animeId":0,"animeTitle":"invalid"}]}`)
		case path == "/v2/bangumi/42":
			_, _ = io.WriteString(w, `{"bangumi":{"episodes":[{"episodeId":123,"episodeTitle":"开始","episodeNumber":"1"},{"episodeId":"124","episodeTitle":"继续","episodeNumber":2},{"episodeId":0,"episodeTitle":"invalid"}]}}`)
		default:
			t.Errorf("unexpected gateway route %q", path)
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	repo := &fakeDandanplayRepository{}
	server := testServer(t, repo)
	server.dandanplay = NewDandanplay(repo, config.DandanplaySettings{APIBase: upstream.URL + "/gateway?custom=value"})
	searchPath := "/api/admin/dandanplay/search"
	response := httptest.NewRecorder()
	server.serveDandanplayAdmin(response, httptest.NewRequest("GET", searchPath+"?"+url.Values{"keyword": {"作品 & ? # 第二季"}}.Encode(), nil), searchPath)
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"animeId":"42"`) || strings.Contains(response.Body.String(), "duplicate") || len(calls) != 2 {
		t.Fatalf("search=%s calls=%v", response.Body, calls)
	}
	episodePath := "/api/admin/dandanplay/anime/42/episodes"
	response = httptest.NewRecorder()
	server.serveDandanplayAdmin(response, httptest.NewRequest("GET", episodePath, nil), episodePath)
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"episodeId":"123"`) || !strings.Contains(response.Body.String(), `"number":"2"`) {
		t.Fatalf("episodes=%s", response.Body)
	}
	if len(repo.dandanplayPools) != 0 {
		t.Fatal("search must not create pools or fetch comments")
	}
	for _, path := range []string{searchPath, episodePath} {
		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if !strings.Contains(response.Body.String(), `"code":401`) {
			t.Errorf("unauthenticated metadata access: %s", response.Body)
		}
	}
	if len(calls) != 3 {
		t.Fatal("unauthenticated search reached upstream")
	}
}

func TestDandanplaySearchErrorsAndBoundedFallback(t *testing.T) {
	calls := 0
	payload := `{"animes":[]}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; _, _ = io.WriteString(w, payload) }))
	defer upstream.Close()
	client := NewDandanplay(&fakeDandanplayRepository{}, config.DandanplaySettings{APIBase: upstream.URL})
	ctx := context.Background()
	if _, err := client.Search(ctx, ""); err == nil || calls != 0 {
		t.Fatal("empty search reached upstream")
	}
	if _, err := client.Episodes(ctx, "1/../../2"); err == nil || calls != 0 {
		t.Fatal("invalid anime ID reached upstream")
	}
	if data, err := client.Search(ctx, "Example Season 2"); err != nil || len(data) != 0 || calls != 2 {
		t.Fatalf("unbounded fallback: calls=%d err=%v", calls, err)
	}
	for _, raw := range []string{`{}`, `{"success":false,"animes":[]}`, `{"animes":null}`, "not json"} {
		payload = raw
		if _, err := client.Search(ctx, "Example"); err == nil {
			t.Errorf("accepted search error %s", raw)
		}
	}
	for _, raw := range []string{`{}`, `{"success":false,"bangumi":{"episodes":[]}}`, `{"bangumi":{"episodes":null}}`, "not json"} {
		payload = raw
		if _, err := client.Episodes(ctx, "42"); err == nil {
			t.Errorf("accepted episodes error %s", raw)
		}
	}
}
