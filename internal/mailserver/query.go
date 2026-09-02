package mailserver

import (
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-message/mail"
)

// EmailQuery describes a mailbox snapshot query. Filtering, sorting and
// pagination are evaluated while the store read lock is held, so Total and the
// returned page describe one consistent mailbox state.
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

// QueryEmails returns deep copies of only the selected page.
func (ms *MailServer) QueryEmails(query EmailQuery) ([]*Email, int) {
	ms.storeMutex.RLock()
	defer ms.storeMutex.RUnlock()

	page, total := ms.queryEmailPageLocked(query)
	result := make([]*Email, 0, len(page))
	for _, email := range page {
		result = append(result, cloneEmail(email))
	}
	return result, total
}

// QueryEmailPreviews returns detached summaries without cloning message bodies,
// headers or attachment structures.
func (ms *MailServer) QueryEmailPreviews(query EmailQuery) ([]EmailPreview, int) {
	ms.storeMutex.RLock()
	defer ms.storeMutex.RUnlock()

	page, total := ms.queryEmailPageLocked(query)
	result := make([]EmailPreview, 0, len(page))
	for _, email := range page {
		preview := EmailPreview{
			ID:            email.ID,
			Time:          email.Time,
			Read:          email.Read,
			Subject:       email.Subject,
			Size:          email.Size,
			SizeHuman:     email.SizeHuman,
			HasAttachment: len(email.Attachments) > 0,
			To:            make([]string, 0, len(email.To)),
		}
		if len(email.From) > 0 {
			preview.From = email.From[0].Address
		}
		for _, address := range email.To {
			preview.To = append(preview.To, address.Address)
		}
		preview.Preview = emailPreviewText(email)
		result = append(result, preview)
	}
	return result, total
}

func (ms *MailServer) queryEmailPageLocked(query EmailQuery) ([]*Email, int) {
	matched := make([]*Email, 0, len(ms.storeOrder))
	for _, id := range ms.storeOrder {
		email, exists := ms.storeByID[id]
		if exists && emailMatchesQuery(email, query) {
			matched = append(matched, email)
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

func emailMatchesQuery(email *Email, query EmailQuery) bool {
	if query.Query != "" {
		needle := strings.ToLower(query.Query)
		if !strings.Contains(strings.ToLower(email.Subject), needle) &&
			!strings.Contains(strings.ToLower(email.Text), needle) &&
			!strings.Contains(strings.ToLower(email.HTML), needle) {
			return false
		}
	}
	if query.From != "" && !addressListContains(email.From, query.From, true) {
		return false
	}
	if query.To != "" && !addressListContains(email.To, query.To, true) &&
		!addressListContains(email.CC, query.To, true) &&
		!addressListContains(email.CalculatedBCC, query.To, false) {
		return false
	}
	if query.DateFrom != "" {
		if from, err := time.Parse("2006-01-02", query.DateFrom); err == nil && email.Time.Before(from) {
			return false
		}
	}
	if query.DateTo != "" {
		if to, err := time.Parse("2006-01-02", query.DateTo); err == nil && email.Time.After(to.Add(24*time.Hour)) {
			return false
		}
	}
	if query.Read != "" && email.Read != (query.Read == "true") {
		return false
	}
	return true
}

func addressListContains(addresses []*mail.Address, value string, includeName bool) bool {
	needle := strings.ToLower(value)
	for _, address := range addresses {
		if address != nil && (strings.Contains(strings.ToLower(address.Address), needle) ||
			(includeName && strings.Contains(strings.ToLower(address.Name), needle))) {
			return true
		}
	}
	return false
}

func sortEmailQueryResults(emails []*Email, sortBy, sortOrder string) {
	ascending := sortOrder == "asc"
	var less func(i, j int) bool
	switch sortBy {
	case "":
		less = func(i, j int) bool { return emails[i].Time.After(emails[j].Time) }
	case "time":
		less = func(i, j int) bool {
			if ascending {
				return emails[i].Time.Before(emails[j].Time)
			}
			return emails[i].Time.After(emails[j].Time)
		}
	case "subject":
		less = func(i, j int) bool {
			a := strings.ToLower(emails[i].Subject)
			b := strings.ToLower(emails[j].Subject)
			if ascending {
				return a < b
			}
			return a > b
		}
	case "from":
		less = func(i, j int) bool {
			a, b := "", ""
			if len(emails[i].From) > 0 {
				a = strings.ToLower(emails[i].From[0].Address)
			}
			if len(emails[j].From) > 0 {
				b = strings.ToLower(emails[j].From[0].Address)
			}
			if ascending {
				return a < b
			}
			return a > b
		}
	case "size":
		less = func(i, j int) bool {
			if ascending {
				return emails[i].Size < emails[j].Size
			}
			return emails[i].Size > emails[j].Size
		}
	}
	if less != nil {
		sort.Slice(emails, less)
	}
}

func emailPreviewText(email *Email) string {
	text := email.Text
	if text == "" {
		text = strings.NewReplacer("<", " <", ">", "> ", "\n", " ", "\r", " ").Replace(email.HTML)
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
