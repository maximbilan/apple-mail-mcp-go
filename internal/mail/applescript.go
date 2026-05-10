package mail

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	defaultMailbox = "INBOX"
	maxSearchLimit = 200
)

// AppleScript payload format:
// - records are joined by ASCII RS (0x1e)
// - fields within each record are joined by ASCII US (0x1f)
// - list-valued fields are joined by ASCII GS (0x1d)

func quoteAS(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 || r == 0x7f {
				continue
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func quoteASMultiline(s string) string {
	normalized := strings.ReplaceAll(s, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	parts := strings.Split(normalized, "\n")
	if len(parts) == 0 {
		return quoteAS("")
	}
	quoted := make([]string, 0, len(parts))
	for _, p := range parts {
		quoted = append(quoted, quoteAS(p))
	}
	return strings.Join(quoted, " & linefeed & ")
}

func asPrelude() string {
	return strings.TrimSpace(`
set fieldSep to (character id 31)
set recordSep to (character id 30)
set listSep to (character id 29)
`) + "\n"
}

func buildListAccountsScript() string {
	return asPrelude() + strings.TrimSpace(`
tell application "Mail"
	set namesList to name of every account
end tell
if (count of namesList) is 0 then
	return ""
end if
set outText to ""
repeat with i from 1 to (count of namesList)
	if i > 1 then set outText to outText & recordSep
	set outText to outText & (item i of namesList as text)
end repeat
return outText
`)
}

func buildListMailboxesScript(account string) string {
	return asPrelude() + fmt.Sprintf(strings.TrimSpace(`
set accountName to %s

tell application "Mail"
	if not (exists account accountName) then
		error "account_not_found:" & accountName
	end if
	set rows to {}
	repeat with mb in every mailbox of account accountName
		set row to (name of mb as text) & fieldSep & ((unread count of mb) as text)
		set end of rows to row
	end repeat
end tell
if (count of rows) is 0 then
	return ""
end if
set outText to ""
repeat with i from 1 to (count of rows)
	if i > 1 then set outText to outText & recordSep
	set outText to outText & (item i of rows as text)
end repeat
return outText
`), quoteAS(account))
}

func buildSearchMessagesScript(q SearchQuery) string {
	mailbox := q.Mailbox
	if mailbox == "" {
		mailbox = defaultMailbox
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > maxSearchLimit {
		limit = maxSearchLimit
	}

	filters := make([]string, 0, 6)
	if q.SenderContains != "" {
		filters = append(filters, fmt.Sprintf("(sender of it contains %s)", quoteAS(q.SenderContains)))
	}
	if q.SubjectContains != "" {
		filters = append(filters, fmt.Sprintf("(subject of it contains %s)", quoteAS(q.SubjectContains)))
	}
	if q.UnreadOnly {
		filters = append(filters, "(read status of it is false)")
	}
	if q.DateFrom != nil {
		filters = append(filters, fmt.Sprintf("(((date sent of it) - epochDate) >= %d)", q.DateFrom.Unix()))
	}
	if q.DateTo != nil {
		filters = append(filters, fmt.Sprintf("(((date sent of it) - epochDate) <= %d)", q.DateTo.Unix()))
	}

	whereClause := ""
	if len(filters) > 0 {
		whereClause = " whose " + strings.Join(filters, " and ")
	}
	messageQuery := "messages of targetMailbox"
	if whereClause != "" {
		messageQuery = "every message of targetMailbox" + whereClause
	}

	return asPrelude() + fmt.Sprintf(strings.TrimSpace(`
set accountName to %s
set mailboxName to %s
set maxResults to %d
set epochDate to current date
set year of epochDate to 1970
set month of epochDate to 1
set day of epochDate to 1
set time of epochDate to 0

tell application "Mail"
	if not (exists account accountName) then
		error "account_not_found:" & accountName
	end if
	set acc to account accountName
	set targetMailbox to missing value
	if targetMailbox is missing value then
		ignoring case
			if mailboxName is "inbox" then
				try
					set targetMailbox to inbox of acc
				end try
			end if
		end ignoring
	end if
	if targetMailbox is missing value then
		try
			set targetMailbox to mailbox mailboxName of acc
		end try
	end if
	if targetMailbox is missing value then
		repeat with mb in every mailbox of acc
			try
				set mbName to (name of mb as text)
				ignoring case
					if (mbName is mailboxName) or (mbName contains mailboxName) or (mailboxName contains mbName) then
						set targetMailbox to mb
						exit repeat
					end if
				end ignoring
			end try
		end repeat
	end if
	if targetMailbox is missing value then
		error "mailbox_not_found:" & mailboxName
	end if
	set rows to {}
	set collectedCount to 0
	repeat with msg in (%s)
		if collectedCount >= maxResults then exit repeat
		try
			set unixDate to ""
			try
				set sentDate to date sent of msg
				if sentDate is not missing value then
					set unixDate to (((sentDate - epochDate) as integer) as text)
				end if
			on error
				set unixDate to ""
			end try
			if unixDate is "" then
				try
					set recvDate to date received of msg
					if recvDate is not missing value then
						set unixDate to (((recvDate - epochDate) as integer) as text)
					end if
				end try
			end if
			set row to (id of msg as text) & fieldSep & (subject of msg as text) & fieldSep & (sender of msg as text) & fieldSep & unixDate & fieldSep & ((read status of msg) as text) & fieldSep & mailboxName
			set end of rows to row
			set collectedCount to collectedCount + 1
		end try
	end repeat
end tell
if (count of rows) is 0 then
	return ""
end if
set outText to ""
repeat with i from 1 to (count of rows)
	if i > 1 then set outText to outText & recordSep
	set outText to outText & (item i of rows as text)
end repeat
return outText
`), quoteAS(q.Account), quoteAS(mailbox), limit, messageQuery)
}

func buildGetMessageScript(messageID, account, mailbox string, includeBody bool) string {
	includeBodyValue := "false"
	if includeBody {
		includeBodyValue = "true"
	}
	return asPrelude() + fmt.Sprintf(strings.TrimSpace(`
set messageIDText to %s
set messageIDNum to (messageIDText as integer)
set includeBody to %s
set accountName to %s
set mailboxName to %s

tell application "Mail"
	set msg to missing value
	if accountName is not "" then
		if exists account accountName then
			set accountList to {account accountName}
		else
			set accountList to {}
		end if
	else
		set accountList to every account
	end if

	repeat with acc in accountList
		if mailboxName is not "" then
			set candidateMailboxes to {}
			try
				set end of candidateMailboxes to (mailbox mailboxName of acc)
			end try
			if (count of candidateMailboxes) is 0 then
				repeat with mb in every mailbox of acc
					try
						set mbName to (name of mb as text)
						ignoring case
							if (mbName is mailboxName) or (mbName contains mailboxName) or (mailboxName contains mbName) then
								set end of candidateMailboxes to mb
								exit repeat
							end if
						end ignoring
					end try
				end repeat
			end if
			repeat with mb in candidateMailboxes
				try
					set matches to (every message of mb whose id is messageIDNum)
				on error
					set matches to {}
				end try
				if (count of matches) > 0 then
					set msg to item 1 of matches
					exit repeat
				end if
			end repeat
		else
			try
				set inboxMatches to (every message of inbox of acc whose id is messageIDNum)
			on error
				set inboxMatches to {}
			end try
			if (count of inboxMatches) > 0 then
				set msg to item 1 of inboxMatches
			else
				repeat with mb in every mailbox of acc
					try
						set matches to (every message of mb whose id is messageIDNum)
					on error
						set matches to {}
					end try
					if (count of matches) > 0 then
						set msg to item 1 of matches
						exit repeat
					end if
				end repeat
			end if
		end if
		if msg is not missing value then
			exit repeat
		end if
	end repeat
	if msg is missing value then
		error "message_not_found:" & messageIDText
	end if
	set epochDate to current date
	set year of epochDate to 1970
	set month of epochDate to 1
	set day of epochDate to 1
	set time of epochDate to 0
	set recipList to address of every to recipient of msg
	set ccList to address of every cc recipient of msg
	set msgID to (id of msg as text)
	set msgSubject to (subject of msg as text)
	set msgSender to (sender of msg as text)
	set msgUnixDate to ""
	try
		set sentDate to date sent of msg
		if sentDate is not missing value then
			set msgUnixDate to (((sentDate - epochDate) as integer) as text)
		end if
	end try
	if msgUnixDate is "" then
		try
			set recvDate to date received of msg
			if recvDate is not missing value then
				set msgUnixDate to (((recvDate - epochDate) as integer) as text)
			end if
		end try
	end if
	set msgMailbox to (name of mailbox of msg as text)
	set msgRead to ((read status of msg) as text)
	set msgFlagged to ((flagged status of msg) as text)
	set msgBody to ""
	if includeBody then
		try
			set msgBody to (content of msg as text)
		on error
			set msgBody to ""
		end try
	end if
end tell
if (count of recipList) is 0 then
	set recips to ""
else
	set recips to ""
	repeat with i from 1 to (count of recipList)
		if i > 1 then set recips to recips & listSep
		set recips to recips & (item i of recipList as text)
	end repeat
end if
if (count of ccList) is 0 then
	set ccRecips to ""
else
	set ccRecips to ""
	repeat with i from 1 to (count of ccList)
		if i > 1 then set ccRecips to ccRecips & listSep
		set ccRecips to ccRecips & (item i of ccList as text)
	end repeat
end if
set row to msgID & fieldSep & msgSubject & fieldSep & msgSender & fieldSep & recips & fieldSep & ccRecips & fieldSep & msgUnixDate & fieldSep & msgBody & fieldSep & msgMailbox & fieldSep & msgRead & fieldSep & msgFlagged
return row
`), quoteAS(messageID), includeBodyValue, quoteAS(account), quoteAS(mailbox))
}

func buildSendOrDraftScript(input ComposeInput, draft bool) string {
	toList := makeAppleScriptAddressLoop("to recipients", input.To)
	ccList := makeAppleScriptAddressLoop("cc recipients", input.CC)
	bccList := makeAppleScriptAddressLoop("bcc recipients", input.BCC)

	action := "send newMessage"
	if draft {
		action = "save newMessage"
	}

	accountSnippet := ""
	if input.Account != "" {
		accountSnippet = fmt.Sprintf(`
	if not (exists account %s) then
		error "account_not_found:" & %s
	end if
	set account of newMessage to account %s
`, quoteAS(input.Account), quoteAS(input.Account), quoteAS(input.Account))
	}

	return asPrelude() + fmt.Sprintf(strings.TrimSpace(`
set subjectText to %s
set contentText to %s

tell application "Mail"
	set newMessage to make new outgoing message with properties {visible:false, subject:subjectText, content:contentText}
	tell newMessage%s
%s
%s
%s
	end tell
	%s
	return (id of newMessage as text)
end tell
`), quoteAS(input.Subject), quoteASMultiline(input.Body), accountSnippet, toList, ccList, bccList, action)
}

func buildMarkAsReadScript(messageIDs []string, read bool) string {
	ids := make([]string, 0, len(messageIDs))
	for _, id := range messageIDs {
		ids = append(ids, quoteAS(id))
	}
	readText := "false"
	if read {
		readText = "true"
	}

	return asPrelude() + fmt.Sprintf(strings.TrimSpace(`
set readValue to %s
set idList to {%s}
set updatedCount to 0

tell application "Mail"
	repeat with idText in idList
		set idNum to (idText as integer)
		repeat with acc in every account
			repeat with mb in every mailbox of acc
				try
					set matches to (every message of mb whose id is idNum)
				on error
					set matches to {}
				end try
				repeat with msg in matches
					set read status of msg to readValue
					set updatedCount to updatedCount + 1
				end repeat
			end repeat
		end repeat
	end repeat
	return updatedCount as text
end tell
`), readText, strings.Join(ids, ", "))
}

func buildUnreadCountScript(account string) string {
	accountFilter := ""
	if account != "" {
		accountFilter = fmt.Sprintf(`
	if not (exists account %s) then
		error "account_not_found:" & %s
	end if
	set accountList to {account %s}
`, quoteAS(account), quoteAS(account), quoteAS(account))
	}

	if accountFilter == "" {
		accountFilter = "\n\tset accountList to every account\n"
	}

	return asPrelude() + strings.TrimSpace(fmt.Sprintf(`
set rows to {}
set totalUnread to 0

tell application "Mail"%s
	repeat with acc in accountList
		repeat with mb in every mailbox of acc
			set unreadCount to unread count of mb
			set totalUnread to totalUnread + unreadCount
			set row to (name of acc as text) & fieldSep & (name of mb as text) & fieldSep & (unreadCount as text)
			set end of rows to row
		end repeat
	end repeat
end tell
if (count of rows) is 0 then
	return totalUnread as text
end if
set rowsText to ""
repeat with i from 1 to (count of rows)
	if i > 1 then set rowsText to rowsText & recordSep
	set rowsText to rowsText & (item i of rows as text)
end repeat
return (totalUnread as text) & recordSep & rowsText
`, accountFilter))
}

func makeAppleScriptAddressLoop(recipientKind string, addrs []string) string {
	if len(addrs) == 0 {
		return ""
	}
	var b strings.Builder
	for i, addr := range addrs {
		name := "a" + strconv.Itoa(i)
		b.WriteString("\n\t\tset ")
		b.WriteString(name)
		b.WriteString(" to ")
		b.WriteString(quoteAS(addr))
		b.WriteString("\n\t\tmake new ")
		b.WriteString(strings.TrimSuffix(recipientKind, "s"))
		b.WriteString(" at end of ")
		b.WriteString(recipientKind)
		b.WriteString(" with properties {address:")
		b.WriteString(name)
		b.WriteString("}")
	}
	return b.String()
}
