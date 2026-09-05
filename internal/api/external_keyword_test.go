package api

import (
	"context"
	"encoding/json"
	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type keywordTestRepository struct {
	*fakeRepository
	keywords []domain.ExternalKeyword
}

func (f *keywordTestRepository) ExternalKeywords(context.Context) ([]domain.ExternalKeyword, error) {
	return f.keywords, nil
}
func (f *keywordTestRepository) CreateExternalKeyword(_ context.Context, pool *string, keyword string) (*domain.ExternalKeyword, error) {
	item := domain.ExternalKeyword{ID: len(f.keywords) + 1, PoolID: pool, Keyword: keyword}
	f.keywords = append(f.keywords, item)
	return &item, nil
}
func (f *keywordTestRepository) DeleteExternalKeyword(_ context.Context, id int) (bool, error) {
	for i, item := range f.keywords {
		if item.ID == id {
			f.keywords = append(f.keywords[:i], f.keywords[i+1:]...)
			return true, nil
		}
	}
	return false, nil
}

func TestExternalKeywordRoutes(t *testing.T) {
	const poolID = "00000000-0000-4000-8000-000000000001"
	base := &fakeRepository{externalPools: []domain.ExternalPool{{ID: poolID, Name: "manual"}}}
	repo := &keywordTestRepository{fakeRepository: base}
	server := testServer(t, base)
	server.repository = repo
	request := func(method, path, body string, want int) {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		server.serveExternalAdmin(w, r, path)
		var result struct {
			Code int `json:"code"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || w.Code != 200 || result.Code != want {
			t.Fatalf("%s %s = %d %s, err=%v", method, path, w.Code, w.Body.String(), err)
		}
	}
	const path = "/api/admin/external/keywords"
	request(http.MethodPost, path, `{"keyword":"  spam  ","poolId":null}`, 0)
	request(http.MethodPost, path, `{"keyword":"pool-only","poolId":"`+poolID+`"}`, 0)
	if len(repo.keywords) != 2 || repo.keywords[0].PoolID != nil || repo.keywords[0].Keyword != "spam" || *repo.keywords[1].PoolID != poolID {
		t.Fatalf("scopes = %#v", repo.keywords)
	}
	request(http.MethodPost, path, `{"keyword":"   "}`, 1)
	request(http.MethodPost, path, `{"keyword":"spam","poolId":"missing"}`, 1)
	request(http.MethodGet, path, "", 0)
	request(http.MethodDelete, path+"/1", "", 0)
	request(http.MethodDelete, path+"/1", "", 1)
	request(http.MethodDelete, path+"/bad", "", 1)
	if len(repo.keywords) != 1 || repo.keywords[0].Keyword != "pool-only" {
		t.Fatalf("delete affected other scope: %#v", repo.keywords)
	}
}
