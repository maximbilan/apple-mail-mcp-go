# Contributing

Thanks for contributing.

## Development Setup

Prerequisites:
- macOS (required for Mail integration)
- Go 1.22+

Build and test:

```sh
go build ./...
go vet ./...
go test ./... -race
```

Integration tests (requires Mail.app + permissions):

```sh
go test ./tests/integration -tags=integration -v
```

## Pull Requests

Please include:
- Clear summary of behavior changes
- Tests for new logic
- Any user-facing docs updates (`README.md`)

## Release Notes Labels (recommended)

To improve generated release notes, use labels such as:
- `enhancement`
- `bug`
- `documentation`
- `dependencies`
- `breaking-change`

## Code Style

- Keep tool handlers small
- Keep AppleScript generation in `internal/mail/applescript.go`
- Use concrete types for MCP tool input/output structs
- Use `gofmt` before submitting
