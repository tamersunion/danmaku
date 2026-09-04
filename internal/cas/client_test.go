package cas

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestValidateCAS30Attributes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/serviceValidate" || r.URL.Query().Get("ticket") != "ST-1" || r.URL.Query().Get("service") != "https://app.example/cas/callback" {
			t.Fatalf("unexpected validation request: %s", r.URL.String())
		}
		_, _ = w.Write([]byte(`<cas:serviceResponse xmlns:cas="http://www.yale.edu/tp/cas"><cas:authenticationSuccess><cas:user>subject-1</cas:user><cas:attributes><cas:username>hanada</cas:username><cas:displayName>Hanada</cas:displayName><cas:email>hanada@example.com</cas:email><cas:avatarUrl>https://example.com/avatar.png</cas:avatarUrl></cas:attributes></cas:authenticationSuccess></cas:serviceResponse>`))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "", "", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := client.Validate(context.Background(), "ST-1", "https://app.example/cas/callback")
	if err != nil {
		t.Fatal(err)
	}
	if identity.Subject != "subject-1" || identity.UserName != "hanada" || identity.DisplayName != "Hanada" || identity.Email != "hanada@example.com" || identity.Avatar == "" {
		t.Fatalf("unexpected identity: %#v", identity)
	}
}

func TestValidateGenericAttributesAndValidationHost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "cas.internal" {
			t.Fatalf("Host = %q", r.Host)
		}
		if r.URL.Path != "/serviceValidate" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`<serviceResponse><authenticationSuccess><user>subject</user><userAttributes><attribute name="username" value="alice"/><attribute name="display_name">Alice</attribute></userAttributes></authenticationSuccess></serviceResponse>`))
	}))
	defer server.Close()
	client, err := NewClient("https://cas.example/application", server.URL, "cas.internal", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := client.Validate(context.Background(), "ST-2", "https://app.example/cas/auth")
	if err != nil {
		t.Fatal(err)
	}
	if identity.UserName != "alice" || identity.DisplayName != "Alice" {
		t.Fatalf("unexpected identity: %#v", identity)
	}
}

func TestInvalidAndUnsafeCASResponses(t *testing.T) {
	tests := []struct {
		name string
		body string
		want error
	}{
		{name: "failure", body: `<serviceResponse><authenticationFailure code="INVALID_TICKET">bad</authenticationFailure></serviceResponse>`, want: ErrTicketInvalid},
		{name: "doctype", body: `<!DOCTYPE foo><serviceResponse/>`, want: nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(test.body)) }))
			defer server.Close()
			client, err := NewClient(server.URL, "", "", time.Second)
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Validate(context.Background(), "ST", "https://app.example/cas/callback")
			if test.want != nil && !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			if test.want == nil && err == nil {
				t.Fatal("expected unsafe response to fail")
			}
		})
	}
}

func TestCASURLsPreserveApplicationPath(t *testing.T) {
	client, err := NewClient("https://cas.example/cas/owner/app", "", "", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	login, _ := url.Parse(client.LoginURL("https://app.example/cas/callback?returnTo=%2Fdanmaku%2Findex"))
	if login.Path != "/cas/owner/app/login" || login.Query().Get("service") == "" {
		t.Fatalf("unexpected login URL: %s", login)
	}
	logout, _ := url.Parse(client.LogoutURL("https://app.example/"))
	if logout.Path != "/cas/owner/app/logout" || logout.Query().Get("service") != "https://app.example/" {
		t.Fatalf("unexpected logout URL: %s", logout)
	}
}
