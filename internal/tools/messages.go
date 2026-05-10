package tools

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/maximbilan/apple-mail-mcp/internal/mail"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type searchMessagesInput struct {
	Account         string `json:"account" jsonschema:"Account name"`
	Mailbox         string `json:"mailbox,omitempty" jsonschema:"Mailbox name (default INBOX)"`
	SenderContains  string `json:"sender_contains,omitempty" jsonschema:"Filter sender contains text"`
	SubjectContains string `json:"subject_contains,omitempty" jsonschema:"Filter subject contains text"`
	UnreadOnly      bool   `json:"unread_only,omitempty" jsonschema:"Only unread messages"`
	DateFrom        string `json:"date_from,omitempty" jsonschema:"RFC3339 lower-bound date"`
	DateTo          string `json:"date_to,omitempty" jsonschema:"RFC3339 upper-bound date"`
	Limit           int    `json:"limit,omitempty" jsonschema:"Max results (default 50, max 200)"`
}

type searchMessagesOutput struct {
	Messages []searchMessageRow `json:"messages"`
}

type searchMessageRow struct {
	ID      string `json:"id"`
	Subject string `json:"subject"`
	Sender  string `json:"sender"`
	Date    string `json:"date"`
	Read    bool   `json:"read"`
	Mailbox string `json:"mailbox"`
}

type getMessageInput struct {
	MessageID   string `json:"message_id" jsonschema:"Mail message id"`
	IncludeBody *bool  `json:"include_body,omitempty" jsonschema:"Include full body (default true)"`
}

type getMessageOutput struct {
	Message messageRow `json:"message"`
}

type messageRow struct {
	ID         string    `json:"id"`
	Subject    string    `json:"subject"`
	Sender     string    `json:"sender"`
	Recipients []string  `json:"recipients"`
	CC         []string  `json:"cc"`
	Date       time.Time `json:"date"`
	Body       string    `json:"body"`
	Mailbox    string    `json:"mailbox"`
	Read       bool      `json:"read"`
	Flagged    bool      `json:"flagged"`
}

type markAsReadInput struct {
	MessageIDs []string `json:"message_ids" jsonschema:"List of message ids (max 100)"`
	Read       *bool    `json:"read,omitempty" jsonschema:"Read status to set (default true)"`
}

type markAsReadOutput struct {
	Updated int `json:"updated"`
}

func registerMessageTools(server *mcp.Server, client MailClient, logger *slog.Logger) {
	mcp.AddTool(server, &mcp.Tool{Name: "search_messages", Description: "Search messages in a mailbox"},
		newSearchMessagesHandler(client, logger))

	mcp.AddTool(server, &mcp.Tool{Name: "get_message", Description: "Get message details and body"},
		newGetMessageHandler(client, logger))
}

func registerMutationTools(server *mcp.Server, client MailClient, logger *slog.Logger) {
	mcp.AddTool(server, &mcp.Tool{Name: "mark_as_read", Description: "Mark messages read/unread"},
		newMarkAsReadHandler(client, logger))
}

func newSearchMessagesHandler(client MailClient, logger *slog.Logger) func(context.Context, *mcp.CallToolRequest, searchMessagesInput) (*mcp.CallToolResult, searchMessagesOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input searchMessagesInput) (*mcp.CallToolResult, searchMessagesOutput, error) {
		defer startToolLog(logger, "search_messages")()
		query, err := toSearchQuery(input)
		if err != nil {
			return nil, searchMessagesOutput{}, err
		}
		msgs, err := client.SearchMessages(ctx, query)
		if err != nil {
			return nil, searchMessagesOutput{}, err
		}
		out := make([]searchMessageRow, 0, len(msgs))
		for _, msg := range msgs {
			out = append(out, searchMessageRow{
				ID:      msg.ID,
				Subject: msg.Subject,
				Sender:  msg.Sender,
				Date:    formatRFC3339OrEmpty(msg.Date),
				Read:    msg.Read,
				Mailbox: msg.Mailbox,
			})
		}
		return nil, searchMessagesOutput{Messages: out}, nil
	}
}

func formatRFC3339OrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func newGetMessageHandler(client MailClient, logger *slog.Logger) func(context.Context, *mcp.CallToolRequest, getMessageInput) (*mcp.CallToolResult, getMessageOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input getMessageInput) (*mcp.CallToolResult, getMessageOutput, error) {
		defer startToolLog(logger, "get_message")()
		includeBody := true
		if input.IncludeBody != nil {
			includeBody = *input.IncludeBody
		}
		msg, err := client.GetMessage(ctx, input.MessageID, includeBody)
		if err != nil {
			return nil, getMessageOutput{}, err
		}
		out := messageRow{
			ID:         msg.ID,
			Subject:    msg.Subject,
			Sender:     msg.Sender,
			Recipients: msg.Recipients,
			CC:         msg.CC,
			Date:       msg.Date,
			Body:       msg.Body,
			Mailbox:    msg.Mailbox,
			Read:       msg.Read,
			Flagged:    msg.Flagged,
		}
		return nil, getMessageOutput{Message: out}, nil
	}
}

func newMarkAsReadHandler(client MailClient, logger *slog.Logger) func(context.Context, *mcp.CallToolRequest, markAsReadInput) (*mcp.CallToolResult, markAsReadOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input markAsReadInput) (*mcp.CallToolResult, markAsReadOutput, error) {
		defer startToolLog(logger, "mark_as_read")()
		read := true
		if input.Read != nil {
			read = *input.Read
		}
		updated, err := client.MarkAsRead(ctx, input.MessageIDs, read)
		if err != nil {
			return nil, markAsReadOutput{}, err
		}
		return nil, markAsReadOutput{Updated: updated}, nil
	}
}

func toSearchQuery(input searchMessagesInput) (mail.SearchQuery, error) {
	if input.Account == "" {
		return mail.SearchQuery{}, errors.New("account is required")
	}
	query := mail.SearchQuery{
		Account:         input.Account,
		Mailbox:         input.Mailbox,
		SenderContains:  input.SenderContains,
		SubjectContains: input.SubjectContains,
		UnreadOnly:      input.UnreadOnly,
		Limit:           input.Limit,
	}
	if input.DateFrom != "" {
		t, err := time.Parse(time.RFC3339, input.DateFrom)
		if err != nil {
			return mail.SearchQuery{}, errors.New("date_from must be RFC3339")
		}
		query.DateFrom = &t
	}
	if input.DateTo != "" {
		t, err := time.Parse(time.RFC3339, input.DateTo)
		if err != nil {
			return mail.SearchQuery{}, errors.New("date_to must be RFC3339")
		}
		query.DateTo = &t
	}
	return query, nil
}
