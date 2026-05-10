package tools

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/maximbilan/apple-mail-mcp-go/internal/mail"
)

type fakeMailClient struct {
	accounts      []mail.Account
	mailboxes     []mail.Mailbox
	searchResults []mail.MessageSummary
	message       mail.Message
	unread        mail.UnreadCounts
	sendID        string
	draftID       string
	markUpdated   int

	lastSearchQuery mail.SearchQuery
	lastMessageID   string
	lastIncludeBody bool
	lastMarkRead    bool

	err error
}

func (f *fakeMailClient) ListAccounts(ctx context.Context) ([]mail.Account, error) {
	return f.accounts, f.err
}
func (f *fakeMailClient) ListMailboxes(ctx context.Context, account string) ([]mail.Mailbox, error) {
	return f.mailboxes, f.err
}
func (f *fakeMailClient) SearchMessages(ctx context.Context, q mail.SearchQuery) ([]mail.MessageSummary, error) {
	f.lastSearchQuery = q
	return f.searchResults, f.err
}
func (f *fakeMailClient) GetMessage(ctx context.Context, messageID string, includeBody bool) (mail.Message, error) {
	f.lastMessageID = messageID
	f.lastIncludeBody = includeBody
	return f.message, f.err
}
func (f *fakeMailClient) SendEmail(ctx context.Context, input mail.ComposeInput) (string, error) {
	return f.sendID, f.err
}
func (f *fakeMailClient) CreateDraft(ctx context.Context, input mail.ComposeInput) (string, error) {
	return f.draftID, f.err
}
func (f *fakeMailClient) MarkAsRead(ctx context.Context, messageIDs []string, read bool) (int, error) {
	f.lastMarkRead = read
	return f.markUpdated, f.err
}
func (f *fakeMailClient) GetUnreadCount(ctx context.Context, account string) (mail.UnreadCounts, error) {
	return f.unread, f.err
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestListAccountsHandler(t *testing.T) {
	fake := &fakeMailClient{accounts: []mail.Account{{Name: "Personal"}, {Name: "Work"}}}
	h := newListAccountsHandler(fake, testLogger())

	_, out, err := h(context.Background(), nil, listAccountsInput{})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if len(out.Accounts) != 2 || out.Accounts[0].Name != "Personal" || out.Accounts[1].Name != "Work" {
		t.Fatalf("unexpected accounts output: %#v", out)
	}
}

func TestSearchMessagesHandlerParsesDatesAndDefaults(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	fake := &fakeMailClient{searchResults: []mail.MessageSummary{{ID: "1", Date: now}}}
	h := newSearchMessagesHandler(fake, testLogger())

	from := "2026-01-02T03:04:05Z"
	to := "2026-01-03T03:04:05Z"
	_, out, err := h(context.Background(), nil, searchMessagesInput{Account: "Personal", DateFrom: from, DateTo: to})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if len(out.Messages) != 1 || out.Messages[0].ID != "1" {
		t.Fatalf("unexpected output: %#v", out)
	}
	if fake.lastSearchQuery.Limit != 0 {
		t.Fatalf("expected handler to pass raw limit through, got %d", fake.lastSearchQuery.Limit)
	}
	if fake.lastSearchQuery.DateFrom == nil || fake.lastSearchQuery.DateTo == nil {
		t.Fatalf("expected parsed dates to be set")
	}
	if out.Messages[0].Date == "" {
		t.Fatalf("expected RFC3339 date output, got empty")
	}
}

func TestSearchMessagesHandlerReturnsEmptyDateForZeroTime(t *testing.T) {
	fake := &fakeMailClient{searchResults: []mail.MessageSummary{{ID: "1"}}}
	h := newSearchMessagesHandler(fake, testLogger())

	_, out, err := h(context.Background(), nil, searchMessagesInput{Account: "Personal"})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if len(out.Messages) != 1 {
		t.Fatalf("unexpected output: %#v", out)
	}
	if out.Messages[0].Date != "" {
		t.Fatalf("expected empty date for zero time, got %q", out.Messages[0].Date)
	}
}

func TestSearchMessagesHandlerRejectsInvalidDate(t *testing.T) {
	fake := &fakeMailClient{}
	h := newSearchMessagesHandler(fake, testLogger())

	_, _, err := h(context.Background(), nil, searchMessagesInput{Account: "Personal", DateFrom: "not-a-date"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetMessageHandlerDefaultsIncludeBodyTrue(t *testing.T) {
	fake := &fakeMailClient{message: mail.Message{ID: "1"}}
	h := newGetMessageHandler(fake, testLogger())

	_, out, err := h(context.Background(), nil, getMessageInput{MessageID: "1"})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if out.Message.ID != "1" {
		t.Fatalf("unexpected output: %#v", out)
	}
	if !fake.lastIncludeBody {
		t.Fatal("expected includeBody default true")
	}
}

func TestMarkAsReadHandlerDefaultsReadTrue(t *testing.T) {
	fake := &fakeMailClient{markUpdated: 2}
	h := newMarkAsReadHandler(fake, testLogger())

	_, out, err := h(context.Background(), nil, markAsReadInput{MessageIDs: []string{"1", "2"}})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if out.Updated != 2 {
		t.Fatalf("unexpected updated count: %d", out.Updated)
	}
	if !fake.lastMarkRead {
		t.Fatal("expected default read=true")
	}
}

func TestComposeHandlersPassThroughErrors(t *testing.T) {
	fake := &fakeMailClient{err: errors.New("boom")}
	sendHandler := newSendEmailHandler(fake, testLogger())
	draftHandler := newCreateDraftHandler(fake, testLogger())

	if _, _, err := sendHandler(context.Background(), nil, sendEmailInput{To: []string{"alice@example.com"}, Subject: "s", Body: "b"}); err == nil {
		t.Fatal("expected send error")
	}
	if _, _, err := draftHandler(context.Background(), nil, createDraftInput{To: []string{"alice@example.com"}, Subject: "s", Body: "b"}); err == nil {
		t.Fatal("expected draft error")
	}
}
