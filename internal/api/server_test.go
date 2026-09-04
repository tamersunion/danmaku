package api

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"git.hanada.info/tamersunion/danmaku/internal/config"
	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"git.hanada.info/tamersunion/danmaku/internal/store"
	"github.com/philippseith/signalr"
)

type fakeRepository struct {
	data          []domain.DanmakuData
	vids          []string
	insertedVid   string
	insertedData  domain.DanmakuData
	insertedIP    net.IP
	insertedRefer domain.Referer
	verifyOK      bool
	verifyUID     int
	verifyRole    int
}

func (f *fakeRepository) Initialize(context.Context) error { return nil }
func (f *fakeRepository) Close()                           {}
func (f *fakeRepository) QueryByVid(context.Context, string) ([]domain.DanmakuData, error) {
	return f.data, nil
}
func (f *fakeRepository) Insert(_ context.Context, vid string, data domain.DanmakuData, ip net.IP, referer domain.Referer) error {
	f.insertedVid, f.insertedData, f.insertedIP, f.insertedRefer = vid, data, ip, referer
	return nil
}
func (f *fakeRepository) List(context.Context, string, int, int, bool) (domain.Page[domain.Danmaku], error) {
	return domain.Page[domain.Danmaku]{List: []domain.Danmaku{}}, nil
}
func (f *fakeRepository) Search(context.Context, store.SearchFilter) (domain.Page[domain.Danmaku], error) {
	return domain.Page[domain.Danmaku]{List: []domain.Danmaku{}}, nil
}
func (f *fakeRepository) Vids(context.Context) ([]string, error) { return f.vids, nil }
func (f *fakeRepository) Get(context.Context, string) (*domain.Danmaku, error) {
	return nil, nil
}
func (f *fakeRepository) Edit(context.Context, string, domain.DanmakuData, bool) (*domain.Danmaku, error) {
	return nil, nil
}
func (f *fakeRepository) Delete(context.Context, string) (bool, error) { return true, nil }
func (f *fakeRepository) VerifyPassword(context.Context, string, string) (bool, int, int, error) {
	return f.verifyOK, f.verifyUID, f.verifyRole, nil
}
func (f *fakeRepository) ChangePassword(context.Context, int, string, string) (bool, error) {
	return true, nil
}
func (f *fakeRepository) ChangeUserInfo(context.Context, int, string, *string, *string) (bool, error) {
	return true, nil
}
func (f *fakeRepository) User(context.Context, int) (*domain.User, error) { return nil, nil }
func (f *fakeRepository) Cache(ctx context.Context, _ string, _ time.Duration, factory func(context.Context) ([]byte, error)) ([]byte, error) {
	return factory(ctx)
}

func testServer(t *testing.T, repository store.Repository) *Server {
	t.Helper()
	cfg := config.Config{
		KestrelSettings: config.ListenerSettings{Host: "127.0.0.1", Port: 8080},
		WithOrigins:     []string{"*"}, LiveWithOrigins: []string{"http://localhost"},
		AdminWithOrigins: []string{"http://localhost:5000"},
		Admin:            config.AdminSettings{Password: "secret", MaxAge: 60},
		Bilibili:         config.BilibiliSettings{CIDCacheMinutes: 1, DataCacheMinutes: 1},
	}
	server, err := New(context.Background(), cfg, repository, nil)
	if err != nil {
		t.Fatal(err)
	}
	return server
}

func TestDPlayerGetContract(t *testing.T) {
	text := "<hello>"
	repository := &fakeRepository{data: []domain.DanmakuData{{Time: 1.5, Mode: 5, Color: 16777215, Author: "alice", Text: &text}}}
	response := httptest.NewRecorder()
	testServer(t, repository).Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/danmu/dplayer/v3?id=video", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	var body struct {
		Code int     `json:"code"`
		Data [][]any `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != 0 || len(body.Data) != 1 || body.Data[0][1] != float64(1) || body.Data[0][4] != "&lt;hello&gt;" {
		t.Fatalf("unexpected response: %s", response.Body.String())
	}
}

func TestCommonXMLContract(t *testing.T) {
	text := "hello & goodbye"
	repository := &fakeRepository{data: []domain.DanmakuData{{Time: 2.5, Mode: 1, Size: 25, Color: 255, Timestamp: 123, Pool: 0, Author: "alice", Text: &text}}}
	response := httptest.NewRecorder()
	testServer(t, repository).Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/danmu/v1/video.xml", nil))
	if response.Header().Get("Content-Type") != "application/xml; charset=utf-8" || !strings.Contains(response.Body.String(), `<d p="2.5,1,25,255,123,0,alice,123">hello &amp; goodbye</d>`) {
		t.Fatalf("unexpected XML: %s", response.Body.String())
	}
}

func TestDPlayerPostUsesExternalContract(t *testing.T) {
	repository := &fakeRepository{}
	request := httptest.NewRequest(http.MethodPost, "/api/danmu/dplayer/v3", strings.NewReader(`{"id":"video","time":3.5,"type":2,"color":255,"author":"42","text":"hello"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Referer", "https://example.com/watch?id=7")
	request.Header.Set("X-Real-IP", "203.0.113.8")
	response := httptest.NewRecorder()
	testServer(t, repository).Handler().ServeHTTP(response, request)
	if repository.insertedVid != "video" || repository.insertedData.Mode != 4 || repository.insertedData.AuthorID != 42 || repository.insertedRefer.Host != "example.com" || repository.insertedIP.String() != "203.0.113.8" {
		t.Fatalf("unexpected insert: %#v %#v %s", repository.insertedData, repository.insertedRefer, repository.insertedIP)
	}
	if response.Body.String() != "{\"code\":0,\"data\":null}\n" {
		t.Fatalf("unexpected response: %s", response.Body.String())
	}
}

func TestAdminCookieAuthentication(t *testing.T) {
	repository := &fakeRepository{verifyOK: true, verifyUID: 7, verifyRole: 1, vids: []string{"video"}}
	server := testServer(t, repository)
	loginResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(loginResponse, httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(`{"name":"admin","password":"hash"}`)))
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status = %d", loginResponse.Code)
	}
	var sessionCookie *http.Cookie
	for _, cookie := range loginResponse.Result().Cookies() {
		if cookie.Name == "DCookie" {
			sessionCookie = cookie
		}
	}
	if sessionCookie == nil {
		t.Fatal("missing DCookie")
	}
	request := httptest.NewRequest(http.MethodGet, "/api/admin/danmulist/vids", nil)
	request.AddCookie(sessionCookie)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Body.String() != "{\"code\":0,\"data\":[\"video\"]}\n" {
		t.Fatalf("unexpected admin response: %s", response.Body.String())
	}
}

func TestAdminUnauthorizedContract(t *testing.T) {
	response := httptest.NewRecorder()
	testServer(t, &fakeRepository{}).Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/admin/danmulist/vids", nil))
	if response.Code != http.StatusOK || response.Body.String() != "{\"code\":401,\"data\":{\"desc\":\"没有权限\"}}\n" {
		t.Fatalf("unexpected unauthorized response: %d %s", response.Code, response.Body.String())
	}
}

func TestSignalRNegotiationRoute(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/live/danmu/negotiate?negotiateVersion=1", nil)
	testServer(t, &fakeRepository{}).Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	body, err := io.ReadAll(response.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"connectionId"`) || !strings.Contains(string(body), `"availableTransports"`) {
		t.Fatalf("unexpected negotiate response: %s", body)
	}
}

func TestPublicCORSContract(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodOptions, "/api/danmu/v1", nil)
	request.Header.Set("Origin", "https://player.example")
	request.Header.Set("Access-Control-Request-Headers", "content-type")
	testServer(t, &fakeRepository{}).Handler().ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || response.Header().Get("Access-Control-Allow-Origin") != "*" || response.Header().Get("Access-Control-Allow-Headers") != "content-type" {
		t.Fatalf("unexpected CORS response: %#v", response.Header())
	}
}

func TestBilibiliJSONCompatibility(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = io.WriteString(w, `<i><d p="1.25,5,25,16777215,123,0,alice,9">hello</d></i>`)
	}))
	defer upstream.Close()

	server := testServer(t, &fakeRepository{})
	server.bilibili.baseURL = upstream.URL
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/danmu/v1/bilibili/danmu.json?cid=99", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"time":1.25`) || !strings.Contains(response.Body.String(), `"author":"alice"`) {
		t.Fatalf("unexpected Bilibili response: %d %s", response.Code, response.Body.String())
	}
}

type liveReceiver struct {
	signalr.Receiver
	messages chan [2]string
}

func (r *liveReceiver) ReceiveMessage(user, message string) {
	r.messages <- [2]string{user, message}
}

func TestSignalRGroupContract(t *testing.T) {
	server := httptest.NewServer(testServer(t, &fakeRepository{}).Handler())
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	receiver1 := &liveReceiver{messages: make(chan [2]string, 1)}
	receiver2 := &liveReceiver{messages: make(chan [2]string, 1)}
	client1 := newSignalRClient(t, ctx, server.URL+"/api/live/danmu", receiver1)
	defer client1.Stop()
	client2 := newSignalRClient(t, ctx, server.URL+"/api/live/danmu", receiver2)
	defer client2.Stop()

	waitInvocation(t, ctx, client1.Invoke("Connection", "room"))
	waitInvocation(t, ctx, client2.Invoke("Connection", "room"))
	waitInvocation(t, ctx, client1.Invoke("SendMessage", "room", "alice", "hello"))

	select {
	case message := <-receiver2.messages:
		if message != [2]string{"alice", "hello"} {
			t.Fatalf("unexpected live message: %#v", message)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for SignalR group message")
	}
	select {
	case message := <-receiver1.messages:
		t.Fatalf("sender unexpectedly received its own message: %#v", message)
	case <-time.After(200 * time.Millisecond):
	}
}

func newSignalRClient(t *testing.T, ctx context.Context, endpoint string, receiver *liveReceiver) signalr.Client {
	t.Helper()
	client, err := signalr.NewClient(ctx, signalr.WithHttpConnection(ctx, endpoint), signalr.WithReceiver(receiver))
	if err != nil {
		t.Fatal(err)
	}
	client.Start()
	return client
}

func waitInvocation(t *testing.T, ctx context.Context, invocation <-chan signalr.InvokeResult) {
	t.Helper()
	select {
	case result, ok := <-invocation:
		if !ok {
			return
		}
		if result.Error != nil {
			t.Fatal(result.Error)
		}
	case <-ctx.Done():
		t.Fatal("SignalR invocation timed out")
	}
}
