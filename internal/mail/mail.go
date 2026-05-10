package mail

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var emailRegex = regexp.MustCompile(`^[A-Za-z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?(?:\.[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?)+$`)

type Client struct {
	runner ScriptRunner
	logger *slog.Logger
}

type API interface {
	ListAccounts(ctx context.Context) ([]Account, error)
	ListMailboxes(ctx context.Context, account string) ([]Mailbox, error)
	SearchMessages(ctx context.Context, q SearchQuery) ([]MessageSummary, error)
	GetMessage(ctx context.Context, messageID string, includeBody bool) (Message, error)
	SendEmail(ctx context.Context, input ComposeInput) (string, error)
	CreateDraft(ctx context.Context, input ComposeInput) (string, error)
	MarkAsRead(ctx context.Context, messageIDs []string, read bool) (int, error)
	GetUnreadCount(ctx context.Context, account string) (UnreadCounts, error)
}

func NewClient(runner ScriptRunner, logger *slog.Logger) *Client {
	if runner == nil {
		runner = NewOsaScriptRunnerFromEnv()
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{runner: runner, logger: logger}
}

func (c *Client) ListAccounts(ctx context.Context) ([]Account, error) {
	raw, err := c.runScript(ctx, buildListAccountsScript())
	if err != nil {
		return nil, err
	}
	if raw == "" {
		return nil, nil
	}
	names := strings.Split(raw, RecordSep)
	accounts := make([]Account, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		accounts = append(accounts, Account{Name: name})
	}
	return accounts, nil
}

func (c *Client) ListMailboxes(ctx context.Context, account string) ([]Mailbox, error) {
	if strings.TrimSpace(account) == "" {
		return nil, errors.New("account is required")
	}
	raw, err := c.runScript(ctx, buildListMailboxesScript(account))
	if err != nil {
		return nil, err
	}
	if raw == "" {
		return nil, nil
	}
	rows := strings.Split(raw, RecordSep)
	out := make([]Mailbox, 0, len(rows))
	for _, row := range rows {
		parts := strings.Split(row, FieldSep)
		if len(parts) != 2 {
			continue
		}
		unread, _ := strconv.Atoi(parts[1])
		out = append(out, Mailbox{Account: account, Name: parts[0], UnreadCount: unread})
	}
	return out, nil
}

func (c *Client) SearchMessages(ctx context.Context, q SearchQuery) ([]MessageSummary, error) {
	if strings.TrimSpace(q.Account) == "" {
		return nil, errors.New("account is required")
	}
	if q.Limit <= 0 {
		q.Limit = 50
	}
	if q.Limit > maxSearchLimit {
		q.Limit = maxSearchLimit
	}
	if q.Mailbox == "" {
		q.Mailbox = defaultMailbox
	}
	if q.DateFrom != nil && q.DateTo != nil && q.DateFrom.After(*q.DateTo) {
		return nil, errors.New("date_from must be before or equal to date_to")
	}

	raw, err := c.runScript(ctx, buildSearchMessagesScript(q))
	if err != nil {
		// Some Mail databases contain messages with malformed/missing "date sent"
		// values that crash date-constrained AppleScript queries. Fall back to a
		// broader query and apply date bounds in Go.
		if (q.DateFrom != nil || q.DateTo != nil) && isDateSentQueryError(err) {
			fallback := q
			fallback.DateFrom = nil
			fallback.DateTo = nil
			rawFallback, errFallback := c.runScript(ctx, buildSearchMessagesScript(fallback))
			if errFallback != nil {
				return nil, err
			}
			msgs := parseSearchMessages(rawFallback)
			msgs = filterMessagesByDate(msgs, q.DateFrom, q.DateTo)
			if len(msgs) > q.Limit {
				msgs = msgs[:q.Limit]
			}
			return msgs, nil
		}
		return nil, err
	}
	return parseSearchMessages(raw), nil
}

func (c *Client) GetMessage(ctx context.Context, messageID string, includeBody bool) (Message, error) {
	if strings.TrimSpace(messageID) == "" {
		return Message{}, errors.New("message_id is required")
	}
	if _, err := strconv.Atoi(messageID); err != nil {
		return Message{}, errors.New("message_id must be an integer string")
	}

	raw, err := c.runScript(ctx, buildGetMessageScript(messageID, includeBody))
	if err != nil {
		return Message{}, err
	}
	parts := strings.Split(raw, FieldSep)
	if len(parts) != 10 {
		return Message{}, fmt.Errorf("unexpected get_message payload format")
	}
	return Message{
		ID:         parts[0],
		Subject:    parts[1],
		Sender:     parts[2],
		Recipients: splitNonEmpty(parts[3], ListSep),
		CC:         splitNonEmpty(parts[4], ListSep),
		Date:       parseUnix(parts[5]),
		Body:       parts[6],
		Mailbox:    parts[7],
		Read:       parseBool(parts[8]),
		Flagged:    parseBool(parts[9]),
	}, nil
}

func (c *Client) SendEmail(ctx context.Context, input ComposeInput) (string, error) {
	if err := validateComposeInput(input); err != nil {
		return "", err
	}
	return c.runScript(ctx, buildSendOrDraftScript(input, false))
}

func (c *Client) CreateDraft(ctx context.Context, input ComposeInput) (string, error) {
	if err := validateComposeInput(input); err != nil {
		return "", err
	}
	return c.runScript(ctx, buildSendOrDraftScript(input, true))
}

func (c *Client) MarkAsRead(ctx context.Context, messageIDs []string, read bool) (int, error) {
	if len(messageIDs) == 0 {
		return 0, errors.New("message_ids is required")
	}
	if len(messageIDs) > 100 {
		return 0, errors.New("message_ids exceeds max of 100")
	}
	for _, id := range messageIDs {
		if _, err := strconv.Atoi(id); err != nil {
			return 0, fmt.Errorf("invalid message id %q: must be an integer string", id)
		}
	}
	raw, err := c.runScript(ctx, buildMarkAsReadScript(messageIDs, read))
	if err != nil {
		return 0, err
	}
	count, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("invalid mark_as_read response: %w", err)
	}
	return count, nil
}

func (c *Client) GetUnreadCount(ctx context.Context, account string) (UnreadCounts, error) {
	raw, err := c.runScript(ctx, buildUnreadCountScript(account))
	if err != nil {
		return UnreadCounts{}, err
	}
	if raw == "" {
		return UnreadCounts{Account: account, Mailboxes: nil}, nil
	}
	rows := strings.Split(raw, RecordSep)
	if len(rows) == 0 {
		return UnreadCounts{Account: account, Mailboxes: nil}, nil
	}
	total, _ := strconv.Atoi(rows[0])
	counts := make([]MailboxCount, 0, len(rows)-1)
	for _, row := range rows[1:] {
		parts := strings.Split(row, FieldSep)
		if len(parts) != 3 {
			continue
		}
		unread, _ := strconv.Atoi(parts[2])
		counts = append(counts, MailboxCount{Account: parts[0], Mailbox: parts[1], Unread: unread})
	}
	return UnreadCounts{Account: account, Total: total, Mailboxes: counts}, nil
}

func (c *Client) runScript(ctx context.Context, script string) (string, error) {
	out, err := c.runner.Run(ctx, script)
	if err == nil {
		return out, nil
	}
	var execErr *ExecError
	if errors.As(err, &execErr) {
		truncated := execErr.Script
		if len(truncated) > 500 {
			truncated = truncated[:500] + "..."
		}
		c.logger.Error("apple script execution failed",
			slog.Int("exit_code", execErr.ExitCode),
			slog.String("stderr", execErr.Stderr),
			slog.String("script", truncated),
		)
	}
	return "", err
}

func splitNonEmpty(s, sep string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	items := strings.Split(s, sep)
	out := make([]string, 0, len(items))
	for _, it := range items {
		it = strings.TrimSpace(it)
		if it == "" {
			continue
		}
		out = append(out, it)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseUnix(s string) time.Time {
	sec, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(sec, 0).UTC()
}

func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "yes" || s == "1"
}

func parseSearchMessages(raw string) []MessageSummary {
	if raw == "" {
		return nil
	}
	rows := strings.Split(raw, RecordSep)
	msgs := make([]MessageSummary, 0, len(rows))
	for _, row := range rows {
		parts := strings.Split(row, FieldSep)
		if len(parts) != 6 {
			continue
		}
		msgs = append(msgs, MessageSummary{
			ID:      parts[0],
			Subject: parts[1],
			Sender:  parts[2],
			Date:    parseUnix(parts[3]),
			Read:    parseBool(parts[4]),
			Mailbox: parts[5],
		})
	}
	return msgs
}

func filterMessagesByDate(msgs []MessageSummary, from, to *time.Time) []MessageSummary {
	if from == nil && to == nil {
		return msgs
	}
	out := make([]MessageSummary, 0, len(msgs))
	for _, msg := range msgs {
		if msg.Date.IsZero() {
			continue
		}
		if from != nil && msg.Date.Before(*from) {
			continue
		}
		if to != nil && msg.Date.After(*to) {
			continue
		}
		out = append(out, msg)
	}
	return out
}

func isDateSentQueryError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "date sent")
}

func validateComposeInput(input ComposeInput) error {
	if len(input.To) == 0 {
		return errors.New("to is required")
	}
	if strings.TrimSpace(input.Subject) == "" {
		return errors.New("subject is required")
	}
	if input.Body == "" {
		return errors.New("body is required")
	}
	for _, addr := range append(append([]string{}, input.To...), append(input.CC, input.BCC...)...) {
		if strings.TrimSpace(addr) == "" {
			return errors.New("email addresses cannot be empty")
		}
		if hasControlChars(addr) {
			return fmt.Errorf("invalid email address %q: contains control characters", addr)
		}
		if !emailRegex.MatchString(addr) {
			return fmt.Errorf("invalid email address %q", addr)
		}
	}
	if hasControlChars(input.Subject) || hasControlChars(input.Body) {
		return errors.New("subject/body contain control characters")
	}
	return nil
}

func hasControlChars(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			if r == '\n' || r == '\r' || r == '\t' {
				continue
			}
			return true
		}
	}
	return false
}
