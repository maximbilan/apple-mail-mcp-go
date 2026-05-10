package tools

import (
	"context"
	"log/slog"

	"github.com/maximbilan/apple-mail-mcp-go/internal/mail"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type sendEmailInput struct {
	To      []string `json:"to" jsonschema:"Recipient email addresses"`
	Subject string   `json:"subject" jsonschema:"Email subject"`
	Body    string   `json:"body" jsonschema:"Plain text email body"`
	CC      []string `json:"cc,omitempty" jsonschema:"CC recipient email addresses"`
	BCC     []string `json:"bcc,omitempty" jsonschema:"BCC recipient email addresses"`
	Account string   `json:"account,omitempty" jsonschema:"Optional Mail account name"`
}

type sendEmailOutput struct {
	MessageID string `json:"message_id"`
}

type createDraftInput = sendEmailInput

type createDraftOutput struct {
	MessageID string `json:"message_id"`
}

func registerComposeTools(server *mcp.Server, client MailClient, logger *slog.Logger) {
	mcp.AddTool(server, &mcp.Tool{Name: "send_email", Description: "Send a new plaintext email"},
		newSendEmailHandler(client, logger))

	mcp.AddTool(server, &mcp.Tool{Name: "create_draft", Description: "Create a draft email"},
		newCreateDraftHandler(client, logger))
}

func newSendEmailHandler(client MailClient, logger *slog.Logger) func(context.Context, *mcp.CallToolRequest, sendEmailInput) (*mcp.CallToolResult, sendEmailOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input sendEmailInput) (*mcp.CallToolResult, sendEmailOutput, error) {
		defer startToolLog(logger, "send_email")()
		id, err := client.SendEmail(ctx, mail.ComposeInput{
			To:      input.To,
			Subject: input.Subject,
			Body:    input.Body,
			CC:      input.CC,
			BCC:     input.BCC,
			Account: input.Account,
		})
		if err != nil {
			return nil, sendEmailOutput{}, err
		}
		return nil, sendEmailOutput{MessageID: id}, nil
	}
}

func newCreateDraftHandler(client MailClient, logger *slog.Logger) func(context.Context, *mcp.CallToolRequest, createDraftInput) (*mcp.CallToolResult, createDraftOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input createDraftInput) (*mcp.CallToolResult, createDraftOutput, error) {
		defer startToolLog(logger, "create_draft")()
		id, err := client.CreateDraft(ctx, mail.ComposeInput{
			To:      input.To,
			Subject: input.Subject,
			Body:    input.Body,
			CC:      input.CC,
			BCC:     input.BCC,
			Account: input.Account,
		})
		if err != nil {
			return nil, createDraftOutput{}, err
		}
		return nil, createDraftOutput{MessageID: id}, nil
	}
}
