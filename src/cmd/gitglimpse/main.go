package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	spinner             spinner.Model
	loading             bool
	login               string
	name                string
	err                 string
	openPRCount         int
	assignedIssueCount  int
	recentActivityCount int
	lastUpdated         time.Time
}

var (
	titleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	labelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Bold(true)
	valueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true)
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	panelStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).Width(60)
)

type authMsg struct {
	login               string
	name                string
	err                 error
	openPRCount         int
	assignedIssueCount  int
	recentActivityCount int
	lastUpdated         time.Time
}

func newSpinner() spinner.Model {
	return spinner.New(
		spinner.WithSpinner(spinner.Line),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("63"))),
	)
}

func initialModel() model {
	return model{
		spinner: newSpinner(),
		loading: true,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, fetchGitHubUserCommand(false))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch strings.ToLower(msg.String()) {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "r":
			m.loading = true
			m.err = ""
			m.spinner = newSpinner()
			return m, tea.Batch(m.spinner.Tick, fetchGitHubUserCommand(true))
		default:
			return m, nil
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case authMsg:
		if msg.err != nil {
			m.loading = false
			m.err = msg.err.Error()
			return m, nil
		}
		m.loading = false
		m.login = msg.login
		m.name = msg.name
		m.openPRCount = msg.openPRCount
		m.assignedIssueCount = msg.assignedIssueCount
		m.recentActivityCount = msg.recentActivityCount
		m.lastUpdated = msg.lastUpdated
		return m, nil
	default:
		return m, nil
	}
}

func (m model) View() string {
	title := titleStyle.Render("gitglimpse")

	if m.loading {
		body := fmt.Sprintf("%s %s", m.spinner.View(), "Authenticating with GitHub...")
		footer := "Press r to refresh, q/esc to quit."
		return panelStyle.Render(fmt.Sprintf("%s\n\n%s\n\n%s", title, body, footer))
	}

	if m.err != "" {
		body := fmt.Sprintf("GitHub auth failed:\n%s", errorStyle.Render(m.err))
		footer := "Press r to retry, q/esc to quit."
		return panelStyle.Render(fmt.Sprintf("%s\n\n%s\n\n%s", title, body, footer))
	}

	identity := m.login
	if m.name != "" {
		identity = fmt.Sprintf("%s (%s)", m.name, m.login)
	}

	lastUpdated := "unknown"
	if !m.lastUpdated.IsZero() {
		lastUpdated = m.lastUpdated.Format("Jan 2 15:04")
	}

	body := strings.Join([]string{
		fmt.Sprintf("%s %s", labelStyle.Render("User:"), valueStyle.Render(identity)),
		"",
		fmt.Sprintf("%s %s", labelStyle.Render("Open PRs:"), valueStyle.Render(fmt.Sprintf("%d", m.openPRCount))),
		fmt.Sprintf("%s %s", labelStyle.Render("Assigned issues:"), valueStyle.Render(fmt.Sprintf("%d", m.assignedIssueCount))),
		fmt.Sprintf("%s %s", labelStyle.Render("Recent activity:"), valueStyle.Render(fmt.Sprintf("%d", m.recentActivityCount))),
		"",
		fmt.Sprintf("%s %s", labelStyle.Render("Last updated:"), valueStyle.Render(lastUpdated)),
	}, "\n")
	footer := "Press r to refresh, q/esc to quit."
	return panelStyle.Render(fmt.Sprintf("%s\n\n%s\n\n%s", title, body, footer))
}

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

func newGitHubRequest(token, urlStr string) (*http.Request, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN is not set")
	}

	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "gitglimpse")
	req.Header.Set("Accept", "application/vnd.github+json")

	return req, nil
}

func parseGitHubError(body []byte, statusCode int) error {
	var errResp struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(body, &errResp)
	message := strings.TrimSpace(errResp.Message)
	if message == "" {
		message = strings.TrimSpace(string(body))
	}
	return fmt.Errorf("GitHub request failed: %s - %s", http.StatusText(statusCode), message)
}

func parseGitHubUserResponse(body []byte, statusCode int) (login, name string, err error) {
	if statusCode != http.StatusOK {
		return "", "", parseGitHubError(body, statusCode)
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

func parseSearchTotalCountResponse(body []byte, statusCode int) (int, error) {
	if statusCode != http.StatusOK {
		return 0, parseGitHubError(body, statusCode)
	}

	var result struct {
		TotalCount int `json:"total_count"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}

	return result.TotalCount, nil
}

func parseJSONArrayCountResponse(body []byte, statusCode int) (int, error) {
	if statusCode != http.StatusOK {
		return 0, parseGitHubError(body, statusCode)
	}

	var items []any
	if err := json.Unmarshal(body, &items); err != nil {
		return 0, err
	}

	return len(items), nil
}

type cacheData struct {
	Login               string    `json:"login"`
	Name                string    `json:"name"`
	OpenPRCount         int       `json:"open_pr_count"`
	AssignedIssueCount  int       `json:"assigned_issue_count"`
	RecentActivityCount int       `json:"recent_activity_count"`
	Timestamp           time.Time `json:"timestamp"`
}

func cacheFilePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "git-glimpse", "cache.json"), nil
}

func loadCache() (cacheData, bool) {
	path, err := cacheFilePath()
	if err != nil {
		return cacheData{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cacheData{}, false
	}
	var cache cacheData
	if err := json.Unmarshal(data, &cache); err != nil {
		return cacheData{}, false
	}
	if time.Since(cache.Timestamp) > 5*time.Minute {
		return cacheData{}, false
	}
	return cache, true
}

func saveCache(cache cacheData) error {
	path, err := cacheFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func fetchGitHubUserCommand(force bool) tea.Cmd {
	return func() tea.Msg {
		token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
		if token == "" {
			return authMsg{err: fmt.Errorf("GITHUB_TOKEN is not set")}
		}

		if !force {
			if cache, ok := loadCache(); ok {
				return authMsg{
					login:               cache.Login,
					name:                cache.Name,
					openPRCount:         cache.OpenPRCount,
					assignedIssueCount:  cache.AssignedIssueCount,
					recentActivityCount: cache.RecentActivityCount,
					lastUpdated:         cache.Timestamp,
				}
			}
		}

		client := &http.Client{Timeout: 10 * time.Second}
		msg := fetchGitHubUserDataWithClient(client, token)
		result, ok := msg.(authMsg)
		if !ok {
			return msg
		}
		if result.err == nil {
			cache := cacheData{
				Login:               result.login,
				Name:                result.name,
				OpenPRCount:         result.openPRCount,
				AssignedIssueCount:  result.assignedIssueCount,
				RecentActivityCount: result.recentActivityCount,
				Timestamp:           time.Now(),
			}
			_ = saveCache(cache)
			result.lastUpdated = cache.Timestamp
			return result
		}
		return result
	}
}

func fetchGitHubUser() tea.Msg {
	return fetchGitHubUserCommand(false)()
}

func fetchGitHubUserForce() tea.Msg {
	return fetchGitHubUserCommand(true)()
}

func fetchGitHubUserDataWithClient(client httpDoer, token string) tea.Msg {
	userReq, err := newGitHubRequest(token, "https://api.github.com/user")
	if err != nil {
		return authMsg{err: err}
	}

	resp, err := client.Do(userReq)
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

	prURL := fmt.Sprintf("https://api.github.com/search/issues?q=%s&per_page=1", url.QueryEscape(fmt.Sprintf("author:%s is:pull-request is:open", login)))
	prReq, err := newGitHubRequest(token, prURL)
	if err != nil {
		return authMsg{err: err}
	}

	resp, err = client.Do(prReq)
	if err != nil {
		return authMsg{err: err}
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return authMsg{err: err}
	}

	openPRCount, err := parseSearchTotalCountResponse(body, resp.StatusCode)
	if err != nil {
		return authMsg{err: err}
	}

	issuesReq, err := newGitHubRequest(token, "https://api.github.com/issues?per_page=100")
	if err != nil {
		return authMsg{err: err}
	}

	resp, err = client.Do(issuesReq)
	if err != nil {
		return authMsg{err: err}
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return authMsg{err: err}
	}

	assignedIssueCount, err := parseJSONArrayCountResponse(body, resp.StatusCode)
	if err != nil {
		return authMsg{err: err}
	}

	eventsURL := fmt.Sprintf("https://api.github.com/users/%s/events?per_page=100", url.QueryEscape(login))
	eventsReq, err := newGitHubRequest(token, eventsURL)
	if err != nil {
		return authMsg{err: err}
	}

	resp, err = client.Do(eventsReq)
	if err != nil {
		return authMsg{err: err}
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return authMsg{err: err}
	}

	recentActivityCount, err := parseJSONArrayCountResponse(body, resp.StatusCode)
	if err != nil {
		return authMsg{err: err}
	}

	return authMsg{
		login:               login,
		name:                name,
		openPRCount:         openPRCount,
		assignedIssueCount:  assignedIssueCount,
		recentActivityCount: recentActivityCount,
	}
}

func main() {
	p := tea.NewProgram(initialModel())
	if err := p.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start program: %v\n", err)
		os.Exit(1)
	}
}
