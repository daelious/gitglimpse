package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type stubDoer struct {
	resp *http.Response
	err  error
}

func (s stubDoer) Do(req *http.Request) (*http.Response, error) {
	return s.resp, s.err
}

func TestNewGitHubRequest(t *testing.T) {
	_, err := newGitHubRequest("   ")
	if err == nil {
		t.Fatal("expected error when token is empty")
	}

	req, err := newGitHubRequest("  token  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if req.Method != http.MethodGet {
		t.Fatalf("expected GET method, got %s", req.Method)
	}
	if req.URL.String() != "https://api.github.com/user" {
		t.Fatalf("unexpected URL: %s", req.URL)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer token" {
		t.Fatalf("unexpected Authorization header: %s", got)
	}
	if got := req.Header.Get("User-Agent"); got != "gitglimpse" {
		t.Fatalf("unexpected User-Agent header: %s", got)
	}
}

func TestParseGitHubUserResponse_Success(t *testing.T) {
	login, name, err := parseGitHubUserResponse([]byte(`{"login":"alice","name":"Alice"}`), http.StatusOK)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if login != "alice" || name != "Alice" {
		t.Fatalf("unexpected parsed user: %s %s", login, name)
	}
}

func TestParseGitHubUserResponse_ErrorBody(t *testing.T) {
	_, _, err := parseGitHubUserResponse([]byte(`{"message":"Bad credentials"}`), http.StatusUnauthorized)
	if err == nil {
		t.Fatal("expected error for unauthorized response")
	}
	if !strings.Contains(err.Error(), "Bad credentials") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestFetchGitHubUserWithClient(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"login":"eve","name":"Eve"}`)),
	}
	msg := fetchGitHubUserWithClient(stubDoer{resp: resp}, "token")
	auth, ok := msg.(authMsg)
	if !ok {
		t.Fatalf("expected authMsg, got %T", msg)
	}
	if auth.err != nil {
		t.Fatalf("unexpected error: %v", auth.err)
	}
	if auth.login != "eve" || auth.name != "Eve" {
		t.Fatalf("unexpected auth data: %s %s", auth.login, auth.name)
	}
}

func TestModelUpdateAuthMsg(t *testing.T) {
	m := model{loading: true}
	msg, _ := m.Update(authMsg{login: "joe", name: "Joe"})
	updated, ok := msg.(model)
	if !ok {
		t.Fatalf("expected model, got %T", msg)
	}
	if updated.loading {
		t.Fatal("expected loading false after authMsg")
	}
	if updated.login != "joe" || updated.name != "Joe" {
		t.Fatalf("unexpected updated model: %v", updated)
	}
}
