package mailserver

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/emersion/go-message/mail"
)

func queryTestServer(count int) *MailServer {
	server := &MailServer{
		storeByID: make(map[string]*Email, count),
		storeOrder: make([]string, 0, count),
		receivedAtByID: make(map[string]time.Time, count),
	}
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("mail-%05d", i)
		email := &Email{
			ID: id, Time: time.Date(2026, 1, 1, 0, i, 0, 0, time.UTC),
			Read: i%2 == 0, Subject: fmt.Sprintf("subject %05d", i),
			From: []*mail.Address{{Name: "Sender", Address: fmt.Sprintf("sender%d@example.test", i%3)}},
			To: []*mail.Address{{Address: fmt.Sprintf("recipient%d@example.test", i%4)}},
			Text: strings.Repeat("message body ", 128), Size: int64(i), SizeHuman: fmt.Sprintf("%d B", i),
			Headers: map[string]interface{}{"X-Test": []string{"one", "two"}},
		}
		server.storeByID[id] = email
		server.storeOrder = append(server.storeOrder, id)
	}
	return server
}

func TestQueryEmailsFiltersSortsAndPaginates(t *testing.T) {
	server := queryTestServer(12)
	page, total := server.QueryEmails(EmailQuery{
		From: "sender1", To: "recipient1", Read: "false",
		SortBy: "size", SortOrder: "desc", Offset: 1, Limit: 2,
	})
	if total != 1 || len(page) != 0 {
		t.Fatalf("total, page length = %d, %d; want 1, 0", total, len(page))
	}

	page, total = server.QueryEmails(EmailQuery{Query: "body", SortBy: "subject", SortOrder: "desc", Offset: 2, Limit: 3})
	if total != 12 || len(page) != 3 {
		t.Fatalf("total, page length = %d, %d; want 12, 3", total, len(page))
	}
	if page[0].ID != "mail-00009" || page[2].ID != "mail-00007" {
		t.Fatalf("unexpected page IDs: %s ... %s", page[0].ID, page[2].ID)
	}
}

func TestQueryEmailsReturnsDetachedPage(t *testing.T) {
	server := queryTestServer(4)
	page, _ := server.QueryEmails(EmailQuery{Limit: 1})
	page[0].Subject = "mutated"
	page[0].From[0].Address = "mutated@example.test"
	page[0].Headers["X-Test"].([]string)[0] = "mutated"

	again, _ := server.QueryEmails(EmailQuery{Limit: 1})
	if again[0].Subject == "mutated" || again[0].From[0].Address == "mutated@example.test" ||
		again[0].Headers["X-Test"].([]string)[0] == "mutated" {
		t.Fatal("query exposed mutable store state")
	}
}

func TestQueryEmailPreviewsDuringConcurrentWrites(t *testing.T) {
	server := queryTestServer(100)
	var writers sync.WaitGroup
	writers.Add(1)
	go func() {
		defer writers.Done()
		for i := 100; i < 300; i++ {
			id := fmt.Sprintf("mail-%05d", i)
			server.storeMutex.Lock()
			server.storeByID[id] = &Email{ID: id, Time: time.Now(), Subject: id, Text: "body"}
			server.storeOrder = append(server.storeOrder, id)
			server.storeMutex.Unlock()
		}
	}()
	for i := 0; i < 200; i++ {
		previews, total := server.QueryEmailPreviews(EmailQuery{Offset: i % 10, Limit: 25})
		if total < len(previews) {
			t.Fatalf("total %d is smaller than page %d", total, len(previews))
		}
	}
	writers.Wait()
}

func BenchmarkMailboxQuery10K(b *testing.B) {
	server := queryTestServer(10_000)
	query := EmailQuery{Offset: 5_000, Limit: 50}
	b.Run("before_clone_all", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			server.storeMutex.RLock()
			all := make([]*Email, 0, len(server.storeOrder))
			for _, id := range server.storeOrder {
				all = append(all, cloneEmail(server.storeByID[id]))
			}
			server.storeMutex.RUnlock()
			sort.Slice(all, func(i, j int) bool { return all[i].Time.After(all[j].Time) })
			_ = all[query.Offset : query.Offset+query.Limit]
		}
	})
	b.Run("after_paginate_then_clone", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = server.QueryEmails(query)
		}
	})
	b.Run("after_preview_without_body_clone", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = server.QueryEmailPreviews(query)
		}
	})
}
