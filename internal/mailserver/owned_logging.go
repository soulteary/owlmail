package mailserver

import "fmt"

// Owned message diagnostics must not quote untrusted MIME or reader contents.
// Unwrap preserves cancellation and other errors.Is/As checks without printing
// the potentially sensitive underlying error in normal logs or API responses.
type ownedMessageParseError struct {
	id    string
	cause error
}

func (e *ownedMessageParseError) Error() string {
	return fmt.Sprintf("owned message parsing failed: id: %s, class: message_parse", e.id)
}

func (e *ownedMessageParseError) Unwrap() error { return e.cause }
