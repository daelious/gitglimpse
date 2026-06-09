# gitglimpse

gitglimpse is a small Bubble Tea-based GitHub dashboard for quickly checking your current GitHub status from the terminal.

It was built as a lightweight Rust-busting side project to fetch:

- open pull requests authored by the current user
- issues assigned to the current user
- recent public activity for the current user

It also supports a token expiry warning when the PAT is about to expire.

## Features

- authenticates with GitHub using `GITHUB_TOKEN`
- fetches open PR count, assigned issue count, and recent activity count
- refresh with `r`
- quit with `q`, `esc`, or `Ctrl+C`
- caches results for 5 minutes to avoid repeated API calls
- warns when the PAT expires within 7 days if `GITHUB_TOKEN_EXPIRES_AT` is set

## Install

```bash
cd /path/to/gitglimpse
go build -o gitglimpse ./src/cmd/gitglimpse
```

## Usage

Set your GitHub personal access token before running:

```bash
export GITHUB_TOKEN="<your_pat>"
```

Optionally provide an expiry date for the token to enable the warning:

```bash
export GITHUB_TOKEN_EXPIRES_AT="2026-06-15T00:00:00Z"
```

Then run:

```bash
./gitglimpse
```

## Token expiry warning

If `GITHUB_TOKEN_EXPIRES_AT` is set, `gitglimpse` will show a warning when the expiry date is within 7 days.

Supported expiry formats:

- `2026-06-15T00:00:00Z`
- `2026-06-15`
- `2026-06-15 15:04`

## Notes

- This is intended as a lightweight terminal dashboard, not a full GitHub client.
- The output is cached for 5 minutes by default.
- If `GITHUB_TOKEN` is missing or invalid, the tool will show an authentication error.

## Development

Run the package tests with:

```bash
go test ./src/cmd/gitglimpse
```
