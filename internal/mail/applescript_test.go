package mail

import (
	"strings"
	"testing"
	"time"
)

func TestQuoteAS(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain", in: "hello", want: `"hello"`},
		{name: "quote and slash", in: "a\"b\\c", want: `"a\"b\\c"`},
		{name: "newline and tab", in: "a\n\tb", want: `"a\n\tb"`},
		{name: "control chars removed", in: "a\x01b\x7fc", want: `"abc"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := quoteAS(tt.in); got != tt.want {
				t.Fatalf("quoteAS() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestQuoteASMultiline(t *testing.T) {
	got := quoteASMultiline("line1\nline2\r\nline3")
	want := `"line1" & linefeed & "line2" & linefeed & "line3"`
	if got != want {
		t.Fatalf("quoteASMultiline() = %q, want %q", got, want)
	}
}

func TestBuildSearchMessagesScriptIncludesFilters(t *testing.T) {
	from := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	script := buildSearchMessagesScript(SearchQuery{
		Account:         "Personal",
		Mailbox:         "INBOX",
		SenderContains:  "alice",
		SubjectContains: "invoice",
		UnreadOnly:      true,
		DateFrom:        &from,
		DateTo:          &to,
		Limit:           10,
	})

	checks := []string{
		`set accountName to "Personal"`,
		`set mailboxName to "INBOX"`,
		`set maxResults to 10`,
		`sender of it contains "alice"`,
		`subject of it contains "invoice"`,
		`read status of it is false`,
		`every message of targetMailbox whose`,
		`repeat with msg in (every message of targetMailbox whose`,
		`set targetMailbox to missing value`,
		`repeat with mb in every mailbox of acc`,
		`(mbName contains mailboxName) or (mailboxName contains mbName)`,
		`set row to (id of msg as text)`,
		`& fieldSep & mailboxName`,
		`set collectedCount to collectedCount + 1`,
		`return outText`,
	}
	for _, c := range checks {
		if !strings.Contains(script, c) {
			t.Fatalf("script missing expected fragment %q", c)
		}
	}
}

func TestBuildListAccountsScriptUsesDelimitedJoin(t *testing.T) {
	script := buildListAccountsScript()
	if !strings.Contains(script, `repeat with i from 1 to (count of namesList)`) {
		t.Fatalf("expected record separator join, got: %s", script)
	}
}

func TestBuildSendOrDraftScriptUsesLinefeedConcatenation(t *testing.T) {
	script := buildSendOrDraftScript(ComposeInput{
		To:      []string{"alice@example.com"},
		Subject: "subject",
		Body:    "line1\nline2",
	}, false)

	if !strings.Contains(script, `"line1" & linefeed & "line2"`) {
		t.Fatalf("script body does not use linefeed concatenation: %s", script)
	}
	if !strings.Contains(script, "send newMessage") {
		t.Fatalf("script should send message")
	}
}

func TestBuildGetMessageScriptReadsBodyInsideTell(t *testing.T) {
	script := buildGetMessageScript("123", "Personal", "INBOX", true)
	if !strings.Contains(script, `set includeBody to true`) {
		t.Fatalf("expected includeBody toggle in script")
	}
	if !strings.Contains(script, `set accountName to "Personal"`) {
		t.Fatalf("expected account hint in script")
	}
	if !strings.Contains(script, `set mailboxName to "INBOX"`) {
		t.Fatalf("expected mailbox hint in script")
	}
	if !strings.Contains(script, `set msgBody to (content of msg as text)`) {
		t.Fatalf("expected body extraction inside Mail tell block")
	}
	if !strings.Contains(script, `set matches to (every message of mb whose id is messageIDNum)`) {
		t.Fatalf("expected mailbox-scoped lookup for message id")
	}
	if strings.Contains(script, `set matches to (every message whose id is messageIDNum)`) {
		t.Fatalf("unexpected global message-id lookup")
	}
	if strings.Contains(script, `& (content of msg as text) &`) {
		t.Fatalf("body must not be read inline outside tell block")
	}
	if !strings.Contains(script, `& fieldSep & msgBody & fieldSep &`) {
		t.Fatalf("expected final row to include msgBody variable")
	}
}
