package maildev

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/types"
)

func TestRelayReceiptProjectionCopiesAndOmitsAbsent(t *testing.T) {
	at := time.Now().UTC()
	email := &types.Email{ID: "receipt", RelayedAt: &at, RelayedTo: []string{"actual@example.test"}}
	dto := FromEmail(email, "/mail")
	if dto.RelayedAt == nil || !dto.RelayedAt.Equal(at) || dto.RelayedTo[0] != "actual@example.test" {
		t.Fatal("receipt missing")
	}
	*dto.RelayedAt = at.Add(time.Hour)
	dto.RelayedTo[0] = "changed@example.test"
	if !email.RelayedAt.Equal(at) || email.RelayedTo[0] != "actual@example.test" {
		t.Fatal("receipt projection aliases source")
	}
	raw, err := json.Marshal(FromEmail(&types.Email{}, "/mail"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "relayed") {
		t.Fatalf("absent receipt emitted: %s", raw)
	}
}
