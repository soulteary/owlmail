package mcpserver

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/types"
)

func TestRelayReceiptDetailProjectionCopiesAndOmitsAbsent(t *testing.T) {
	at := time.Now().UTC()
	email := &types.Email{RelayedAt: &at, RelayedTo: []string{"actual@example.test"}}
	detail := makeEmailDetail(email, false)
	if detail.RelayedAt == nil || !detail.RelayedAt.Equal(at) || detail.RelayedTo[0] != "actual@example.test" {
		t.Fatal("receipt missing")
	}
	*detail.RelayedAt = at.Add(time.Hour)
	detail.RelayedTo[0] = "changed@example.test"
	if !email.RelayedAt.Equal(at) || email.RelayedTo[0] != "actual@example.test" {
		t.Fatal("receipt projection aliases source")
	}
	raw, err := json.Marshal(makeEmailDetail(&types.Email{}, false))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "relayed_") {
		t.Fatalf("absent receipt emitted: %s", raw)
	}
}
