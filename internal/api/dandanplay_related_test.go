package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"git.hanada.info/tamersunion/danmaku/internal/domain"
)

func TestDandanplayRelatedPoolsAreIndependent(t *testing.T) {
	calls := map[string]int{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, err := url.Parse(r.URL.Query().Get("path"))
		if err != nil {
			t.Fatal(err)
		}
		mode := path.Query().Get("withRelated")
		if mode != "true" && mode != "false" {
			t.Errorf("missing explicit mode: %s", path)
		}
		calls[mode]++
		_, _ = fmt.Fprintf(w, `{"comments":[{"p":"1,1,255,u","m":"mode-%s"}]}`, mode)
	}))
	defer upstream.Close()
	repo := &fakeDandanplayRepository{fakeRepository: fakeRepository{videos: []domain.Video{{ID: 1, Vid: "mixed", DefaultPool: true}}}}
	server := testServer(t, repo)
	server.dandanplay.settings.APIBase = upstream.URL
	ctx := context.Background()
	for _, body := range []string{`{"episodeId":"123"}`, `{"episodeId":"123","withRelated":false}`} {
		response := httptest.NewRecorder()
		server.serveDandanplayAdmin(response, httptest.NewRequest("POST", "/api/admin/dandanplay/pools", strings.NewReader(body)), "/api/admin/dandanplay/pools")
		if response.Code != 200 || !strings.Contains(response.Body.String(), `"code":0`) {
			t.Fatalf("create=%s", response.Body)
		}
	}
	if len(repo.dandanplayPools) != 2 || !repo.dandanplayPools[0].WithRelated || repo.dandanplayPools[1].WithRelated {
		t.Fatalf("pools=%#v", repo.dandanplayPools)
	}
	for pass := 0; pass < 2; pass++ {
		for _, mode := range []bool{true, false} {
			data, err := readDandanplayAfterRefresh(server.dandanplay, ctx, "00123", 0, mode)
			if err != nil || len(data) != 1 || *data[0].Text != fmt.Sprint("mode-", mode) {
				t.Fatalf("mode=%t data=%v err=%v", mode, data, err)
			}
		}
	}
	if calls["true"] != 1 || calls["false"] != 1 {
		t.Fatalf("shared cache window: %v", calls)
	}
	// Refreshing the false pool cannot change the mode or expire the true pool.
	repo.dandanplayClaims[2] = time.Now().Add(-601 * time.Second)
	if _, err := readDandanplayAfterRefresh(server.dandanplay, ctx, "123", 0, false); err != nil {
		t.Fatal(err)
	}
	if _, _, err := server.dandanplay.SyncPool(ctx, 2); err != nil {
		t.Fatal(err)
	}
	if calls["true"] != 1 || calls["false"] != 3 {
		t.Fatalf("refresh changed mode: %v", calls)
	}
	for _, pool := range repo.dandanplayPools {
		if _, err := repo.UpsertVideoDandanplayBinding(ctx, 1, pool.ID, 0); err != nil {
			t.Fatal(err)
		}
	}
	merged, err := server.mergedVideoData(httptest.NewRequest("GET", "/", nil), "mixed")
	server.refresh.wg.Wait()
	if err != nil || len(merged) != 2 || calls["true"] != 1 || calls["false"] != 3 {
		t.Fatalf("merge=%v err=%v calls=%v", merged, err, calls)
	}
	for _, format := range []string{"dplayer/v3", "v1", "artplayer/v1"} {
		for _, mode := range []string{"false", "true", ""} {
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/api/danmaku/"+format+"/dandanplay?episodeId=123&withRelated="+mode, nil))
			server.refresh.wg.Wait()
			want := mode
			if want == "" {
				want = "true"
			}
			if response.Code != 200 || !strings.Contains(response.Body.String(), "mode-"+want) || strings.Contains(response.Body.String(), "mode-"+fmt.Sprint(want == "false")) {
				t.Fatalf("format=%s mode=%s body=%s", format, mode, response.Body)
			}
		}
	}
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/api/danmaku/v1/dandanplay?episodeId=123&withRelated=invalid", nil))
	server.refresh.wg.Wait()
	if response.Code != 400 || len(repo.dandanplayPools) != 2 {
		t.Fatal("invalid mode should fail without creating a pool")
	}
}
