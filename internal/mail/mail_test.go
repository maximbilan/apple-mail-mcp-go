package mail

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

type fakeRunner struct {
	runFunc func(context.Context, string) (string, error)
}

func (f *fakeRunner) Run(ctx context.Context, script string) (string, error) {
	if f.runFunc == nil {
		return "", nil
	}
	return f.runFunc(ctx, script)
}

func newTestClient(fn func(context.Context, string) (string, error)) *Client {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewClient(&fakeRunner{runFunc: fn}, logger)
}

func TestListAccounts(t *testing.T) {
	c := newTestClient(func(ctx context.Context, script string) (string, error) {
		return "Personal" + RecordSep + "Work", nil
	})
	accounts, err := c.ListAccounts(context.Background())
	if err != nil {
		t.Fatalf("ListAccounts error: %v", err)
	}
	if len(accounts) != 2 || accounts[0].Name != "Personal" || accounts[1].Name != "Work" {
		t.Fatalf("unexpected accounts: %#v", accounts)
	}
}

func TestListMailboxesValidation(t *testing.T) {
	c := newTestClient(nil)
	if _, err := c.ListMailboxes(context.Background(), ""); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestSearchMessagesParsesRows(t *testing.T) {
	row := strings.Join([]string{"1", "Subject", "alice@example.com", "1700000000", "false", "INBOX"}, FieldSep)
	c := newTestClient(func(ctx context.Context, script string) (string, error) {
		return row, nil
	})
	msgs, err := c.SearchMessages(context.Background(), SearchQuery{Account: "Personal"})
	if err != nil {
		t.Fatalf("SearchMessages error: %v", err)
	}
	if len(msgs) != 1 || msgs[0].ID != encodeMessageRef("Personal", "INBOX", "1") || msgs[0].Read {
		t.Fatalf("unexpected messages: %#v", msgs)
	}
	if msgs[0].Date.IsZero() {
		t.Fatal("expected parsed date")
	}
}

func TestSearchMessagesFallsBackWhenDateSentFails(t *testing.T) {
	calls := 0
	c := newTestClient(func(ctx context.Context, script string) (string, error) {
		calls++
		if calls == 1 {
			return "", &ExecError{ExitCode: 1, Stderr: "Mail got an error: Can't get date sent. (-1728)", Script: script}
		}
		within := strings.Join([]string{"1", "Within", "alice@example.com", "1700000000", "false", "INBOX"}, FieldSep)
		outside := strings.Join([]string{"2", "Outside", "bob@example.com", "1600000000", "false", "INBOX"}, FieldSep)
		return within + RecordSep + outside, nil
	})

	from := time.Unix(1699990000, 0).UTC()
	to := time.Unix(1700005000, 0).UTC()
	msgs, err := c.SearchMessages(context.Background(), SearchQuery{
		Account:  "Personal",
		DateFrom: &from,
		DateTo:   &to,
		Limit:    50,
	})
	if err != nil {
		t.Fatalf("SearchMessages fallback error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected two script calls, got %d", calls)
	}
	if len(msgs) != 1 || msgs[0].ID != encodeMessageRef("Personal", "INBOX", "1") {
		t.Fatalf("unexpected filtered fallback messages: %#v", msgs)
	}
}

func TestSearchMessagesDateRangeValidation(t *testing.T) {
	from := time.Now()
	to := from.Add(-time.Hour)
	c := newTestClient(nil)
	_, err := c.SearchMessages(context.Background(), SearchQuery{Account: "Personal", DateFrom: &from, DateTo: &to})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestGetMessageValidationAndParse(t *testing.T) {
	c := newTestClient(func(ctx context.Context, script string) (string, error) {
		fields := []string{"7", "Subj", "Sender", "alice@example.com" + ListSep + "bob@example.com", "carol@example.com", "1700000000", "Body", "INBOX", "true", "false"}
		return strings.Join(fields, FieldSep), nil
	})
	msg, err := c.GetMessage(context.Background(), "7", true)
	if err != nil {
		t.Fatalf("GetMessage error: %v", err)
	}
	if msg.ID != "7" || len(msg.Recipients) != 2 || len(msg.CC) != 1 || !msg.Read || msg.Flagged {
		t.Fatalf("unexpected message: %#v", msg)
	}

	if _, err := c.GetMessage(context.Background(), "abc", true); err == nil {
		t.Fatal("expected invalid id error")
	}
}

func TestValidateComposeInput(t *testing.T) {
	base := ComposeInput{To: []string{"alice@example.com"}, Subject: "sub", Body: "body"}
	if err := validateComposeInput(base); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	bad := base
	bad.To = []string{"bad"}
	if err := validateComposeInput(bad); err == nil {
		t.Fatal("expected invalid email error")
	}
}

func TestMarkAsReadValidationAndParse(t *testing.T) {
	c := newTestClient(func(ctx context.Context, script string) (string, error) {
		return "3", nil
	})
	id1 := encodeMessageRef("Personal", "INBOX", "1")
	id2 := encodeMessageRef("Personal", "INBOX", "2")
	count, err := c.MarkAsRead(context.Background(), []string{id1, id2}, true)
	if err != nil {
		t.Fatalf("MarkAsRead error: %v", err)
	}
	if count != 3 {
		t.Fatalf("unexpected count %d", count)
	}
	if _, err := c.MarkAsRead(context.Background(), []string{}, true); err == nil {
		t.Fatal("expected empty ids error")
	}
}

func TestDecodeMessageRef(t *testing.T) {
	ref, err := decodeMessageRef(encodeMessageRef("Google", "INBOX", "42"))
	if err != nil {
		t.Fatalf("decodeMessageRef error: %v", err)
	}
	if ref.Account != "Google" || ref.Mailbox != "INBOX" || ref.NumericID != "42" {
		t.Fatalf("unexpected decoded ref: %#v", ref)
	}
	legacy, err := decodeMessageRef("7")
	if err != nil {
		t.Fatalf("legacy decode error: %v", err)
	}
	if legacy.NumericID != "7" {
		t.Fatalf("unexpected legacy decode: %#v", legacy)
	}
}

func TestGetUnreadCountParsesRows(t *testing.T) {
	row1 := strings.Join([]string{"Personal", "INBOX", "4"}, FieldSep)
	row2 := strings.Join([]string{"Work", "Archive", "2"}, FieldSep)
	c := newTestClient(func(ctx context.Context, script string) (string, error) {
		return "6" + RecordSep + row1 + RecordSep + row2, nil
	})
	counts, err := c.GetUnreadCount(context.Background(), "")
	if err != nil {
		t.Fatalf("GetUnreadCount error: %v", err)
	}
	if counts.Total != 6 || len(counts.Mailboxes) != 2 {
		t.Fatalf("unexpected counts: %#v", counts)
	}
}

func TestRunScriptExecError(t *testing.T) {
	execErr := &ExecError{ExitCode: 1, Stderr: "boom", Script: "test"}
	c := newTestClient(func(ctx context.Context, script string) (string, error) {
		return "", execErr
	})
	_, err := c.ListAccounts(context.Background())
	if !errors.Is(err, execErr) {
		t.Fatalf("expected exec error, got %v", err)
	}
}
