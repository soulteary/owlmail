package mailserver

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestScopedWaitCapacityIsPerMailboxAndCleansCounters(t *testing.T) {
	server := isolatedServer(t, t.TempDir())
	a, err := server.MailboxView(uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	b, err := server.MailboxView(uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	cursor, err := a.Cursor()
	if err != nil {
		t.Fatal(err)
	}
	server.storeMutex.Lock()
	server.scopedWaiters = map[string]int{a.id: 8}
	server.storeMutex.Unlock()
	if _, err = a.Wait(context.Background(), cursor, DeliveryFilter{}, 1, time.Second); !errors.Is(err, ErrWaitCapacity) {
		t.Fatal("personal wait capacity not enforced")
	}
	otherCursor, err := b.Cursor()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = b.Wait(ctx, otherCursor, DeliveryFilter{}, 1, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatal("other mailbox was starved", err)
	}
	server.storeMutex.RLock()
	defer server.storeMutex.RUnlock()
	if _, exists := server.scopedWaiters[b.id]; exists || server.deliveryWaiters != 0 {
		t.Fatal("wait counters leaked")
	}
}
