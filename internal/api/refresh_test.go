package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"git.hanada.info/tamersunion/danmaku/internal/domain"
)

type cachedRefreshRepository struct{ *fakeDandanplayRepository }

func (r *cachedRefreshRepository) CachedMergedDanmaku(_ context.Context, _ string, _ func(context.Context) ([]domain.DanmakuData, error)) ([]domain.DanmakuData, error) {
	text := "existing merged cache"
	return []domain.DanmakuData{{Time: 1, Text: &text}}, nil
}

// Parser/cache tests explicitly wait for passive refresh completion.
// Production reads do not use these helpers.
func readBilibiliAfterRefresh(b *Bilibili, ctx context.Context, query bilibiliQuery) ([]domain.DanmakuData, error) {
	if _, err := b.Data(ctx, query); err != nil {
		return nil, err
	}
	b.refresh.wg.Wait()
	data, err := b.Data(ctx, query)
	b.refresh.wg.Wait()
	return data, err
}
func readIqiyiAfterRefresh(i *Iqiyi, ctx context.Context, vid string) ([]domain.DanmakuData, error) {
	if _, err := i.Data(ctx, vid); err != nil {
		return nil, err
	}
	i.refresh.wg.Wait()
	data, err := i.Data(ctx, vid)
	i.refresh.wg.Wait()
	return data, err
}
func readDandanplayAfterRefresh(i *Dandanplay, ctx context.Context, id string, offset float64, related bool) ([]domain.DanmakuData, error) {
	if _, err := i.DataWithOffset(ctx, id, offset, related); err != nil {
		return nil, err
	}
	i.refresh.wg.Wait()
	data, err := i.DataWithOffset(ctx, id, offset, related)
	i.refresh.wg.Wait()
	return data, err
}

func TestPublicReadsNeverWaitForThirdParty(t *testing.T) {
	for _, target := range []string{
		"/api/danmaku/dplayer/v3/bilibili?cid=99",
		"/api/danmaku/dplayer/v3/iqiyi?vid=abc",
		"/api/danmaku/dplayer/v3/dandanplay?episodeId=123",
		"/api/danmaku/dplayer/v3?id=video",
		"/api/danmaku/dplayer/v3?id=video&cached=true",
	} {
		t.Run(target, func(t *testing.T) {
			entered := make(chan struct{}, 8)
			release := make(chan struct{})
			var calls atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				entered <- struct{}{}
				select {
				case <-release:
				case <-r.Context().Done():
					return
				}
				http.Error(w, "upstream unavailable", 502)
			}))
			defer upstream.Close()
			defer close(release)
			text := "existing"
			data := []domain.DanmakuData{{Time: 1, Text: &text}}
			repo := &fakeDandanplayRepository{
				fakeRepository: fakeRepository{
					videos:           []domain.Video{{ID: 1, Vid: "video", DefaultPool: true}},
					bilibiliPools:    []domain.BilibiliPool{{ID: 1, CID: 99, Page: 1}},
					bilibiliData:     map[int][]domain.DanmakuData{1: data},
					iqiyiPools:       []domain.IqiyiPool{{ID: 1, VID: "abc"}},
					iqiyiData:        map[int][]domain.DanmakuData{1: data},
					bilibiliBindings: []domain.BilibiliBinding{{PoolID: 1, CID: 99, Page: 1, Vid: "video"}},
					iqiyiBindings:    []domain.IqiyiBinding{{PoolID: 1, PoolVID: "abc", Vid: "video"}},
				},
				dandanplayPools:    []domain.DandanplayPool{{ID: 1, EpisodeID: "123", WithRelated: true}},
				dandanplayData:     map[int][]domain.DanmakuData{1: data},
				dandanplayBindings: []domain.DandanplayBinding{{PoolID: 1, PoolEpisodeID: "123", WithRelated: true, Vid: "video"}},
			}
			server := testServer(t, repo)
			if strings.Contains(target, "cached=true") {
				server.Close()
				server = testServer(t, &cachedRefreshRepository{repo})
			}
			defer server.Close()
			server.bilibili.baseURL = upstream.URL
			server.iqiyi.decodeAPIBase = upstream.URL
			server.dandanplay.settings.APIBase = upstream.URL
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			for attempt := 0; attempt < 2; attempt++ {
				done := make(chan struct{})
				response := httptest.NewRecorder()
				go func() {
					server.Handler().ServeHTTP(response, httptest.NewRequest("GET", target, nil).WithContext(ctx))
					close(done)
				}()
				select {
				case <-done:
				case <-time.After(time.Second):
					t.Fatal("public read blocked on upstream")
				}
				if response.Code != 200 || !strings.Contains(response.Body.String(), "existing") {
					t.Fatalf("lost local snapshot: %d %s", response.Code, response.Body)
				}
			}
			expected := 1
			if strings.Contains(target, "?id=") {
				expected = 3
			}
			for n := 0; n < expected; n++ {
				select {
				case <-entered:
				case <-time.After(time.Second):
					t.Fatal("background refresh did not start")
				}
			}
			cancel()
			server.refresh.mu.Lock()
			active := len(server.refresh.active)
			server.refresh.mu.Unlock()
			if active != expected || calls.Load() != int32(expected) {
				t.Fatalf("refresh not coalesced: active=%d calls=%d", active, calls.Load())
			}
		})
	}
}

func TestColdBilibiliMetadataIsAlsoAsynchronous(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/x/player/pagelist" {
			close(entered)
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
			_, _ = w.Write([]byte(`{"code":0,"data":[{"cid":99,"page":1}]}`))
		} else {
			_, _ = w.Write([]byte(`<i><d p="1,1,25,255,123,0,u,1">new</d></i>`))
		}
	}))
	defer upstream.Close()
	server := testServer(t, &fakeRepository{})
	defer server.Close()
	server.bilibili.baseURL = upstream.URL
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		server.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/api/danmaku/dplayer/v3/bilibili?aid=79671692", nil))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("metadata blocked cold response")
	}
	if response.Body.String() != "{\"code\":0,\"data\":[]}\n" {
		t.Fatalf("cold response=%s", response.Body)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("metadata not fetched")
	}
	close(release)
	server.refresh.wg.Wait()
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/api/danmaku/dplayer/v3/bilibili?aid=79671692", nil))
	server.refresh.wg.Wait()
	if !strings.Contains(response.Body.String(), "new") {
		t.Fatalf("new cache not visible: %s", response.Body)
	}
}

func TestBackgroundRefreshCapacityAndShutdown(t *testing.T) {
	refresh := newBackgroundRefresh(context.Background(), nil)
	var running atomic.Int32
	var peak atomic.Int32
	for n := 0; n < 100; n++ {
		refresh.schedule(string(rune(n)), func(ctx context.Context) error {
			count := running.Add(1)
			defer running.Add(-1)
			for old := peak.Load(); count > old; old = peak.Load() {
				if peak.CompareAndSwap(old, count) {
					break
				}
			}
			<-ctx.Done()
			return ctx.Err()
		})
	}
	refresh.close()
	if peak.Load() > 8 || running.Load() != 0 {
		t.Fatalf("worker lifecycle: peak=%d running=%d", peak.Load(), running.Load())
	}
	refresh.schedule("closed", func(context.Context) error { t.Error("work accepted after close"); return nil })
	refresh.wg.Wait()
}

func TestBackgroundRefreshQueuesMoreThanEightPools(t *testing.T) {
	refresh := newBackgroundRefresh(context.Background(), nil)
	defer refresh.close()
	release := make(chan struct{})
	entered := make(chan struct{}, 24)
	var completed atomic.Int32
	for n := 0; n < 24; n++ {
		refresh.schedule(fmt.Sprint(n), func(ctx context.Context) error {
			entered <- struct{}{}
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
			completed.Add(1)
			return nil
		})
	}
	for n := 0; n < 8; n++ {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("workers did not start")
		}
	}
	select {
	case <-entered:
		t.Fatal("more than eight concurrent upstream jobs")
	default:
	}
	close(release)
	refresh.wg.Wait()
	if completed.Load() != 24 {
		t.Fatalf("queued pools lost: %d", completed.Load())
	}
}
