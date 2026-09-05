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
	data, err := client.Data(context.Background(), "abc")
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

	data, err := testIqiyiClient(upstream.URL).Data(context.Background(), "original")
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
	client := NewIqiyi()
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
	client := NewIqiyi()
	data, err := client.Data(ctx, vid)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("iqiyi upstream returned no danmaku")
	}
	t.Logf("received %d iqiyi danmaku", len(data))
}

func testIqiyiClient(baseURL string) *Iqiyi {
	client := NewIqiyi()
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
