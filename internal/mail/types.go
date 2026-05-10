package mail

import "time"

const (
	FieldSep  = "\x1f" // Unit Separator
	RecordSep = "\x1e" // Record Separator
	ListSep   = "\x1d" // Group Separator
)

type Account struct {
	Name string `json:"name"`
}

type Mailbox struct {
	Account     string `json:"account"`
	Name        string `json:"name"`
	UnreadCount int    `json:"unread_count"`
}

type MessageSummary struct {
	ID      string    `json:"id"`
	Subject string    `json:"subject"`
	Sender  string    `json:"sender"`
	Date    time.Time `json:"date"`
	Read    bool      `json:"read"`
	Mailbox string    `json:"mailbox"`
}

type Message struct {
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

type SearchQuery struct {
	Account         string
	Mailbox         string
	SenderContains  string
	SubjectContains string
	UnreadOnly      bool
	DateFrom        *time.Time
	DateTo          *time.Time
	Limit           int
}

type ComposeInput struct {
	To      []string
	Subject string
	Body    string
	CC      []string
	BCC     []string
	Account string
}

type UnreadCounts struct {
	Account   string         `json:"account,omitempty"`
	Total     int            `json:"total"`
	Mailboxes []MailboxCount `json:"mailboxes"`
}

type MailboxCount struct {
	Account string `json:"account"`
	Mailbox string `json:"mailbox"`
	Unread  int    `json:"unread"`
}
