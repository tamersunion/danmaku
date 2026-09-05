package api

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	discardInsert bool
	verifyOK      bool
	verifyUID     int
	verifyRole    int
	casUser       *domain.User
	casCreated    bool
	casProfile    domain.CASProfile
	users         []domain.User
	updatedUser   store.UserUpdate
	statusUserID  int
	deletedUserID int
}

func (f *fakeRepository) Initialize(context.Context) error { return nil }
func (f *fakeRepository) Close()                           {}
func (f *fakeRepository) QueryByVid(context.Context, string) ([]domain.DanmakuData, error) {
	return f.data, nil
}
func (f *fakeRepository) Insert(_ context.Context, vid string, data domain.DanmakuData, ip net.IP, referer domain.Referer) (bool, error) {
	if f.discardInsert {
		return false, nil
	}
	f.insertedVid, f.insertedData, f.insertedIP, f.insertedRefer = vid, data, ip, referer
	return true, nil
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
func (f *fakeRepository) User(_ context.Context, id int) (*domain.User, error) {
	for index := range f.users {
		if f.users[index].ID == id {
			return &f.users[index], nil
		}
	}
	return nil, nil
}
func (f *fakeRepository) Users(context.Context, store.UserFilter) (domain.Page[domain.User], error) {
	return domain.Page[domain.User]{Total: len(f.users), List: f.users}, nil
}
func (f *fakeRepository) CreateUser(_ context.Context, input store.UserCreate) (*domain.User, error) {
	return &domain.User{ID: 10, Name: input.Name, Role: input.Role, Enabled: true}, nil
}
func (f *fakeRepository) UpdateUser(_ context.Context, id int, input store.UserUpdate) (*domain.User, error) {
	f.updatedUser = input
	return &domain.User{ID: id, Name: input.Name, Role: input.Role, Enabled: true}, nil
}
func (f *fakeRepository) SetUserEnabled(_ context.Context, id int, _ bool) (bool, error) {
	f.statusUserID = id
	return true, nil
}
func (f *fakeRepository) DeleteUser(_ context.Context, id int) (bool, error) {
	f.deletedUserID = id
	return true, nil
}
func (f *fakeRepository) UpsertCASUser(_ context.Context, profile domain.CASProfile, _ int, _ bool) (*domain.User, bool, error) {
	f.casProfile = profile
	if f.casUser == nil {
		f.casUser = &domain.User{ID: 1, Name: profile.UserName, Role: 1}
	}
	return f.casUser, f.casCreated, nil
}
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

func testServerWithConfig(t *testing.T, repository store.Repository, cfg config.Config) *Server {
	t.Helper()
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
	testServer(t, repository).Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/danmaku/dplayer/v3?id=video", nil))
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
	testServer(t, repository).Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/danmaku/v1/video.xml", nil))
	if response.Header().Get("Content-Type") != "application/xml; charset=utf-8" || !strings.Contains(response.Body.String(), `<d p="2.5,1,25,255,123,0,alice,123">hello &amp; goodbye</d>`) {
		t.Fatalf("unexpected XML: %s", response.Body.String())
	}
}

func TestDPlayerPostUsesExternalContract(t *testing.T) {
	repository := &fakeRepository{}
	request := httptest.NewRequest(http.MethodPost, "/api/danmaku/dplayer/v3", strings.NewReader(`{"id":"video","time":3.5,"type":2,"color":255,"author":"42","text":"hello"}`))
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

func TestDiscardedDPlayerPostStillUsesSuccessContract(t *testing.T) {
	repository := &fakeRepository{discardInsert: true}
	request := httptest.NewRequest(http.MethodPost, "/api/danmaku/dplayer/v3", strings.NewReader(`{"id":"video","time":3.5,"type":2,"color":255,"author":"42","text":"hello"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Referer", "https://example.com/watch?id=7")
	request.Header.Set("X-Real-IP", "203.0.113.8")
	response := httptest.NewRecorder()

	testServer(t, repository).Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Body.String() != "{\"code\":0,\"data\":null}\n" {
		t.Fatalf("discarded insert response = %d %q", response.Code, response.Body.String())
	}
	if repository.insertedVid != "" {
		t.Fatalf("discarded insert was recorded as %q", repository.insertedVid)
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
	request := httptest.NewRequest(http.MethodGet, "/api/admin/danmakulist/vids", nil)
	request.AddCookie(sessionCookie)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Body.String() != "{\"code\":0,\"data\":[\"video\"]}\n" {
		t.Fatalf("unexpected admin response: %s", response.Body.String())
	}
}

func TestLoginSuccessPreservesLegacyJSONFieldOrder(t *testing.T) {
	repository := &fakeRepository{verifyOK: true, verifyUID: 1, verifyRole: 1}
	server := testServer(t, repository)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(`{"name":"Hanada","password":"hash"}`))

	server.Handler().ServeHTTP(response, request)

	const want = `{"code":0,"data":{"url":"/","uid":1}}` + "\n"
	if response.Code != http.StatusOK || response.Body.String() != want {
		t.Fatalf("login response = %d %q, want %d %q", response.Code, response.Body.String(), http.StatusOK, want)
	}
}

func TestNativeCASAuthCreatesSessionAndRestoresProfile(t *testing.T) {
	casServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cas/application/serviceValidate" || r.URL.Query().Get("ticket") != "ST-1" {
			t.Fatalf("unexpected CAS validation request: %s", r.URL.String())
		}
		if got := r.URL.Query().Get("service"); got != "https://danmaku.example/cas/auth?returnTo=%2Fdanmaku%2Findex" {
			t.Fatalf("service = %q", got)
		}
		_, _ = io.WriteString(w, `<cas:serviceResponse xmlns:cas="http://www.yale.edu/tp/cas"><cas:authenticationSuccess><cas:user>cas-subject</cas:user><cas:attributes><cas:username>hanada</cas:username><cas:displayName>花田</cas:displayName><cas:email>hanada@example.com</cas:email><cas:avatar>https://example.com/avatar.png</cas:avatar></cas:attributes></cas:authenticationSuccess></cas:serviceResponse>`)
	}))
	defer casServer.Close()

	repository := &fakeRepository{casUser: &domain.User{ID: 9, Name: "hanada", Role: 1}, casCreated: true}
	cfg := config.Config{
		KestrelSettings: config.ListenerSettings{Host: "127.0.0.1", Port: 8080},
		Admin:           config.AdminSettings{Password: "secret", MaxAge: 60},
		Bilibili:        config.BilibiliSettings{CIDCacheMinutes: 1, DataCacheMinutes: 1},
		CAS: config.CASSettings{
			Enabled: true, BaseURL: casServer.URL + "/cas/application", PublicURL: "https://danmaku.example",
			AutoCreateUsers: true, DefaultRole: 1, SessionMaxAgeMinutes: 60, RequestTimeoutSeconds: 2,
		},
	}
	server := testServerWithConfig(t, repository, cfg)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/cas/auth?returnTo=%2Fdanmaku%2Findex&ticket=ST-1", nil))
	if response.Code != http.StatusFound || response.Header().Get("Location") != "/danmaku/index" {
		t.Fatalf("unexpected CAS callback: %d %s", response.Code, response.Header().Get("Location"))
	}
	if repository.casProfile.Subject != "cas-subject" || repository.casProfile.UserName != "hanada" || repository.casProfile.DisplayName != "花田" {
		t.Fatalf("unexpected provisioned profile: %#v", repository.casProfile)
	}
	var authCookie *http.Cookie
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == sessionCookie {
			authCookie = cookie
		}
	}
	if authCookie == nil {
		t.Fatal("CAS callback did not set DCookie")
	}

	sessionRequest := httptest.NewRequest(http.MethodGet, "/api/admin/session", nil)
	sessionRequest.AddCookie(authCookie)
	sessionResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(sessionResponse, sessionRequest)
	var restored struct {
		Code int `json:"code"`
		Data struct {
			ID       int    `json:"id"`
			Name     string `json:"name"`
			Provider string `json:"provider"`
		} `json:"data"`
	}
	if err := json.Unmarshal(sessionResponse.Body.Bytes(), &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Code != 0 || restored.Data.ID != 9 || restored.Data.Name != "花田" || restored.Data.Provider != "cas" {
		t.Fatalf("unexpected restored session: %s", sessionResponse.Body.String())
	}
	if strings.Contains(sessionResponse.Body.String(), `"casEnabled"`) || strings.Contains(sessionResponse.Body.String(), `"profileEditable"`) {
		t.Fatalf("legacy session response shape changed: %s", sessionResponse.Body.String())
	}

	changeRequest := httptest.NewRequest(http.MethodPost, "/api/admin/user/changepassword", strings.NewReader(`{}`))
	changeRequest.AddCookie(authCookie)
	changeResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(changeResponse, changeRequest)
	if changeResponse.Code != http.StatusForbidden {
		t.Fatalf("CAS password change status = %d", changeResponse.Code)
	}
}

func TestCASLoginRejectsExternalReturnURL(t *testing.T) {
	repository := &fakeRepository{}
	cfg := config.Config{
		KestrelSettings: config.ListenerSettings{Host: "127.0.0.1", Port: 8080},
		Admin:           config.AdminSettings{Password: "secret", MaxAge: 60},
		Bilibili:        config.BilibiliSettings{CIDCacheMinutes: 1, DataCacheMinutes: 1},
		CAS: config.CASSettings{
			Enabled: true, BaseURL: "https://cas.example/cas/application", PublicURL: "https://danmaku.example",
			AutoCreateUsers: true, DefaultRole: 1, SessionMaxAgeMinutes: 60, RequestTimeoutSeconds: 2,
		},
	}
	response := httptest.NewRecorder()
	testServerWithConfig(t, repository, cfg).Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/cas/login?returnTo=https://evil.example/", nil))
	location, err := url.Parse(response.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusFound || location.Host != "cas.example" || !strings.Contains(location.Query().Get("service"), "returnTo=%2Fdanmaku%2Findex") {
		t.Fatalf("unsafe return URL was not replaced: %d %s", response.Code, location)
	}
}

func TestAdminUnauthorizedContract(t *testing.T) {
	response := httptest.NewRecorder()
	testServer(t, &fakeRepository{}).Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/admin/danmakulist/vids", nil))
	if response.Code != http.StatusOK || response.Body.String() != "{\"code\":401,\"data\":{\"desc\":\"没有权限\"}}\n" {
		t.Fatalf("unexpected unauthorized response: %d %s", response.Code, response.Body.String())
	}
}

func TestGeneralUserCanOpenProfileButNotDanmakuManagement(t *testing.T) {
	repository := &fakeRepository{
		verifyOK: true, verifyUID: 7, verifyRole: 3,
		users: []domain.User{{ID: 7, Name: "viewer", Role: 3, Enabled: true}},
	}
	server := testServer(t, repository)
	loginResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(loginResponse, httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(`{"name":"viewer","password":"hash"}`)))
	var cookie *http.Cookie
	for _, candidate := range loginResponse.Result().Cookies() {
		if candidate.Name == sessionCookie {
			cookie = candidate
		}
	}
	if cookie == nil {
		t.Fatal("missing general-user session cookie")
	}

	managementRequest := httptest.NewRequest(http.MethodGet, "/api/admin/danmakulist/vids", nil)
	managementRequest.AddCookie(cookie)
	managementResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(managementResponse, managementRequest)
	if managementResponse.Body.String() != "{\"code\":401,\"data\":{\"desc\":\"没有权限\"}}\n" {
		t.Fatalf("general user management response = %s", managementResponse.Body.String())
	}

	profileRequest := httptest.NewRequest(http.MethodGet, "/api/admin/user/user?uid=7", nil)
	profileRequest.AddCookie(cookie)
	profileResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(profileResponse, profileRequest)
	if profileResponse.Code != http.StatusOK || !strings.Contains(profileResponse.Body.String(), `"name":"viewer"`) {
		t.Fatalf("general user profile response = %d %s", profileResponse.Code, profileResponse.Body.String())
	}
	if strings.Contains(profileResponse.Body.String(), `"enabled"`) {
		t.Fatalf("legacy profile response shape changed: %s", profileResponse.Body.String())
	}
}

func TestDanmakuManagerCanManageDanmakuButNotUsers(t *testing.T) {
	repository := &fakeRepository{
		verifyOK: true, verifyUID: 2, verifyRole: 2,
		vids: []string{"video"}, users: []domain.User{{ID: 7, Name: "viewer", Role: 3, Enabled: true}},
	}
	server := testServer(t, repository)
	loginResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(loginResponse, httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(`{"name":"moderator","password":"hash"}`)))
	var cookie *http.Cookie
	for _, candidate := range loginResponse.Result().Cookies() {
		if candidate.Name == sessionCookie {
			cookie = candidate
		}
	}

	danmakuRequest := httptest.NewRequest(http.MethodGet, "/api/admin/danmakulist/vids", nil)
	danmakuRequest.AddCookie(cookie)
	danmakuResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(danmakuResponse, danmakuRequest)
	if danmakuResponse.Body.String() != "{\"code\":0,\"data\":[\"video\"]}\n" {
		t.Fatalf("danmaku manager response = %s", danmakuResponse.Body.String())
	}

	usersRequest := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	usersRequest.AddCookie(cookie)
	usersResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(usersResponse, usersRequest)
	if usersResponse.Body.String() != "{\"code\":401,\"data\":{\"desc\":\"没有权限\"}}\n" {
		t.Fatalf("danmaku manager users response = %s", usersResponse.Body.String())
	}

	profileRequest := httptest.NewRequest(http.MethodGet, "/api/admin/user/user?uid=7", nil)
	profileRequest.AddCookie(cookie)
	profileResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(profileResponse, profileRequest)
	if profileResponse.Code != http.StatusForbidden {
		t.Fatalf("danmaku manager other profile status = %d", profileResponse.Code)
	}
}

func TestAdministratorCanListManagedUsers(t *testing.T) {
	repository := &fakeRepository{
		verifyOK: true, verifyUID: 1, verifyRole: 1,
		users: []domain.User{
			{ID: 1, Name: "admin", Role: 1, Enabled: true},
			{ID: 2, Name: "moderator", Role: 2, Enabled: true},
			{ID: 3, Name: "viewer", Role: 3, Enabled: true},
		},
	}
	server := testServer(t, repository)
	loginResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(loginResponse, httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(`{"name":"admin","password":"hash"}`)))
	var cookie *http.Cookie
	for _, candidate := range loginResponse.Result().Cookies() {
		if candidate.Name == sessionCookie {
			cookie = candidate
		}
	}
	request := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"role":"administrator"`) || !strings.Contains(response.Body.String(), `"role":"danmaku_manager"`) || !strings.Contains(response.Body.String(), `"role":"user"`) {
		t.Fatalf("managed users response = %d %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), `"phoneNumber"`) {
		t.Fatalf("managed users response exposed legacy phone field: %s", response.Body.String())
	}
}

func TestCASEnabledDisablesLocalProfileMutations(t *testing.T) {
	repository := &fakeRepository{verifyOK: true, verifyUID: 2, verifyRole: 2}
	cfg := config.Config{
		KestrelSettings: config.ListenerSettings{Host: "127.0.0.1", Port: 8080},
		Admin:           config.AdminSettings{Password: "secret", MaxAge: 60},
		Bilibili:        config.BilibiliSettings{CIDCacheMinutes: 1, DataCacheMinutes: 1},
		CAS: config.CASSettings{
			Enabled: true, BaseURL: "https://cas.example/cas/application", PublicURL: "https://danmaku.example",
			AutoCreateUsers: true, DefaultRole: 3, SessionMaxAgeMinutes: 60, RequestTimeoutSeconds: 2,
		},
	}
	server := testServerWithConfig(t, repository, cfg)
	loginResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(loginResponse, httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(`{"name":"admin","password":"hash"}`)))
	var cookie *http.Cookie
	for _, candidate := range loginResponse.Result().Cookies() {
		if candidate.Name == sessionCookie {
			cookie = candidate
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/api/admin/user/changepassword", strings.NewReader(`{"uid":2,"oldP":"old","newP":"new"}`))
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("password mutation status = %d", response.Code)
	}
}

func TestCASEnabledManagedUserAllowsRoleOnly(t *testing.T) {
	subject := "cas-viewer"
	repository := &fakeRepository{
		verifyOK: true, verifyUID: 1, verifyRole: 1,
		users: []domain.User{{ID: 7, Name: "viewer", Role: 3, Enabled: true, CASSubject: &subject}},
	}
	cfg := config.Config{
		KestrelSettings: config.ListenerSettings{Host: "127.0.0.1", Port: 8080},
		Admin:           config.AdminSettings{Password: "secret", MaxAge: 60},
		Bilibili:        config.BilibiliSettings{CIDCacheMinutes: 1, DataCacheMinutes: 1},
		CAS: config.CASSettings{
			Enabled: true, BaseURL: "https://cas.example/cas/application", PublicURL: "https://danmaku.example",
			AutoCreateUsers: true, DefaultRole: 3, SessionMaxAgeMinutes: 60, RequestTimeoutSeconds: 2,
		},
	}
	server := testServerWithConfig(t, repository, cfg)
	loginResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(loginResponse, httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(`{"name":"admin","password":"hash"}`)))
	var cookie *http.Cookie
	for _, candidate := range loginResponse.Result().Cookies() {
		if candidate.Name == sessionCookie {
			cookie = candidate
		}
	}

	roleRequest := httptest.NewRequest(http.MethodPut, "/api/admin/users/7", strings.NewReader(`{"role":"danmaku_manager"}`))
	roleRequest.AddCookie(cookie)
	roleResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(roleResponse, roleRequest)
	if roleResponse.Code != http.StatusOK || repository.updatedUser.Role != 2 {
		t.Fatalf("role-only update = %d %s, role %d", roleResponse.Code, roleResponse.Body.String(), repository.updatedUser.Role)
	}

	profileRequest := httptest.NewRequest(http.MethodPut, "/api/admin/users/7", strings.NewReader(`{"role":"danmaku_manager","name":"changed"}`))
	profileRequest.AddCookie(cookie)
	profileResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(profileResponse, profileRequest)
	if profileResponse.Code != http.StatusConflict {
		t.Fatalf("CAS profile update status = %d, body %s", profileResponse.Code, profileResponse.Body.String())
	}
}

func TestSignalRNegotiationRoute(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/live/danmaku/negotiate?negotiateVersion=1", nil)
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
	request := httptest.NewRequest(http.MethodOptions, "/api/danmaku/v1", nil)
	request.Header.Set("Origin", "https://player.example")
	request.Header.Set("Access-Control-Request-Headers", "content-type")
	testServer(t, &fakeRepository{}).Handler().ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || response.Header().Get("Access-Control-Allow-Origin") != "*" || response.Header().Get("Access-Control-Allow-Headers") != "content-type" {
		t.Fatalf("unexpected CORS response: %#v", response.Header())
	}
}

func TestLegacyDanmakuAPIPathIsRemoved(t *testing.T) {
	response := httptest.NewRecorder()
	testServer(t, &fakeRepository{}).Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/danmu/v1?id=video", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("legacy API status = %d, want %d", response.Code, http.StatusNotFound)
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
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/danmaku/v1/bilibili/danmaku.json?cid=99", nil))
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
	client1 := newSignalRClient(t, ctx, server.URL+"/api/live/danmaku", receiver1)
	defer client1.Stop()
	client2 := newSignalRClient(t, ctx, server.URL+"/api/live/danmaku", receiver2)
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
