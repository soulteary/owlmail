package mailserver

import (
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-message/mail"
)

// EmailQuery describes a mailbox snapshot query.
type EmailQuery struct {
	Query     string
	From      string
	To        string
	DateFrom  string
	DateTo    string
	Read      string
	SortBy    string
	SortOrder string
	Offset    int
	Limit     int
}

// EmailPreview is a lightweight, detached mailbox-list value.
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

type queryAddress struct {
	name    string
	address string
}

type emailQueryRecord struct {
	source        *Email
	id            string
	time          time.Time
	read          bool
	subject       string
	from          []queryAddress
	to            []queryAddress
	cc            []queryAddress
	calculatedBCC []queryAddress
	text          string
	html          string
	size          int64
	sizeHuman     string
	hasAttachment bool
}

// QueryEmails returns deep copies of only the selected page. The lightweight
// query snapshot is captured under the store lock; body scans and sorting do
// not block mailbox writers.
func (ms *MailServer) QueryEmails(query EmailQuery) ([]*Email, int) {
	page, total := paginateEmailRecords(ms.queryEmailSnapshot(), query)

	ms.storeMutex.RLock()
	defer ms.storeMutex.RUnlock()
	result := make([]*Email, 0, len(page))
	for _, record := range page {
		email := cloneEmail(record.source)
		// Read state can change between the lightweight snapshot and cloning.
		// Preserve the state that was used to select this page.
		email.Read = record.read
		result = append(result, email)
	}
	return result, total
}

// QueryEmailPreviews returns detached summaries without cloning message bodies,
// headers or attachment structures.
func (ms *MailServer) QueryEmailPreviews(query EmailQuery) ([]EmailPreview, int) {
	page, total := paginateEmailRecords(ms.queryEmailSnapshot(), query)
	result := make([]EmailPreview, 0, len(page))
	for _, record := range page {
		preview := EmailPreview{
			ID:            record.id,
			Time:          record.time,
			Read:          record.read,
			Subject:       record.subject,
			Size:          record.size,
			SizeHuman:     record.sizeHuman,
			HasAttachment: record.hasAttachment,
			To:            make([]string, 0, len(record.to)),
			Preview:       emailPreviewText(record.text, record.html),
		}
		if len(record.from) > 0 {
			preview.From = record.from[0].address
		}
		for _, address := range record.to {
			preview.To = append(preview.To, address.address)
		}
		result = append(result, preview)
	}
	return result, total
}

func (ms *MailServer) queryEmailSnapshot() []emailQueryRecord {
	ms.storeMutex.RLock()
	defer ms.storeMutex.RUnlock()

	records := make([]emailQueryRecord, 0, len(ms.storeOrder))
	for _, id := range ms.storeOrder {
		email, exists := ms.storeByID[id]
		if !exists {
			continue
		}
		records = append(records, emailQueryRecord{
			source:        email,
			id:            email.ID,
			time:          email.Time,
			read:          email.Read,
			subject:       email.Subject,
			from:          snapshotAddresses(email.From),
			to:            snapshotAddresses(email.To),
			cc:            snapshotAddresses(email.CC),
			calculatedBCC: snapshotAddresses(email.CalculatedBCC),
			text:          email.Text,
			html:          email.HTML,
			size:          email.Size,
			sizeHuman:     email.SizeHuman,
			hasAttachment: len(email.Attachments) > 0,
		})
	}
	return records
}

func snapshotAddresses(addresses []*mail.Address) []queryAddress {
	result := make([]queryAddress, 0, len(addresses))
	for _, address := range addresses {
		if address != nil {
			result = append(result, queryAddress{name: address.Name, address: address.Address})
		}
	}
	return result
}

func paginateEmailRecords(records []emailQueryRecord, query EmailQuery) ([]emailQueryRecord, int) {
	matched := records[:0]
	for _, record := range records {
		if emailMatchesQuery(record, query) {
			matched = append(matched, record)
		}
	}
	sortEmailQueryResults(matched, query.SortBy, query.SortOrder)
	total := len(matched)
	start := query.Offset
	if start < 0 {
		start = 0
	}
	if start > total {
		start = total
	}
	limit := query.Limit
	if limit < 0 {
		limit = 0
	}
	end := start + limit
	if end > total {
		end = total
	}
	return matched[start:end], total
}

func emailMatchesQuery(email emailQueryRecord, query EmailQuery) bool {
	if query.Query != "" {
		needle := strings.ToLower(query.Query)
		if !strings.Contains(strings.ToLower(email.subject), needle) &&
			!strings.Contains(strings.ToLower(email.text), needle) &&
			!strings.Contains(strings.ToLower(email.html), needle) {
			return false
		}
	}
	if query.From != "" && !addressListContains(email.from, query.From, true) {
		return false
	}
	if query.To != "" && !addressListContains(email.to, query.To, true) &&
		!addressListContains(email.cc, query.To, true) &&
		!addressListContains(email.calculatedBCC, query.To, false) {
		return false
	}
	if query.DateFrom != "" {
		if from, err := time.Parse("2006-01-02", query.DateFrom); err == nil && email.time.Before(from) {
			return false
		}
	}
	if query.DateTo != "" {
		if to, err := time.Parse("2006-01-02", query.DateTo); err == nil && email.time.After(to.Add(24*time.Hour)) {
			return false
		}
	}
	if query.Read != "" && email.read != (query.Read == "true") {
		return false
	}
	return true
}

func addressListContains(addresses []queryAddress, value string, includeName bool) bool {
	needle := strings.ToLower(value)
	for _, address := range addresses {
		if strings.Contains(strings.ToLower(address.address), needle) ||
			(includeName && strings.Contains(strings.ToLower(address.name), needle)) {
			return true
		}
	}
	return false
}

func sortEmailQueryResults(emails []emailQueryRecord, sortBy, sortOrder string) {
	ascending := sortOrder == "asc"
	var less func(i, j int) bool
	switch sortBy {
	case "":
		less = func(i, j int) bool { return emails[i].time.After(emails[j].time) }
	case "time":
		less = func(i, j int) bool {
			if ascending {
				return emails[i].time.Before(emails[j].time)
			}
			return emails[i].time.After(emails[j].time)
		}
	case "subject":
		less = func(i, j int) bool {
			a := strings.ToLower(emails[i].subject)
			b := strings.ToLower(emails[j].subject)
			if ascending {
				return a < b
			}
			return a > b
		}
	case "from":
		less = func(i, j int) bool {
			a, b := "", ""
			if len(emails[i].from) > 0 {
				a = strings.ToLower(emails[i].from[0].address)
			}
			if len(emails[j].from) > 0 {
				b = strings.ToLower(emails[j].from[0].address)
			}
			if ascending {
				return a < b
			}
			return a > b
		}
	case "size":
		less = func(i, j int) bool {
			if ascending {
				return emails[i].size < emails[j].size
			}
			return emails[i].size > emails[j].size
		}
	}
	if less != nil {
		sort.Slice(emails, less)
	}
}

func emailPreviewText(text, html string) string {
	if text == "" {
		text = strings.NewReplacer("<", " <", ">", "> ", "\n", " ", "\r", " ").Replace(html)
		for strings.Contains(text, "  ") {
			text = strings.ReplaceAll(text, "  ", " ")
		}
		text = strings.TrimSpace(text)
	}
	if len(text) > 200 {
		text = text[:200] + "..."
	}
	return text
}
