package mailbox

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestSystemAccounts(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), 10)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "accounts.yaml")
	doc := fmt.Sprintf("version: 1\naccounts:\n  - id: gorge\n    username: gorge\n    password-hash: %s\n    namespace: main\n    enabled: true\n    allowed-domains: [example.com]\n", hash)
	if err = os.WriteFile(path, []byte(doc), 0600); err != nil {
		t.Fatal(err)
	}
	accounts, err := LoadAccounts(path, "main", []string{"example.com"})
	if err != nil {
		t.Fatal(err)
	}
	account, err := accounts.Authenticate("gorge", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if account.PasswordHash != "" || !account.Allows("alice@EXAMPLE.com") || account.Allows("alice@other.com") {
		t.Fatal("invalid account projection or domain policy")
	}
	for _, user := range []string{"gorge", "unknown"} {
		if _, err = accounts.Authenticate(user, "wrong"); err == nil {
			t.Fatal("accepted wrong credentials")
		}
	}
	if _, err = LoadAccounts(path, "main", []string{"other.com"}); err == nil {
		t.Fatal("account widened instance domain policy")
	}
	if err = os.WriteFile(path, []byte(doc+"unknown: value\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadAccounts(path, "main", []string{"example.com"}); err == nil {
		t.Fatal("accepted unknown option")
	}
}
