package mailserver

import (
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-message/mail"
	"github.com/soulteary/owlmail/internal/types"
)

// EmailQuery describes a mailbox snapshot query. DateTo is the inclusive
// upper boundary used by the existing HTTP API, and Limit is applied after
// filtering and sorting.
type EmailQuery struct {
	Text string
	// SearchAddresses includes sender and recipient names and addresses in the
	// free-text match. ExcludeHTML keeps compatibility queries aligned with
	// MailDev, whose summary search covers the plain-text body but not HTML.
	SearchAddresses bool
	ExcludeHTML     bool
	From            string
	To              string
	DateFrom        *time.Time
	DateTo          *time.Time
	Read            *bool
	SortBy          string
	SortOrder       string
	Offset          int
	Limit           int
	// MatchStoreEmail applies an additional filter while the mailbox read lock
	// is held. The callback must not retain or mutate the supplied store email.
	// It lets compatibility layers filter on their own DTO fields before full
	// messages are cloned for the selected page.
	MatchStoreEmail func(*Email) bool
}

// EmailPreview is the detached, lightweight representation returned for a
// mailbox preview query.
type EmailPreview struct {
	ID            string    `json:"id"`
	Time          time.Time `json:"time"`
	Read          bool      `json:"read"`
	Subject       string    `json:"subject"`
	From          string    `json:"from"`
	To            []string  `json:"to"`
	Size          int64     `json:"size"`
	SizeHuman     string    `json:"sizeHuman"`
	HasAttachment bool      `json:"hasAttachment"`
	Preview       string    `json:"preview"`
}

// EmailSummaryAddress is a detached address used by lightweight summary
// projections that need both the display name and address.
type EmailSummaryAddress struct {
	Name    string
	Address string
}

// EmailSummary is a detached MailDev-capable summary projection. It contains
// no HTML, headers, envelope, or attachment metadata, so summary requests do
// not clone full messages before discarding those fields.
type EmailSummary struct {
	ID              string
	Time            time.Time
	ReceivedAt      time.Time
	Read            bool
	Subject         string
	Size            int64
	SizeHuman       string
	From            []EmailSummaryAddress
	To              []EmailSummaryAddress
	CC              []EmailSummaryAddress
	AttachmentCount int
	Text            string
	EnvelopeFrom    string
	EnvelopeTo      []string
	SMTPEnvelope    bool
}

// QueryEmails returns detached full-email snapshots for one page and the
// total number of matches before pagination. The store lock is released while
// filtering and sorting potentially large bodies, then reacquired only to
// deep-clone the selected page.
func (ms *MailServer) QueryEmails(query EmailQuery) ([]*Email, int) {
	page, total := ms.queryEmailPage(query)

	ms.storeMutex.RLock()
	defer ms.storeMutex.RUnlock()
	emails := make([]*Email, 0, len(page))
	for _, entry := range page {
		// source is an internal identity handle captured under storeMutex. It is
		// never dereferenced without the lock and never leaves this package.
		email := cloneEmail(entry.source)
		// Keep the response consistent with the read state used by filtering if
		// a concurrent mark-as-read completed between the two lock sections.
		email.Read = entry.read
		emails = append(emails, email)
	}
	return emails, total
}

// QueryEmailPreviews returns detached summaries for one page and the total
// number of matches before pagination. No full Email snapshot is created.
func (ms *MailServer) QueryEmailPreviews(query EmailQuery) ([]EmailPreview, int) {
	page, total := ms.queryEmailPage(query)
	ms.snapshotPreviewQueryAddresses(page)
	previews := make([]EmailPreview, 0, len(page))
	for _, entry := range page {
		previews = append(previews, makeEmailPreview(entry))
	}
	return previews, total
}

// GetEmailPreview returns one detached summary using the same snapshot and
// formatting boundary as QueryEmailPreviews.
func (ms *MailServer) GetEmailPreview(id string) (EmailPreview, bool) {
	ms.storeMutex.RLock()
	email, exists := ms.storeByID[id]
	if !exists {
		ms.storeMutex.RUnlock()
		return EmailPreview{}, false
	}
	entry := snapshotEmailQueryEntry(email, true, true)
	ms.storeMutex.RUnlock()
	return makeEmailPreview(entry), true
}

// QueryEmailSummaries returns lightweight detached summary projections for
// one page. Unlike QueryEmails, it never clones complete message bodies,
// headers, envelopes, or attachment records.
func (ms *MailServer) QueryEmailSummaries(query EmailQuery) ([]EmailSummary, int) {
	page, total := ms.queryEmailPage(query)
	ms.snapshotSummaryQueryAddresses(page)
	summaries := make([]EmailSummary, 0, len(page))
	for _, entry := range page {
		summaries = append(summaries, makeEmailSummary(entry))
	}
	return summaries, total
}

type emailQueryAddress struct {
	name    string
	address string
}

// emailQueryEntry carries a lightweight, detached view of the fields needed by
// mailbox filtering, sorting, and previews. Strings are immutable, so copying
// their headers avoids copying message bodies while address values and slices
// are detached from the store object graph. source is an opaque internal handle
// that is dereferenced only while storeMutex is held.
type emailQueryEntry struct {
	source          *types.Email
	id              string
	time            time.Time
	receivedAt      time.Time
	read            bool
	subject         string
	from            []emailQueryAddress
	to              []emailQueryAddress
	cc              []emailQueryAddress
	calculatedBCC   []emailQueryAddress
	envelopeFrom    string
	envelopeTo      []string
	smtpEnvelope    bool
	text            string
	html            string
	size            int64
	sizeHuman       string
	hasAttachment   bool
	attachmentCount int
	sortKey         string
}

func (ms *MailServer) queryEmailPage(query EmailQuery) ([]emailQueryEntry, int) {
	if page, total, ok := ms.queryIndexedEmailPage(query); ok {
		return page, total
	}
	entries := ms.snapshotEmailQueryEntries(query)
	compiled := compileEmailQuery(query)
	matches := entries[:0]
	for _, entry := range entries {
		if compiled.matches(entry) {
			matches = append(matches, entry)
		}
	}

	sortEmailMatches(matches, query.SortBy, query.SortOrder)
	total := len(matches)
	start, end := pageBounds(total, query.Offset, query.Limit)
	return matches[start:end], total
}

func (ms *MailServer) snapshotEmailQueryEntries(query EmailQuery) []emailQueryEntry {
	ms.storeMutex.RLock()
	defer ms.storeMutex.RUnlock()

	searchAddresses := query.SearchAddresses && query.Text != ""
	needFrom := query.From != "" || query.SortBy == "from" || searchAddresses
	needTo := query.To != "" || searchAddresses
	entries := make([]emailQueryEntry, 0, len(ms.storeOrder))
	for _, id := range ms.storeOrder {
		if email, exists := ms.storeByID[id]; exists {
			if query.MatchStoreEmail != nil && !query.MatchStoreEmail(email) {
				continue
			}
			entry := snapshotEmailQueryEntry(email, needFrom, needTo)
			entry.receivedAt = ms.receivedAtByID[id]
			entries = append(entries, entry)
		}
	}
	return entries
}

func snapshotEmailQueryEntry(email *types.Email, needFrom, needTo bool) emailQueryEntry {
	entry := emailQueryEntry{
		source:          email,
		id:              email.ID,
		time:            email.Time,
		read:            email.Read,
		subject:         email.Subject,
		text:            email.Text,
		html:            email.HTML,
		size:            email.Size,
		sizeHuman:       email.SizeHuman,
		hasAttachment:   len(email.Attachments) > 0,
		attachmentCount: len(email.Attachments),
	}
	if needFrom {
		entry.from = snapshotQueryAddresses(email.From)
	}
	if needTo {
		entry.to = snapshotQueryAddresses(email.To)
		entry.cc = snapshotQueryAddresses(email.CC)
		entry.calculatedBCC = snapshotQueryAddresses(email.CalculatedBCC)
	}
	return entry
}

func (ms *MailServer) snapshotSummaryQueryAddresses(entries []emailQueryEntry) {
	ms.storeMutex.RLock()
	defer ms.storeMutex.RUnlock()
	for i := range entries {
		entries[i].from = snapshotQueryAddresses(entries[i].source.From)
		entries[i].to = snapshotQueryAddresses(entries[i].source.To)
		entries[i].cc = snapshotQueryAddresses(entries[i].source.CC)
		if entries[i].source.Envelope != nil {
			entries[i].envelopeFrom = entries[i].source.Envelope.From
			entries[i].envelopeTo = append([]string(nil), entries[i].source.Envelope.To...)
			entries[i].smtpEnvelope = entries[i].source.Envelope.SMTPTransaction
		}
	}
}

func (ms *MailServer) snapshotPreviewQueryAddresses(entries []emailQueryEntry) {
	ms.storeMutex.RLock()
	defer ms.storeMutex.RUnlock()
	for i := range entries {
		entries[i].from = snapshotQueryAddresses(entries[i].source.From)
		entries[i].to = snapshotQueryAddresses(entries[i].source.To)
	}
}

func snapshotQueryAddresses(addresses []*mail.Address) []emailQueryAddress {
	if len(addresses) == 0 {
		return nil
	}
	snapshot := make([]emailQueryAddress, 0, len(addresses))
	for _, address := range addresses {
		if address != nil {
			snapshot = append(snapshot, emailQueryAddress{name: address.Name, address: address.Address})
		}
	}
	return snapshot
}

type compiledEmailQuery struct {
	text            string
	searchAddresses bool
	excludeHTML     bool
	from            string
	to              string
	dateFrom        *time.Time
	dateTo          *time.Time
	read            *bool
}

func compileEmailQuery(query EmailQuery) compiledEmailQuery {
	return compiledEmailQuery{
		text:            strings.ToLower(query.Text),
		searchAddresses: query.SearchAddresses,
		excludeHTML:     query.ExcludeHTML,
		from:            strings.ToLower(query.From),
		to:              strings.ToLower(query.To),
		dateFrom:        query.DateFrom,
		dateTo:          query.DateTo,
		read:            query.Read,
	}
}

func (query compiledEmailQuery) matches(email emailQueryEntry) bool {
	if query.text != "" {
		matched := strings.Contains(strings.ToLower(email.subject), query.text) ||
			strings.Contains(strings.ToLower(email.text), query.text)
		if !matched && !query.excludeHTML {
			matched = strings.Contains(strings.ToLower(email.html), query.text)
		}
		if !matched && query.searchAddresses {
			matched = queryAddressesContain(email.from, query.text, true) ||
				queryAddressesContain(email.to, query.text, true) ||
				queryAddressesContain(email.cc, query.text, true)
		}
		if !matched {
			return false
		}
	}
	if query.from != "" && !queryAddressesContain(email.from, query.from, true) {
		return false
	}
	if query.to != "" &&
		!queryAddressesContain(email.to, query.to, true) &&
		!queryAddressesContain(email.cc, query.to, true) &&
		!queryAddressesContain(email.calculatedBCC, query.to, false) {
		return false
	}
	if query.dateFrom != nil && email.time.Before(*query.dateFrom) {
		return false
	}
	if query.dateTo != nil && email.time.After(*query.dateTo) {
		return false
	}
	return query.read == nil || email.read == *query.read
}

func queryAddressesContain(addresses []emailQueryAddress, needle string, includeName bool) bool {
	for _, address := range addresses {
		if strings.Contains(strings.ToLower(address.address), needle) ||
			(includeName && strings.Contains(strings.ToLower(address.name), needle)) {
			return true
		}
	}
	return false
}

func sortEmailMatches(emails []emailQueryEntry, sortBy, sortOrder string) {
	ascending := sortOrder == "asc"
	switch sortBy {
	case "store":
		if sortOrder == "desc" {
			for left, right := 0, len(emails)-1; left < right; left, right = left+1, right-1 {
				emails[left], emails[right] = emails[right], emails[left]
			}
		}
		return
	case "":
		sort.Slice(emails, func(i, j int) bool {
			return emails[i].time.After(emails[j].time)
		})
	case "time":
		sort.Slice(emails, func(i, j int) bool {
			if ascending {
				return emails[i].time.Before(emails[j].time)
			}
			return emails[i].time.After(emails[j].time)
		})
	case "subject":
		for i := range emails {
			emails[i].sortKey = strings.ToLower(emails[i].subject)
		}
		sort.Slice(emails, func(i, j int) bool {
			if ascending {
				return emails[i].sortKey < emails[j].sortKey
			}
			return emails[i].sortKey > emails[j].sortKey
		})
	case "from":
		for i := range emails {
			emails[i].sortKey = firstQueryAddress(emails[i].from)
		}
		sort.Slice(emails, func(i, j int) bool {
			if ascending {
				return emails[i].sortKey < emails[j].sortKey
			}
			return emails[i].sortKey > emails[j].sortKey
		})
	case "size":
		sort.Slice(emails, func(i, j int) bool {
			if ascending {
				return emails[i].size < emails[j].size
			}
			return emails[i].size > emails[j].size
		})
	}
}

func firstQueryAddress(addresses []emailQueryAddress) string {
	if len(addresses) == 0 {
		return ""
	}
	return strings.ToLower(addresses[0].address)
}

func pageBounds(total, offset, limit int) (int, int) {
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	if limit <= 0 {
		return offset, offset
	}
	if limit > total-offset {
		return offset, total
	}
	return offset, offset + limit
}

func makeEmailPreview(email emailQueryEntry) EmailPreview {
	preview := EmailPreview{
		ID:            email.id,
		Time:          email.time,
		Read:          email.read,
		Subject:       email.subject,
		To:            make([]string, 0, len(email.to)),
		Size:          email.size,
		SizeHuman:     email.sizeHuman,
		HasAttachment: email.hasAttachment,
	}
	if len(email.from) > 0 {
		preview.From = email.from[0].address
	}
	for _, address := range email.to {
		preview.To = append(preview.To, address.address)
	}

	previewText := email.text
	if previewText == "" {
		previewText = email.html
		previewText = strings.ReplaceAll(previewText, "<", " <")
		previewText = strings.ReplaceAll(previewText, ">", "> ")
		previewText = strings.ReplaceAll(previewText, "\n", " ")
		previewText = strings.ReplaceAll(previewText, "\r", " ")
		for strings.Contains(previewText, "  ") {
			previewText = strings.ReplaceAll(previewText, "  ", " ")
		}
		previewText = strings.TrimSpace(previewText)
	}
	if len(previewText) > 200 {
		previewText = previewText[:200] + "..."
	}
	preview.Preview = previewText
	return preview
}

func makeEmailSummary(email emailQueryEntry) EmailSummary {
	return EmailSummary{
		ID:              email.id,
		Time:            email.time,
		ReceivedAt:      email.receivedAt,
		Read:            email.read,
		Subject:         email.subject,
		Size:            email.size,
		SizeHuman:       email.sizeHuman,
		From:            makeEmailSummaryAddresses(email.from),
		To:              makeEmailSummaryAddresses(email.to),
		CC:              makeEmailSummaryAddresses(email.cc),
		AttachmentCount: email.attachmentCount,
		Text:            email.text,
		EnvelopeFrom:    email.envelopeFrom,
		EnvelopeTo:      email.envelopeTo,
		SMTPEnvelope:    email.smtpEnvelope,
	}
}

func makeEmailSummaryAddresses(addresses []emailQueryAddress) []EmailSummaryAddress {
	if len(addresses) == 0 {
		return nil
	}
	result := make([]EmailSummaryAddress, 0, len(addresses))
	for _, address := range addresses {
		result = append(result, EmailSummaryAddress{Name: address.name, Address: address.address})
	}
	return result
}
