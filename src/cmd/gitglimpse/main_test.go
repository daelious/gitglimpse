package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type stubDoer struct {
	resps []*http.Response
	err   error
	idx   int
}

func (s *stubDoer) Do(req *http.Request) (*http.Response, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.idx >= len(s.resps) {
		return nil, io.EOF
	}
	resp := s.resps[s.idx]
	s.idx++
	return resp, nil
}

func TestNewGitHubRequest(t *testing.T) {
	_, err := newGitHubRequest("   ", "https://api.github.com/user")
	if err == nil {
		t.Fatal("expected error when token is empty")
	}

	req, err := newGitHubRequest("  token  ", "https://api.github.com/user")
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

func TestParseSearchTotalCountResponse(t *testing.T) {
	count, err := parseSearchTotalCountResponse([]byte(`{"total_count": 7}`), http.StatusOK)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 7 {
		t.Fatalf("expected 7, got %d", count)
	}
}

func TestParseJSONArrayCountResponse(t *testing.T) {
	count, err := parseJSONArrayCountResponse([]byte(`[{}, {}]`), http.StatusOK)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2, got %d", count)
	}
}

func TestFetchGitHubUserDataWithClient(t *testing.T) {
	responses := []*http.Response{
		{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"login":"eve","name":"Eve"}`)),
		},
		{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"total_count": 7}`)),
		},
		{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`[{}, {}]`)),
		},
		{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`[{}, {}, {}]`)),
		},
	}

	msg := fetchGitHubUserDataWithClient(&stubDoer{resps: responses}, "token")
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
	if auth.openPRCount != 7 {
		t.Fatalf("expected 7 open PRs, got %d", auth.openPRCount)
	}
	if auth.assignedIssueCount != 2 {
		t.Fatalf("expected 2 assigned issues, got %d", auth.assignedIssueCount)
	}
	if auth.recentActivityCount != 3 {
		t.Fatalf("expected 3 recent activity events, got %d", auth.recentActivityCount)
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
