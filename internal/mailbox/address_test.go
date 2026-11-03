package mailbox

import (
	"context"
	"strconv"
	"strings"
	"testing"
)

func TestNormalizeAddressIDNALengthBoundary(t *testing.T) {
	// This valid domain expands by six bytes per label when encoded as IDNA.
	domain := strings.TrimSuffix(strings.Repeat("é"+strings.Repeat("a", 41)+".", 4), ".")
	for _, localLength := range []int{54, 55, 64} {
		input := strings.Repeat("a", localLength) + "@" + domain
		t.Run(strconv.Itoa(localLength), func(t *testing.T) {
			canonical, err := NormalizeAddress(input)
			if localLength > 54 {
				if err == nil {
					t.Fatalf("accepted overlong IDNA address: %d bytes", len(canonical))
				}
				return
			}
			if err != nil || len(canonical) != 254 {
				t.Fatalf("254-byte boundary: %q, %v", canonical, err)
			}
			again, err := NormalizeAddress(canonical)
			if err != nil || again != canonical {
				t.Fatalf("normalization is not idempotent: %q, %v", again, err)
			}
			_, asciiDomain, _ := strings.Cut(canonical, "@")
			account := Account{AllowedDomains: []string{asciiDomain}}
			if !account.Allows(canonical) {
				t.Fatal("accepted boundary address cannot pass ingress account checks")
			}
		})
	}
}

func TestBindLoginRejectsOverlongIDNARoute(t *testing.T) {
	s := testStore(t)
	input := strings.Repeat("a", 64) + "@" + strings.TrimSuffix(strings.Repeat("é"+strings.Repeat("a", 41)+".", 4), ".")
	_, err := s.BindLogin(context.Background(), Login{Namespace: "main", Subject: "alice", Email: input, Verified: true}, VerifiedEmail)
	if err == nil {
		t.Fatal("bound an address that cannot be routed")
	}
	var routes int
	if err := s.db.QueryRow("SELECT count(*) FROM recipient_route").Scan(&routes); err != nil {
		t.Fatal(err)
	}
	if routes != 0 {
		t.Fatalf("persisted %d routes after invalid login", routes)
	}
}
