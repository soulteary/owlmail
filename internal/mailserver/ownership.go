package mailserver

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/soulteary/owlmail/internal/mailbox"
	"github.com/soulteary/owlmail/internal/types"
)

const ownershipMarker = ".owlmail-multiuser"
const ownershipMarkerContent = "owlmail-multiuser-v1\n"

// prepareOwnershipDirectory never upgrades an existing unmarked directory.
// This guards new standalone binaries and stdio observers; old binaries must
// still be kept away from isolated data by the documented rollback procedure.
func prepareOwnershipDirectory(directory string, required bool) error {
	path := filepath.Join(directory, ownershipMarker)
	info, err := os.Lstat(path)
	if err == nil {
		if !info.Mode().IsRegular() {
			return errors.New("invalid multiuser directory marker")
		}
		value, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if string(value) != ownershipMarkerContent {
			return errors.New("unsupported multiuser directory format")
		}
		if !required {
			return errors.New("multiuser mail directory cannot be opened in standalone mode")
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if !required {
		return nil
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return errors.New("isolated storage requires an empty directory or an explicit migration")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if _, err = f.WriteString(ownershipMarkerContent); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return syncDirectory(directory)
}

func validateOwnershipValue(value *types.Ownership) error {
	if value == nil {
		return errors.New("mailbox ownership required")
	}
	for _, id := range []string{value.MailboxID, value.TransactionID} {
		parsed, err := uuid.Parse(id)
		if err != nil || parsed.String() != id {
			return errors.New("invalid ownership identifier")
		}
	}
	if value.IngestAccountID == "" || len(value.IngestAccountID) > 256 || strings.ContainsAny(value.IngestAccountID, "\r\n\x00") {
		return errors.New("invalid ingest account")
	}
	if len(value.MatchedRecipients) == 0 || len(value.MatchedRecipients) != len(value.RouteRevisions) {
		return errors.New("invalid recipient ownership decision")
	}
	seen := map[string]bool{}
	for i, address := range value.MatchedRecipients {
		canonical, err := mailbox.NormalizeAddress(address)
		if err != nil || canonical != address || seen[address] || value.RouteRevisions[i] < 1 {
			return errors.New("invalid recipient route evidence")
		}
		seen[address] = true
	}
	return nil
}
func (ms *MailServer) validateOwnership(value *types.Ownership) error {
	if !ms.requireOwnership {
		if value != nil {
			return fmt.Errorf("ownership cannot be written in standalone mode")
		}
		return nil
	}
	return validateOwnershipValue(value)
}

// RequiresOwnership identifies isolated storage without exposing another user's ID.
func (ms *MailServer) RequiresOwnership() bool { return ms.requireOwnership }

// MailboxControl returns the shared authority for routed ingress and HTTP identity binding.
// Its lifetime belongs to the application, never to individual requests.
func (ms *MailServer) MailboxControl() *mailbox.Store {
	if ms.routedIngress == nil {
		return nil
	}
	return ms.routedIngress.options.Control
}
