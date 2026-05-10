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
		`repeat with i from 1 to (count of rows)`,
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
