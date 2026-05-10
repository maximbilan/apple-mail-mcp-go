# apple-mail-mcp

`apple-mail-mcp` is a Go Model Context Protocol (MCP) server for macOS that gives MCP-compatible assistants programmatic access to Mail.app for listing accounts/mailboxes, searching and reading messages, sending drafts/emails, and unread-count workflows.

## Prerequisites

- macOS 12+
- Mail.app configured with at least one account
- Go 1.22+ (only required for building from source)

## Installation

### 1. One-click for Claude Desktop

Download `apple-mail-mcp.mcpb` from the [latest release](https://github.com/<owner>/apple-mail-mcp/releases/latest) and double-click it. Claude Desktop installs and registers the server automatically.

### 2. One-line install (recommended for Claude Code or both clients)

```sh
curl -fsSL https://raw.githubusercontent.com/<owner>/apple-mail-mcp/main/install.sh | sh
```

This installer:
- Detects macOS architecture and installs `~/.local/bin/apple-mail-mcp`
- Uses local `go build`/`go install` when Go exists, otherwise downloads the latest release binary and verifies SHA256
- Detects Claude Desktop and/or Claude Code and offers registration

### 3. Build from source (Go developers)

```sh
go install github.com/<owner>/apple-mail-mcp/cmd/apple-mail-mcp@latest
apple-mail-mcp install
```

### 4. Manual configuration

Claude Desktop config path:

`~/Library/Application Support/Claude/claude_desktop_config.json`

Add/update:

```json
{
  "mcpServers": {
    "apple-mail": {
      "command": "/Users/<you>/.local/bin/apple-mail-mcp",
      "args": []
    }
  }
}
```

Claude Code:

```sh
claude mcp add apple-mail "$HOME/.local/bin/apple-mail-mcp" --scope user
```

## Permissions

The first time `osascript` controls Mail.app, macOS shows an Automation permission prompt.
Grant access for the calling process (for example Terminal, Claude Desktop, or Claude Code).
If needed, manage this at:

`System Settings -> Privacy & Security -> Automation`

## Tool Reference

Generated from registered tools via:

```sh
go run ./cmd/apple-mail-mcp tools-docs
```

| Name | Params | Description |
|---|---|---|
| list_accounts | none | List account names configured in Mail.app. |
| list_mailboxes | account (string, required) | List mailboxes for an account with unread counts. |
| search_messages | account (required), mailbox, sender_contains, subject_contains, unread_only, date_from, date_to, limit | Search messages with server-side filters. |
| get_message | message_id (required), include_body | Get message metadata and optionally full body. |
| get_unread_count | account (optional) | Get total and per-mailbox unread counts. |
| send_email | to, subject, body, cc, bcc, account | Send a plaintext email immediately. |
| create_draft | to, subject, body, cc, bcc, account | Create a draft email without sending. |
| mark_as_read | message_ids, read | Mark messages as read/unread (max 100 ids). |

## Running

Start over stdio transport:

```sh
apple-mail-mcp --log-level info
```

Read-only mode (write tools hidden):

```sh
apple-mail-mcp --read-only
```

## Integration Tests

Integration tests require Mail.app on macOS and explicit build tag:

```sh
go test ./tests/integration -tags=integration -v
```

## Troubleshooting

- Mail.app not responding:
  - Open Mail.app once and ensure account sync completes.
- Permission denied:
  - Re-check macOS Automation permissions for the process invoking the server.
- AppleScript timeout:
  - Increase timeout with `APPLE_MAIL_MCP_TIMEOUT` (e.g. `45s` or `60`).
- Config did not update:
  - Restart Claude Desktop and run `claude mcp list` for Claude Code.

## Uninstall

```sh
./uninstall.sh
```

Or if installed from `go install`:

```sh
apple-mail-mcp uninstall
```

## License

MIT
