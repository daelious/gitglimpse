# Contributing to gitglimpse

Thanks for your interest in contributing! This project is intended as a small, approachable GitHub dashboard built with Bubble Tea.

## How to contribute

- Open an issue first for bugs or feature ideas.
- If you want to work on something, leave a comment so maintainers know.
- Follow the existing Go style and keep the code simple.
- Run tests before submitting a pull request:

```bash
go test ./src/cmd/gitglimpse
```

- Format Go files with `gofmt`:

```bash
gofmt -w src/cmd/gitglimpse/*.go
```

## Pull request checklist

- [ ] I have read the documentation in `README.md`.
- [ ] My code is formatted with `gofmt`.
- [ ] I have added or updated tests where appropriate.
- [ ] The CI build passes.
- [ ] The PR description explains the change clearly.

## Code review

Pull requests are reviewed on GitHub. Small, focused changes are easiest to merge.
