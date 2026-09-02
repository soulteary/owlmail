package mailserver

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/emersion/go-message/mail"
)

func TestMailboxQueryFiltersSortsAndPaginates(t *testing.T) {
	server, base := newMailboxQueryTestServer(t)
	defer func() { _ = server.Close() }()

	read := false
	dateFrom := base.Add(24 * time.Hour)
	dateTo := base.Add(36 * time.Hour)
	tests := []struct {
		name  string
		query EmailQuery
		want  []string
	}{
		{name: "text and HTML", query: EmailQuery{Text: "html needle", Limit: 10}, want: []string{"id-2"}},
		{name: "from name", query: EmailQuery{From: "alice sender", Limit: 10}, want: []string{"id-1"}},
		{name: "to address", query: EmailQuery{To: "team@example.test", Limit: 10}, want: []string{"id-1"}},
		{name: "cc name", query: EmailQuery{To: "copy recipient", Limit: 10}, want: []string{"id-1"}},
		{name: "calculated bcc address", query: EmailQuery{To: "hidden@example.test", Limit: 10}, want: []string{"id-1"}},
		{name: "date from", query: EmailQuery{DateFrom: &dateFrom, Limit: 10}, want: []string{"id-3", "id-2"}},
		{name: "date to", query: EmailQuery{DateTo: &dateTo, Limit: 10}, want: []string{"id-2", "id-1"}},
		{name: "read state", query: EmailQuery{Read: &read, Limit: 10}, want: []string{"id-3", "id-1"}},
		{name: "time ascending", query: EmailQuery{SortBy: "time", SortOrder: "asc", Limit: 10}, want: []string{"id-1", "id-2", "id-3"}},
		{name: "subject descending", query: EmailQuery{SortBy: "subject", SortOrder: "desc", Limit: 10}, want: []string{"id-3", "id-2", "id-1"}},
		{name: "from ascending", query: EmailQuery{SortBy: "from", SortOrder: "asc", Limit: 10}, want: []string{"id-1", "id-2", "id-3"}},
		{name: "size ascending", query: EmailQuery{SortBy: "size", SortOrder: "asc", Limit: 10}, want: []string{"id-2", "id-3", "id-1"}},
		{name: "store descending", query: EmailQuery{SortBy: "store", SortOrder: "desc", Limit: 10}, want: []string{"id-3", "id-2", "id-1"}},
		{name: "unknown sort preserves store order", query: EmailQuery{SortBy: "unknown", SortOrder: "asc", Limit: 10}, want: []string{"id-1", "id-2", "id-3"}},
		{name: "pagination after filtering", query: EmailQuery{Text: "needle", SortBy: "subject", SortOrder: "asc", Offset: 1, Limit: 1}, want: []string{"id-2"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			emails, _ := server.QueryEmails(test.query)
			if got := emailIDs(emails); fmt.Sprint(got) != fmt.Sprint(test.want) {
				t.Fatalf("IDs = %v, want %v", got, test.want)
			}
		})
	}

	emails, total := server.QueryEmails(EmailQuery{Text: "needle", SortBy: "subject", SortOrder: "asc", Offset: 1, Limit: 1})
	if total != 2 || len(emails) != 1 || emails[0].ID != "id-2" {
		t.Fatalf("paginated result = total %d, emails %v", total, emailIDs(emails))
	}
	emails, total = server.QueryEmails(EmailQuery{Offset: math.MaxInt, Limit: math.MaxInt})
	if total != 3 || emails == nil || len(emails) != 0 {
		t.Fatalf("out-of-range result = total %d, emails %#v", total, emails)
	}
}

func TestMailboxQueryReturnsDetachedResults(t *testing.T) {
	server, _ := newMailboxQueryTestServer(t)
	defer func() { _ = server.Close() }()

	query := EmailQuery{SortBy: "unknown", Limit: 1}
	emails, total := server.QueryEmails(query)
	if total != 3 || len(emails) != 1 {
		t.Fatalf("unexpected query result: total %d, emails %d", total, len(emails))
	}
	emails[0].Subject = "caller mutation"
	emails[0].From[0].Address = "mutated@example.test"
	emails[0].Attachments[0].FileName = "mutated.txt"
	emails[0].Envelope.To[0] = "mutated@example.test"
	emails[0].Headers["X-Trace"].([]string)[0] = "mutated"

	again, _ := server.QueryEmails(query)
	if again[0].Subject != "Alpha" || again[0].From[0].Address != "alice@example.test" {
		t.Fatalf("full query exposed mutable store state: %#v", again[0])
	}
	if again[0].Attachments[0].FileName != "alpha.txt" || again[0].Envelope.To[0] != "team@example.test" {
		t.Fatalf("nested full-query state was not detached: %#v", again[0])
	}
	if again[0].Headers["X-Trace"].([]string)[0] != "alpha" {
		t.Fatalf("header state was not detached: %#v", again[0].Headers)
	}

	previews, total := server.QueryEmailPreviews(query)
	if total != 3 || len(previews) != 1 || len(previews[0].To) != 1 {
		t.Fatalf("unexpected preview result: total %d, previews %#v", total, previews)
	}
	previews[0].To[0] = "mutated@example.test"
	previewsAgain, _ := server.QueryEmailPreviews(query)
	if previewsAgain[0].To[0] != "team@example.test" {
		t.Fatalf("preview query exposed mutable store state: %#v", previewsAgain[0])
	}
	previewByID, exists := server.GetEmailPreview("id-1")
	if !exists || previewByID.ID != "id-1" || len(previewByID.To) != 1 {
		t.Fatalf("preview by ID = %#v, %v", previewByID, exists)
	}
	previewByID.To[0] = "mutated@example.test"
	previewByIDAgain, _ := server.GetEmailPreview("id-1")
	if previewByIDAgain.To[0] != "team@example.test" {
		t.Fatalf("preview by ID exposed mutable store state: %#v", previewByIDAgain)
	}
}

func TestMailboxQueryPreviewBuildsOnlySummaryFields(t *testing.T) {
	server, _ := newMailboxQueryTestServer(t)
	defer func() { _ = server.Close() }()

	previews, total := server.QueryEmailPreviews(EmailQuery{SortBy: "unknown", Offset: 1, Limit: 1})
	if total != 3 || len(previews) != 1 {
		t.Fatalf("preview result = total %d, previews %#v", total, previews)
	}
	preview := previews[0]
	if preview.ID != "id-2" || preview.From != "bob@example.test" || fmt.Sprint(preview.To) != "[other@example.test]" {
		t.Fatalf("preview addressing = %#v", preview)
	}
	if !preview.HasAttachment || preview.Size != 10 || preview.SizeHuman != "10 bytes" {
		t.Fatalf("preview metadata = %#v", preview)
	}
	if preview.Preview == "" || strings.Contains(preview.Preview, "\n") {
		t.Fatalf("preview text = %q", preview.Preview)
	}
}

func TestMailboxQueryBuildsLightweightMailDevSummaries(t *testing.T) {
	server, _ := newMailboxQueryTestServer(t)
	defer func() { _ = server.Close() }()

	summaries, total := server.QueryEmailSummaries(EmailQuery{SortBy: "store", Offset: 0, Limit: 1})
	if total != 3 || len(summaries) != 1 {
		t.Fatalf("summary result = total %d, summaries %#v", total, summaries)
	}
	summary := summaries[0]
	if summary.ID != "id-1" || summary.Subject != "Alpha" || summary.Text != "plain needle" {
		t.Fatalf("summary identity/body projection = %#v", summary)
	}
	if len(summary.From) != 1 || summary.From[0].Name != "Alice Sender" || summary.From[0].Address != "alice@example.test" {
		t.Fatalf("summary sender projection = %#v", summary.From)
	}
	if len(summary.To) != 1 || len(summary.CC) != 1 || summary.AttachmentCount != 1 {
		t.Fatalf("summary recipients/attachments = %#v", summary)
	}
}

func TestMailboxQueryAppliesStorePredicateBeforePagination(t *testing.T) {
	server, _ := newMailboxQueryTestServer(t)
	defer func() { _ = server.Close() }()

	matched := 0
	emails, total := server.QueryEmails(EmailQuery{
		SortBy: "store", Offset: 1, Limit: 1,
		MatchStoreEmail: func(email *Email) bool {
			if strings.Contains(email.Subject, "a") {
				matched++
				return true
			}
			return false
		},
	})
	if matched != 3 || total != 3 || len(emails) != 1 || emails[0].ID != "id-2" {
		t.Fatalf("predicate/page result = matched %d, total %d, IDs %v", matched, total, emailIDs(emails))
	}
}

func TestMailboxQueryConcurrentWrites(t *testing.T) {
	server, _ := newMailboxQueryTestServer(t)
	defer func() { _ = server.Close() }()

	const iterations = 80
	start := make(chan struct{})
	errors := make(chan error, 8)
	var workers sync.WaitGroup

	workers.Add(1)
	go func() {
		defer workers.Done()
		<-start
		for i := 0; i < iterations; i++ {
			id := fmt.Sprintf("writer-%03d", i)
			email := &Email{
				Subject: id,
				Time:    time.Unix(int64(i), 0).UTC(),
				From:    []*mail.Address{{Address: "writer@example.test"}},
				To:      []*mail.Address{{Address: "reader@example.test"}},
				Text:    "concurrent needle",
			}
			if err := server.SaveEmailToStore(id, false, &Envelope{To: []string{"reader@example.test"}}, email); err != nil {
				errors <- fmt.Errorf("save %s: %w", id, err)
				return
			}
			if i%2 == 1 {
				deleteID := fmt.Sprintf("writer-%03d", i-1)
				if err := server.DeleteEmail(deleteID); err != nil {
					errors <- fmt.Errorf("delete %s: %w", deleteID, err)
					return
				}
			}
		}
	}()

	workers.Add(1)
	go func() {
		defer workers.Done()
		<-start
		for i := 0; i < iterations*2; i++ {
			if err := server.ReadEmail(fmt.Sprintf("id-%d", i%3+1)); err != nil {
				errors <- fmt.Errorf("mark read: %w", err)
				return
			}
		}
	}()

	for reader := 0; reader < 4; reader++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			for i := 0; i < iterations*2; i++ {
				emails, total := server.QueryEmails(EmailQuery{Text: "needle", Offset: i % 7, Limit: 5})
				if len(emails) > 5 || total < len(emails) {
					errors <- fmt.Errorf("invalid full result: total %d, page %d", total, len(emails))
					return
				}
				previews, previewTotal := server.QueryEmailPreviews(EmailQuery{SortBy: "subject", SortOrder: "asc", Limit: 5})
				if len(previews) > 5 || previewTotal < len(previews) {
					errors <- fmt.Errorf("invalid preview result: total %d, page %d", previewTotal, len(previews))
					return
				}
			}
		}()
	}

	close(start)
	workers.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}

func newMailboxQueryTestServer(t *testing.T) (*MailServer, time.Time) {
	t.Helper()
	server, err := NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)
	emails := []struct {
		id    string
		read  bool
		email *Email
	}{
		{id: "id-1", email: &Email{
			Subject: "Alpha", Time: base, Text: "plain needle", Size: 30, SizeHuman: "30 B",
			From:          []*mail.Address{{Name: "Alice Sender", Address: "alice@example.test"}},
			To:            []*mail.Address{{Address: "team@example.test"}},
			CC:            []*mail.Address{{Name: "Copy Recipient", Address: "copy@example.test"}},
			CalculatedBCC: []*mail.Address{{Address: "hidden@example.test"}},
			Attachments:   []*Attachment{{FileName: "alpha.txt"}},
			Headers:       map[string]interface{}{"X-Trace": []string{"alpha"}},
		}},
		{id: "id-2", read: true, email: &Email{
			Subject: "Beta", Time: base.Add(24 * time.Hour), HTML: "<b>html needle</b>\n", Size: 10, SizeHuman: "10 B",
			From:        []*mail.Address{{Name: "Bob Sender", Address: "bob@example.test"}},
			To:          []*mail.Address{{Address: "other@example.test"}},
			Attachments: []*Attachment{{FileName: "beta.txt"}},
			Headers:     map[string]interface{}{"X-Trace": []string{"beta"}},
		}},
		{id: "id-3", email: &Email{
			Subject: "Gamma", Time: base.Add(48 * time.Hour), Text: "different", Size: 20, SizeHuman: "20 B",
			From:    []*mail.Address{{Name: "Carol Sender", Address: "carol@example.test"}},
			To:      []*mail.Address{{Address: "receiver@example.test"}},
			Headers: map[string]interface{}{"X-Trace": []string{"gamma"}},
		}},
	}
	for _, item := range emails {
		if err := os.WriteFile(filepath.Join(server.mailDir, item.id+".eml"), make([]byte, item.email.Size), 0644); err != nil {
			_ = server.Close()
			t.Fatal(err)
		}
		envelopeTo := []string{item.email.To[0].Address}
		if item.id == "id-1" {
			envelopeTo = append(envelopeTo, "hidden@example.test")
		}
		envelope := &Envelope{From: item.email.From[0].Address, To: envelopeTo}
		if err := server.SaveEmailToStore(item.id, item.read, envelope, item.email); err != nil {
			_ = server.Close()
			t.Fatal(err)
		}
	}
	return server, base
}

func emailIDs(emails []*Email) []string {
	ids := make([]string, 0, len(emails))
	for _, email := range emails {
		ids = append(ids, email.ID)
	}
	return ids
}

var (
	benchmarkEmailsSink   []*Email
	benchmarkPreviewsSink []EmailPreview
	benchmarkTotalSink    int
)

func BenchmarkMailboxQuery10K(b *testing.B) {
	server := benchmarkMailbox(10_000)
	query := EmailQuery{Text: "needle", SortBy: "subject", SortOrder: "asc", Offset: 5_000, Limit: 50}

	b.Run("before/full-get-all-then-paginate", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			benchmarkEmailsSink, benchmarkTotalSink = legacyMailboxQuery(server, query)
		}
	})
	b.Run("after/full-paginate-then-clone", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			benchmarkEmailsSink, benchmarkTotalSink = server.QueryEmails(query)
		}
	})
	b.Run("before/preview-get-all-then-paginate", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			emails, total := legacyMailboxQuery(server, query)
			previews := make([]EmailPreview, 0, len(emails))
			for _, email := range emails {
				previews = append(previews, makeEmailPreview(snapshotEmailQueryEntry(email, true, true)))
			}
			benchmarkPreviewsSink, benchmarkTotalSink = previews, total
		}
	})
	b.Run("after/preview-paginate-then-summarize", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			benchmarkPreviewsSink, benchmarkTotalSink = server.QueryEmailPreviews(query)
		}
	})
}

func benchmarkMailbox(count int) *MailServer {
	server := &MailServer{
		storeByID:      make(map[string]*Email, count),
		storeOrder:     make([]string, 0, count),
		receivedAtByID: make(map[string]time.Time, count),
	}
	text := "mailbox benchmark needle " + strings.Repeat("body ", 100)
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("benchmark-%05d", i)
		email := &Email{
			ID:            id,
			Time:          time.Unix(int64(i), 0).UTC(),
			Read:          i%2 == 0,
			Subject:       fmt.Sprintf("Subject %05d", count-i),
			From:          []*mail.Address{{Name: "Sender", Address: "sender@example.test"}},
			To:            []*mail.Address{{Name: "Recipient", Address: "recipient@example.test"}},
			CC:            []*mail.Address{{Address: "copy@example.test"}},
			CalculatedBCC: []*mail.Address{{Address: "hidden@example.test"}},
			Text:          text,
			Attachments: []*Attachment{
				{FileName: "one.txt", GeneratedFileName: "one.txt"},
				{FileName: "two.txt", GeneratedFileName: "two.txt"},
			},
			Envelope:  &Envelope{From: "sender@example.test", To: []string{"recipient@example.test"}},
			Size:      int64(i),
			SizeHuman: "1 KiB",
			Headers: map[string]interface{}{
				"Received": []string{"first", "second"},
				"Nested":   map[string]interface{}{"Values": []interface{}{"one", "two"}},
			},
		}
		server.storeByID[id] = email
		server.storeOrder = append(server.storeOrder, id)
	}
	return server
}

func legacyMailboxQuery(server *MailServer, query EmailQuery) ([]*Email, int) {
	emails := server.GetAllEmail()
	matches := make([]*Email, 0, len(emails))
	for _, email := range emails {
		if legacyEmailMatches(email, query) {
			matches = append(matches, email)
		}
	}
	legacySortEmailMatches(matches, query.SortBy, query.SortOrder)
	start, end := pageBounds(len(matches), query.Offset, query.Limit)
	return matches[start:end], len(matches)
}

func legacyEmailMatches(email *Email, query EmailQuery) bool {
	if query.Text != "" {
		queryLower := strings.ToLower(query.Text)
		if !strings.Contains(strings.ToLower(email.Subject), queryLower) &&
			!strings.Contains(strings.ToLower(email.Text), queryLower) &&
			!strings.Contains(strings.ToLower(email.HTML), queryLower) {
			return false
		}
	}
	if query.From != "" && !legacyAddressesContain(email.From, strings.ToLower(query.From), true) {
		return false
	}
	if query.To != "" {
		toLower := strings.ToLower(query.To)
		if !legacyAddressesContain(email.To, toLower, true) &&
			!legacyAddressesContain(email.CC, toLower, true) &&
			!legacyAddressesContain(email.CalculatedBCC, toLower, false) {
			return false
		}
	}
	if query.DateFrom != nil && email.Time.Before(*query.DateFrom) {
		return false
	}
	if query.DateTo != nil && email.Time.After(*query.DateTo) {
		return false
	}
	return query.Read == nil || email.Read == *query.Read
}

func legacyAddressesContain(addresses []*mail.Address, needle string, includeName bool) bool {
	for _, address := range addresses {
		if address == nil {
			continue
		}
		if strings.Contains(strings.ToLower(address.Address), needle) ||
			(includeName && strings.Contains(strings.ToLower(address.Name), needle)) {
			return true
		}
	}
	return false
}

func legacySortEmailMatches(emails []*Email, sortBy, sortOrder string) {
	ascending := sortOrder == "asc"
	switch sortBy {
	case "":
		sort.Slice(emails, func(i, j int) bool {
			return emails[i].Time.After(emails[j].Time)
		})
	case "time":
		sort.Slice(emails, func(i, j int) bool {
			if ascending {
				return emails[i].Time.Before(emails[j].Time)
			}
			return emails[i].Time.After(emails[j].Time)
		})
	case "subject":
		sort.Slice(emails, func(i, j int) bool {
			subjectI := strings.ToLower(emails[i].Subject)
			subjectJ := strings.ToLower(emails[j].Subject)
			if ascending {
				return subjectI < subjectJ
			}
			return subjectI > subjectJ
		})
	case "from":
		sort.Slice(emails, func(i, j int) bool {
			fromI, fromJ := "", ""
			if len(emails[i].From) > 0 && emails[i].From[0] != nil {
				fromI = strings.ToLower(emails[i].From[0].Address)
			}
			if len(emails[j].From) > 0 && emails[j].From[0] != nil {
				fromJ = strings.ToLower(emails[j].From[0].Address)
			}
			if ascending {
				return fromI < fromJ
			}
			return fromI > fromJ
		})
	case "size":
		sort.Slice(emails, func(i, j int) bool {
			if ascending {
				return emails[i].Size < emails[j].Size
			}
			return emails[i].Size > emails[j].Size
		})
	}
}
