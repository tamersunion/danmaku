package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"git.hanada.info/tamersunion/danmaku/internal/config"
	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"github.com/andybalholm/brotli"
)

func TestIqiyiDataFetchesSignedBrotliSegments(t *testing.T) {
	payload := brotliPayload(t, `<danmu><data><entry><list><bulletInfo><content>hello &amp; goodbye</content><showTime>1.25</showTime><color>ff0000</color><likeCount>2</likeCount></bulletInfo></list></entry></data></danmu>`)
	requested := map[string]int{}
	var lock sync.Mutex
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lock.Lock()
		requested[r.URL.RequestURI()]++
		lock.Unlock()
		switch r.URL.Path {
		case "/decode/abc":
			_, _ = io.WriteString(w, `{"code":"0","data":123456}`)
		case "/video/123456":
			_, _ = io.WriteString(w, `{"code":"A00000","data":{"durationSec":61,"displayBarrage":true}}`)
		case "/bullet/34/56/123456_60_1_dce89d6c.br":
			if r.Header.Get("Accept-Encoding") != "br" || r.Header.Get("Content-Type") != "application/octet-stream" || r.Header.Get("User-Agent") == "" {
				t.Errorf("unexpected segment headers: %#v", r.Header)
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(payload)
		case "/bullet/34/56/123456_60_2_029fca1c.br":
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	client := testIqiyiClient(upstream.URL)
	data, err := readIqiyiAfterRefresh(client, context.Background(), "abc")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 1 || data[0].Time != 1.25 || data[0].Mode != 1 || data[0].Color != 16777215 || data[0].Text == nil || *data[0].Text != "hello & goodbye" {
		t.Fatalf("unexpected iqiyi data: %#v", data)
	}
	for _, endpoint := range []string{
		"/decode/abc?platformId=3&modeCode=intl&langCode=sg",
		"/video/123456",
		"/bullet/34/56/123456_60_1_dce89d6c.br",
		"/bullet/34/56/123456_60_2_029fca1c.br",
	} {
		if requested[endpoint] != 1 {
			t.Fatalf("request count for %s = %d", endpoint, requested[endpoint])
		}
	}
}

func TestIqiyiPoolKeepsCachedDanmakuWhenLaterSyncIsEmpty(t *testing.T) {
	payload := brotliPayload(t, `<danmu><data><entry><list><bulletInfo><content>cached</content><showTime>1.25</showTime></bulletInfo></list></entry></data></danmu>`)
	empty := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/decode/"):
			_, _ = io.WriteString(w, `{"code":"0","data":"1078946400"}`)
		case strings.HasPrefix(r.URL.Path, "/video/"):
			_, _ = io.WriteString(w, `{"code":"A00000","data":{"durationSec":60,"displayBarrage":true}}`)
		case strings.HasPrefix(r.URL.Path, "/bullet/"):
			if empty {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	repository := &fakeRepository{}
	client := NewIqiyi(repository, config.IqiyiSettings{SyncIntervalSeconds: 600})
	client.decodeAPIBase = upstream.URL + "/decode"
	client.videoInfoAPIBase = upstream.URL + "/video"
	client.danmakuAPIBase = upstream.URL + "/bullet"
	pool, inserted, err := client.PreparePool(context.Background(), "source-vid")
	if err != nil || pool == nil || inserted != 1 {
		t.Fatalf("initial sync pool=%#v inserted=%d err=%v", pool, inserted, err)
	}
	empty = true
	pool, inserted, err = client.SyncPool(context.Background(), pool.ID)
	if err != nil || pool == nil || inserted != 0 {
		t.Fatalf("empty sync pool=%#v inserted=%d err=%v", pool, inserted, err)
	}
	data, err := readIqiyiAfterRefresh(client, context.Background(), "source-vid")
	if err != nil || len(data) != 1 || data[0].Text == nil || *data[0].Text != "cached" {
		t.Fatalf("cached data=%#v err=%v", data, err)
	}
}

func TestIqiyiAdminPoolCanBindAndMergeIntoVideo(t *testing.T) {
	payload := brotliPayload(t, `<danmu><data><entry><list><bulletInfo><content>linked</content><showTime>1.25</showTime></bulletInfo></list></entry></data></danmu>`)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/decode/"):
			_, _ = io.WriteString(w, `{"code":"0","data":"1078946400"}`)
		case strings.HasPrefix(r.URL.Path, "/video/"):
			_, _ = io.WriteString(w, `{"code":"A00000","data":{"durationSec":60,"displayBarrage":true}}`)
		case strings.HasPrefix(r.URL.Path, "/bullet/"):
			_, _ = w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	repository := &fakeRepository{videos: []domain.Video{{ID: 1, Vid: "local-video", DefaultPool: true}}}
	server := testServer(t, repository)
	server.iqiyi.decodeAPIBase = upstream.URL + "/decode"
	server.iqiyi.videoInfoAPIBase = upstream.URL + "/video"
	server.iqiyi.danmakuAPIBase = upstream.URL + "/bullet"

	create := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/admin/iqiyi/pools", strings.NewReader(`{"vid":"source-vid"}`))
	server.serveIqiyiAdmin(create, createRequest, "/api/admin/iqiyi/pools")
	if create.Code != http.StatusOK || len(repository.iqiyiPools) != 1 || len(repository.iqiyiData[1]) != 1 {
		t.Fatalf("create pool = %d %s pools=%#v data=%#v", create.Code, create.Body.String(), repository.iqiyiPools, repository.iqiyiData)
	}

	bind := httptest.NewRecorder()
	bindRequest := httptest.NewRequest(http.MethodPost, "/api/admin/videos/1/iqiyi-bindings", strings.NewReader(`{"poolId":1,"offset":2.5}`))
	server.serveVideoAdmin(bind, bindRequest, "/api/admin/videos/1/iqiyi-bindings")
	if bind.Code != http.StatusOK || len(repository.iqiyiBindings) != 1 {
		t.Fatalf("bind pool = %d %s bindings=%#v", bind.Code, bind.Body.String(), repository.iqiyiBindings)
	}

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/danmaku/dplayer/v3?id=local-video", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `3.75`) || !strings.Contains(response.Body.String(), `linked`) {
		t.Fatalf("merged response = %d %s", response.Code, response.Body.String())
	}

	detail := httptest.NewRecorder()
	server.serveVideoAdmin(detail, httptest.NewRequest(http.MethodGet, "/api/admin/videos/1", nil), "/api/admin/videos/1")
	if !strings.Contains(detail.Body.String(), `"iqiyiPoolCount":1`) || !strings.Contains(detail.Body.String(), `"iqiyiBindings":[`) {
		t.Fatalf("video detail = %s", detail.Body.String())
	}
}

func TestIqiyiDPlayerRouteKeepsExternalContract(t *testing.T) {
	payload := brotliPayload(t, `<danmu><data><entry><list><bulletInfo><content>hello</content><showTime>2.5</showTime></bulletInfo></list></entry></data></danmu>`)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/decode/"):
			_, _ = io.WriteString(w, `{"code":"0","data":"123456"}`)
		case strings.HasPrefix(r.URL.Path, "/video/"):
			_, _ = io.WriteString(w, `{"code":"A00000","data":{"durationSec":"1","displayBarrage":true}}`)
		case strings.HasPrefix(r.URL.Path, "/bullet/"):
			_, _ = w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	server := testServer(t, &fakeRepository{})
	server.iqiyi = testIqiyiClient(upstream.URL)
	server.iqiyi.refresh = server.refresh
	if _, _, err := server.iqiyi.PreparePool(context.Background(), "abc"); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/danmaku/dplayer/v3/iqiyi/?VID=abc", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Code int             `json:"code"`
		Data [][]interface{} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != 0 || len(body.Data) != 1 || len(body.Data[0]) != 5 || body.Data[0][0] != 2.5 || body.Data[0][1] != float64(0) || body.Data[0][2] != float64(16777215) || body.Data[0][3] != "" || body.Data[0][4] != "hello" {
		t.Fatalf("unexpected response: %s", response.Body.String())
	}

	missing := httptest.NewRecorder()
	server.Handler().ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/api/danmaku/dplayer/v3/iqiyi/", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing vid status = %d", missing.Code)
	}
}

func TestIqiyiVideoInfoFallbackAndDisabledDanmaku(t *testing.T) {
	requests := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch r.URL.Path {
		case "/decode/original":
			_, _ = io.WriteString(w, `{"code":"0","data":987654}`)
		case "/video/987654":
			_, _ = io.WriteString(w, `{"code":"A00001","data":"not found"}`)
		case "/video/original":
			_, _ = io.WriteString(w, `{"code":"A00000","data":{"durationSec":60,"displayBarrage":false}}`)
		default:
			t.Fatalf("unexpected request: %s", r.URL.RequestURI())
		}
	}))
	defer upstream.Close()

	data, err := readIqiyiAfterRefresh(testIqiyiClient(upstream.URL), context.Background(), "original")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 || requests != 3 {
		t.Fatalf("data=%#v requests=%d", data, requests)
	}
}

func TestParseIqiyiProtoDanmaku(t *testing.T) {
	item := iqiyiProtoStringField(2, "hello &amp; world")
	item = append(item, iqiyiProtoStringField(6, "3.75")...)
	item = append(item, iqiyiProtoStringField(8, "ff0000")...)
	block := iqiyiProtoStringField(1, "2")
	block = append(block, iqiyiProtoBytesField(2, item)...)
	payload := append(iqiyiProtoBytesField(6, block), 0)

	data, err := parseIqiyiProto(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 1 || data[0].Time != 3.75 || data[0].Text == nil || *data[0].Text != "hello & world" {
		t.Fatalf("unexpected protobuf data: %#v", data)
	}
	if _, err := parseIqiyiProto([]byte{0x32, 0x80}); err == nil {
		t.Fatal("truncated protobuf was accepted")
	}
}

func TestIqiyiSegmentURL(t *testing.T) {
	client := NewIqiyi(&fakeRepository{}, config.IqiyiSettings{})
	got := client.segmentURL("1078946400", 1)
	want := "https://cmts.iqiyi.com/bullet/64/00/1078946400_60_1_fc5e9d5c.br"
	if got != want {
		t.Fatalf("segment URL = %q, want %q", got, want)
	}
}

func TestIqiyiLive(t *testing.T) {
	vid := os.Getenv("IQIYI_LIVE_TEST_VID")
	if vid == "" {
		t.Skip("set IQIYI_LIVE_TEST_VID to run the upstream integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	client := NewIqiyi(&fakeRepository{}, config.IqiyiSettings{})
	data, err := readIqiyiAfterRefresh(client, ctx, vid)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("iqiyi upstream returned no danmaku")
	}
	t.Logf("received %d iqiyi danmaku", len(data))
}

func testIqiyiClient(baseURL string) *Iqiyi {
	client := NewIqiyi(&fakeRepository{}, config.IqiyiSettings{})
	client.decodeAPIBase = baseURL + "/decode"
	client.videoInfoAPIBase = baseURL + "/video"
	client.danmakuAPIBase = baseURL + "/bullet"
	return client
}

func brotliPayload(t *testing.T, value string) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := brotli.NewWriter(&output)
	if _, err := writer.Write([]byte(value)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func iqiyiProtoStringField(number int, value string) []byte {
	return iqiyiProtoBytesField(number, []byte(value))
}

func iqiyiProtoBytesField(number int, value []byte) []byte {
	result := appendIqiyiTestVarint(nil, uint64(number<<3|2))
	result = appendIqiyiTestVarint(result, uint64(len(value)))
	return append(result, value...)
}

func appendIqiyiTestVarint(target []byte, value uint64) []byte {
	for value >= 0x80 {
		target = append(target, byte(value)|0x80)
		value >>= 7
	}
	return append(target, byte(value))
}
