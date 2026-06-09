package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	loading bool
	login   string
	name    string
	err     string
}

type authMsg struct {
	login string
	name  string
	err   error
}

func (m model) Init() tea.Cmd {
	return fetchGitHubUser
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		default:
			return m, nil
		}
	case authMsg:
		if msg.err != nil {
			m.loading = false
			m.err = msg.err.Error()
			return m, nil
		}
		m.loading = false
		m.login = msg.login
		m.name = msg.name
		return m, nil
	default:
		return m, nil
	}
}

func (m model) View() string {
	if m.loading {
		return "gitglimpse\n\nAuthenticating with GitHub...\n\nPress q, esc, or ctrl+c to quit.\n"
	}

	if m.err != "" {
		return fmt.Sprintf("gitglimpse\n\nGitHub auth failed:\n%s\n\nPress q, esc, or ctrl+c to quit.\n", m.err)
	}

	identity := m.login
	if m.name != "" {
		identity = fmt.Sprintf("%s (%s)", m.name, m.login)
	}

	return fmt.Sprintf("gitglimpse\n\nAuthenticated as: %s\n\nPress q, esc, or ctrl+c to quit.\n", identity)
}

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

func newGitHubRequest(token string) (*http.Request, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN is not set")
	}

	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "gitglimpse")
	req.Header.Set("Accept", "application/vnd.github+json")

	return req, nil
}

func parseGitHubUserResponse(body []byte, statusCode int) (login, name string, err error) {
	if statusCode != http.StatusOK {
		var errResp struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(body, &errResp)
		message := strings.TrimSpace(errResp.Message)
		if message == "" {
			message = strings.TrimSpace(string(body))
		}
		return "", "", fmt.Errorf("GitHub authentication failed: %s - %s", http.StatusText(statusCode), message)
	}

	var result struct {
		Login string `json:"login"`
		Name  string `json:"name"`
	}
	if err = json.Unmarshal(body, &result); err != nil {
		return "", "", err
	}

	return result.Login, result.Name, nil
}

func fetchGitHubUserWithClient(client httpDoer, token string) tea.Msg {
	req, err := newGitHubRequest(token)
	if err != nil {
		return authMsg{err: err}
	}

	resp, err := client.Do(req)
	if err != nil {
		return authMsg{err: err}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return authMsg{err: err}
	}

	login, name, err := parseGitHubUserResponse(body, resp.StatusCode)
	if err != nil {
		return authMsg{err: err}
	}

	return authMsg{login: login, name: name}
}

func fetchGitHubUser() tea.Msg {
	token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	client := &http.Client{Timeout: 10 * time.Second}
	return fetchGitHubUserWithClient(client, token)
}

func main() {
	p := tea.NewProgram(model{loading: true})
	if err := p.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start program: %v\n", err)
		os.Exit(1)
	}
}
