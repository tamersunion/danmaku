package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"git.hanada.info/tamersunion/danmaku/internal/config"
	"git.hanada.info/tamersunion/danmaku/internal/domain"
)

func TestDandanplayParser(t *testing.T) {
	data, err := parseDandanplayComments([]byte(`{"comments":[{"p":"1.25,1,16777215,user","m":"hello & world"},{"p":"2,4,255,u","m":"bottom"},{"p":"3,5,16711680,u","m":"top"},{"p":"NaN,1,1,u","m":"bad"},{"p":"1,1,-1,u","m":"bad"}]}`))
	if err != nil || len(data) != 3 {
		t.Fatalf("data=%#v err=%v", data, err)
	}
	if data[0].Time != 1.25 || data[0].Author != "user" || *data[0].Text != "hello & world" || data[1].Mode != 4 || data[2].Color != 16711680 {
		t.Fatalf("unexpected parse: %#v", data)
	}
	for _, raw := range []string{`{}`, `{"success":false,"comments":[]}`, `{"comments":null}`, `{"comments":{}}`, `{"comments":[{"p":"NaN,1,1,u","m":"bad"}]}`, "not json"} {
		if _, err := parseDandanplayComments([]byte(raw)); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	if data, err := parseDandanplayComments([]byte(`{"comments":[]}`)); err != nil || len(data) != 0 {
		t.Fatalf("empty response: %v %v", data, err)
	}
}

func TestDandanplayEpisodeIdentity(t *testing.T) {
	for _, id := range []string{"", "0", "-1", "+1", "1/2", "1?x=1", "1.5", "9223372036854775808", "abc"} {
		if _, err := normalizeDandanplayEpisodeID(id); err == nil {
			t.Errorf("accepted ID %q", id)
		}
	}
	if got, err := normalizeDandanplayEpisodeID(" 00123 "); err != nil || got != "123" {
		t.Fatalf("normalized=%q err=%v", got, err)
	}
}

func TestDandanplayPassiveIncrementalCacheAndFailure(t *testing.T) {
	calls := 0
	payload := `{"comments":[{"p":"1.25,1,16777215,u","m":"original"}]}`
	status := 200
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("path") != "/v2/comment/123?from=0&withRelated=true&chConvert=0" || r.URL.Query().Get("custom") != "yes" {
			t.Errorf("gateway URL: %s", r.URL)
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, payload)
	}))
	defer upstream.Close()
	repo := &fakeDandanplayRepository{}
	client := NewDandanplay(repo, config.DandanplaySettings{APIBase: upstream.URL + "?custom=yes", SyncIntervalSeconds: 600})
	ctx := context.Background()
	data, err := client.DataWithOffset(ctx, "00123", 2)
	if err != nil || len(data) != 1 || data[0].Time != 3.25 || calls != 1 {
		t.Fatalf("initial=%v %v calls=%d", data, err, calls)
	}
	data, err = client.DataWithOffset(ctx, "123", 0)
	if err != nil || data[0].Time != 1.25 || calls != 1 {
		t.Fatal("cache not reused or offset mutated cached data")
	}
	payload = `{"comments":[{"p":"1.25,1,16777215,u","m":"original"},{"p":"2,5,255,u","m":"new"}]}`
	repo.dandanplayClaims[1] = time.Now().Add(-601 * time.Second)
	data, err = client.DataWithOffset(ctx, "123", 0)
	if err != nil || len(data) != 2 || calls != 2 {
		t.Fatalf("incremental=%v %v calls=%d", data, err, calls)
	}
	payload = `{"comments":[]}`
	_, inserted, err := client.SyncPool(ctx, 1)
	if err != nil || inserted != 0 || len(repo.dandanplayData[1]) != 2 || calls != 3 {
		t.Fatal("empty refresh lost history or did not force refresh")
	}
	status = 502
	if _, _, err = client.SyncPool(ctx, 1); err == nil {
		t.Fatal("manual refresh must report upstream failure")
	}
	repo.dandanplayClaims[1] = time.Now().Add(-601 * time.Second)
	data, err = client.DataWithOffset(ctx, "123", 0)
	if err != nil || len(data) != 2 {
		t.Fatal("passive error discarded stale data")
	}
	if _, _, err = client.PreparePool(ctx, "bad"); err == nil || len(repo.dandanplayPools) != 1 {
		t.Fatal("invalid identity reached storage")
	}
}

func TestDandanplayAdminRoutesAndVideoMerge(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"comments":[{"p":"1.25,1,255,u","m":"linked"}]}`)
	}))
	defer upstream.Close()
	repo := &fakeDandanplayRepository{fakeRepository: fakeRepository{videos: []domain.Video{{ID: 1, Vid: "local-video", DefaultPool: true}}}}
	server := testServer(t, repo)
	server.dandanplay.settings.APIBase = upstream.URL
	create := httptest.NewRecorder()
	server.serveDandanplayAdmin(create, httptest.NewRequest("POST", "/api/admin/dandanplay/pools", strings.NewReader(`{"episodeId":"123"}`)), "/api/admin/dandanplay/pools")
	if create.Code != 200 || len(repo.dandanplayData[1]) != 1 || !strings.Contains(create.Body.String(), `"episodeId":"123"`) {
		t.Fatalf("create=%s", create.Body)
	}
	bind := httptest.NewRecorder()
	server.serveVideoAdmin(bind, httptest.NewRequest("POST", "/api/admin/videos/1/dandanplay-bindings", strings.NewReader(`{"poolId":1,"offset":2.5}`)), "/api/admin/videos/1/dandanplay-bindings")
	if bind.Code != 200 || len(repo.dandanplayBindings) != 1 {
		t.Fatalf("bind=%s", bind.Body)
	}
	for _, path := range []string{"/api/danmaku/dplayer/v3/dandanplay/?episodeId=123&offset=2.5", "/api/danmaku/v1/dandanplay?episodeId=123&offset=2.5", "/api/danmaku/artplayer/v1/dandanplay?episodeId=123&offset=2.5", "/api/danmaku/v1/dandanplay/xml?episodeId=123&offset=2.5", "/api/danmaku/dplayer/v3?id=local-video"} {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Code != 200 || !strings.Contains(response.Body.String(), "linked") || !strings.Contains(response.Body.String(), "3.75") {
			t.Errorf("%s: %d %s", path, response.Code, response.Body)
		}
	}
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/api/admin/dandanplay/pools", nil))
	if !strings.Contains(response.Body.String(), `"code":401`) {
		t.Fatal("admin API must require authentication")
	}
	response = httptest.NewRecorder()
	server.serveAdmin(response, httptest.NewRequest("GET", "/api/admin/dandanplay/pools", nil), "/api/admin/dandanplay/pools", session{Role: 3})
	if !strings.Contains(response.Body.String(), `"code":401`) {
		t.Fatal("ordinary users must not manage provider pools")
	}
	repo.videos[0].IsDeleted = true
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/api/danmaku/dplayer/v3?id=local-video", nil))
	if strings.Contains(response.Body.String(), "linked") {
		t.Fatal("deleted video exposed linked comments")
	}
}
