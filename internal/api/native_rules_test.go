package api

import (
	"context"
	"encoding/json"
	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"git.hanada.info/tamersunion/danmaku/internal/store"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type nativeRuleTestRepository struct {
	*fakeRepository
	kind  string
	input store.NativeRuleInput
	err   error
}

func (f *nativeRuleTestRepository) NativeRules(context.Context, string) ([]store.NativeRule, error) {
	return []store.NativeRule{}, nil
}
func (f *nativeRuleTestRepository) CreateNativeRule(_ context.Context, kind string, input store.NativeRuleInput) (*store.NativeRuleResult, error) {
	f.kind = kind
	f.input = input
	return &store.NativeRuleResult{Replaced: 7}, f.err
}
func (f *nativeRuleTestRepository) DeleteNativeRule(_ context.Context, kind string, id int) (bool, error) {
	f.kind = kind
	return id == 1, nil
}

func TestNativeRuleManagementPermissionsAndScanFlag(t *testing.T) {
	base := &fakeRepository{}
	repo := &nativeRuleTestRepository{fakeRepository: base}
	server := testServer(t, base)
	server.repository = repo
	request := func(role int, method, path, body string, want int) {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		server.serveAdmin(w, r, path, session{Role: role})
		var response struct {
			Code int `json:"code"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || w.Code != 200 || response.Code != want {
			t.Fatalf("response=%d %s err=%v", w.Code, w.Body.String(), err)
		}
	}
	const path = "/api/admin/danmaku-rules/authors"
	request(0, http.MethodPost, path, `{"value":"a","replacement":"b","scanExisting":true}`, 401)
	if repo.kind != "" {
		t.Fatal("ordinary user reached rule creation")
	}
	for _, role := range []int{1, 2} {
		request(role, http.MethodPost, path, `{"value":"a","replacement":"b","scanExisting":true}`, 0)
		if repo.kind != "authors" || repo.input.Value != "a" || repo.input.Replacement != "b" || !repo.input.ScanExisting {
			t.Fatalf("request lost fields: %#v", repo)
		}
	}
	request(2, http.MethodPost, path, `{"value":"a","replacement":"b"}`, 0)
	if repo.input.ScanExisting {
		t.Fatal("historical replacement must be opt-in")
	}
	repo.err = store.ErrNativeRuleExists
	request(1, http.MethodPost, path, `{"value":"a","replacement":"b"}`, 1)
	request(2, http.MethodDelete, path+"/bad", "", 1)
	request(2, http.MethodDelete, path+"/2", "", 1)
	request(2, http.MethodDelete, path+"/1", "", 0)
	request(2, http.MethodGet, path, "", 0)
}

type deniedSubmissionRepository struct{ *fakeRepository }

func (f *deniedSubmissionRepository) Insert(context.Context, string, domain.DanmakuData, net.IP, domain.Referer) (bool, error) {
	return false, store.ErrSubmissionDenied
}

func TestBlockedSubmissionUsesGenericFailureAcrossFormats(t *testing.T) {
	base := &fakeRepository{}
	server := testServer(t, base)
	server.repository = &deniedSubmissionRepository{base}
	for _, tt := range []struct{ path, body string }{
		{"/api/danmaku/dplayer/v3", `{"id":"v","text":"x","author":"a"}`},
		{"/api/danmaku/v1", `{"id":"v","text":"x","author":"a"}`},
		{"/api/danmaku/artplayer/v1", `{"id":"v","text":"x","color":"#ffffff"}`},
	} {
		t.Run(tt.path, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			r.Header.Set("Referer", "https://example.com/player")
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			server.Handler().ServeHTTP(w, r)
			if w.Code != 200 || strings.TrimSpace(w.Body.String()) != `{"code":1,"data":null}` {
				t.Fatalf("response=%d %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestRequestIPDoesNotTrustBodyOverConnection(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.RemoteAddr = "192.0.2.1:1234"
	if ip := requestIP(r, net.ParseIP("198.51.100.1")); ip.String() != "192.0.2.1" {
		t.Fatalf("body bypass: %v", ip)
	}
	r.Header.Set("X-Real-IP", "203.0.113.1")
	if ip := requestIP(r, nil); ip.String() != "203.0.113.1" {
		t.Fatalf("proxy address=%v", ip)
	}
}
