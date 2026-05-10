package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/maximbilan/apple-mail-mcp/internal/mail"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type MailClient interface {
	ListAccounts(ctx context.Context) ([]mail.Account, error)
	ListMailboxes(ctx context.Context, account string) ([]mail.Mailbox, error)
	SearchMessages(ctx context.Context, q mail.SearchQuery) ([]mail.MessageSummary, error)
	GetMessage(ctx context.Context, messageID string, includeBody bool) (mail.Message, error)
	SendEmail(ctx context.Context, input mail.ComposeInput) (string, error)
	CreateDraft(ctx context.Context, input mail.ComposeInput) (string, error)
	MarkAsRead(ctx context.Context, messageIDs []string, read bool) (int, error)
	GetUnreadCount(ctx context.Context, account string) (mail.UnreadCounts, error)
}

type RegisterOptions struct {
	ReadOnly bool
	Logger   *slog.Logger
}

type ToolDoc struct {
	Name        string
	Params      string
	Description string
}

func Register(server *mcp.Server, client MailClient, opts RegisterOptions) {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	registerAccountsTools(server, client, opts.Logger)
	registerMessageTools(server, client, opts.Logger)
	if !opts.ReadOnly {
		registerComposeTools(server, client, opts.Logger)
		registerMutationTools(server, client, opts.Logger)
	}
}

func ToolDocs(readOnly bool) []ToolDoc {
	docs := []ToolDoc{
		{Name: "list_accounts", Params: "none", Description: "List account names configured in Mail.app."},
		{Name: "list_mailboxes", Params: "account (string, required)", Description: "List mailboxes for an account with unread counts."},
		{Name: "search_messages", Params: "account (required), mailbox, sender_contains, subject_contains, unread_only, date_from, date_to, limit", Description: "Search messages with server-side filters."},
		{Name: "get_message", Params: "message_id (required), include_body", Description: "Get message metadata and optionally full body."},
		{Name: "get_unread_count", Params: "account (optional)", Description: "Get total and per-mailbox unread counts."},
	}
	if !readOnly {
		docs = append(docs,
			ToolDoc{Name: "send_email", Params: "to, subject, body, cc, bcc, account", Description: "Send a plaintext email immediately."},
			ToolDoc{Name: "create_draft", Params: "to, subject, body, cc, bcc, account", Description: "Create a draft email without sending."},
			ToolDoc{Name: "mark_as_read", Params: "message_ids, read", Description: "Mark messages as read/unread (max 100 ids)."},
		)
	}
	return docs
}

func startToolLog(logger *slog.Logger, name string) func() {
	started := time.Now()
	return func() {
		logger.Info("tool_call", slog.String("tool", name), slog.Duration("duration", time.Since(started)))
	}
}
