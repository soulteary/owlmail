package mailserver

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ErrInvalidCursor means the cursor is malformed, from another process, or ahead of this mailbox.
var ErrInvalidCursor = errors.New("invalid or expired delivery cursor")

// ErrWaitCapacity means all bounded delivery wait slots are occupied.
var ErrWaitCapacity = errors.New("delivery wait capacity reached")

// ErrMailboxClosed means the mailbox stopped while a caller was waiting.
var ErrMailboxClosed = errors.New("mailbox closed")

// DeliveryFilter groups test messages without granting a separate security boundary.
// Recipient matches the complete SMTP envelope address, case-sensitively.
type DeliveryFilter struct {
	Recipient      string `json:"recipient,omitempty"`
	RunID          string `json:"run_id,omitempty"`
	Subject        string `json:"subject,omitempty"`
	Text           string `json:"text,omitempty"`
	subjectPattern *regexp.Regexp
	textPattern    *regexp.Regexp
}

// ValidateRunID accepts a bounded, portable test-run identifier. Empty means ungrouped.
func ValidateRunID(id string) error {
	if len(id) > 128 {
		return fmt.Errorf("run_id cannot exceed 128 bytes")
	}
	for _, c := range id {
		valid := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("._:-", c)
		if !valid {
			return fmt.Errorf("run_id must contain only ASCII letters, digits, dot, underscore, colon, or hyphen")
		}
	}
	return nil
}

// Validate checks filter bounds before waiting or querying.
func (f DeliveryFilter) Validate() error {
	if err := ValidateRunID(f.RunID); err != nil {
		return err
	}
	for _, s := range []string{f.Recipient, f.Subject, f.Text} {
		if len(s) > 1024 {
			return fmt.Errorf("delivery filters cannot exceed 1024 bytes")
		}
	}
	return nil
}

// Matches applies exact run/envelope selectors and case-insensitive content substrings.
func (f DeliveryFilter) Matches(e *Email) bool {
	if e == nil || f.RunID != "" && e.RunID != f.RunID {
		return false
	}
	if f.Recipient != "" {
		found := false
		if e.Envelope != nil {
			for _, r := range e.Envelope.To {
				if r == f.Recipient {
					found = true
					break
				}
			}
		}
		if !found {
			return false
		}
	}
	return foldedContains(e.Subject, f.Subject, f.subjectPattern) && (f.Text == "" || foldedContains(e.Text, f.Text, f.textPattern) || foldedContains(e.HTML, f.Text, f.textPattern))
}

func foldedContains(value, literal string, pattern *regexp.Regexp) bool {
	if literal == "" {
		return true
	}
	if pattern == nil {
		var err error
		pattern, err = regexp.Compile("(?i)" + regexp.QuoteMeta(literal))
		if err != nil {
			return false
		}
	}
	return pattern.MatchString(value)
}

func (ms *MailServer) initDeliveriesLocked() {
	if ms.deliveryEpoch == "" {
		ms.deliveryEpoch = rand.Text()
		ms.deliverySequences = make(map[string]uint64)
		ms.deliveryChanged = make(chan struct{})
	}
}
func (ms *MailServer) recordDeliveryLocked(id string) {
	ms.initDeliveriesLocked()
	if _, exists := ms.deliverySequences[id]; exists {
		return
	}
	ms.deliverySequence++
	ms.deliverySequences[id] = ms.deliverySequence
	if !ms.deliveryClosed {
		close(ms.deliveryChanged)
		ms.deliveryChanged = make(chan struct{})
	}
}
func (ms *MailServer) cursorLocked() string {
	return ms.deliveryEpoch + ":" + strconv.FormatUint(ms.deliverySequence, 10)
}

// DeliveryCursor identifies the last committed message in this process. Restarts invalidate it.
func (ms *MailServer) DeliveryCursor() string {
	ms.storeMutex.Lock()
	defer ms.storeMutex.Unlock()
	ms.initDeliveriesLocked()
	return ms.cursorLocked()
}
func (ms *MailServer) parseCursorLocked(cursor string) (uint64, error) {
	epoch, seq, ok := strings.Cut(cursor, ":")
	n, err := strconv.ParseUint(seq, 10, 64)
	if !ok || err != nil || epoch != ms.deliveryEpoch || n > ms.deliverySequence || seq != strconv.FormatUint(n, 10) {
		return 0, ErrInvalidCursor
	}
	return n, nil
}

// DeliveryResult contains detached committed messages, including a normal timeout outcome.
type DeliveryResult struct {
	Emails             []*Email `json:"emails"`
	Cursor             string   `json:"cursor"`
	TimedOut           bool     `json:"timed_out"`
	WaitedMilliseconds int64    `json:"waited_milliseconds"`
}

// WaitDeliveries replays retained messages after a cursor and atomically subscribes
// to subsequent commits. It never polls, changes read state, or installs permanent listeners.
// Count is an at-least threshold (1..100), not an exactly-once delivery guarantee.
func (ms *MailServer) WaitDeliveries(ctx context.Context, after string, filter DeliveryFilter, count int, timeout time.Duration) (DeliveryResult, error) {
	result := DeliveryResult{Emails: make([]*Email, 0)}
	if err := filter.Validate(); err != nil {
		return result, err
	}
	var compileErr error
	filter.subjectPattern, compileErr = regexp.Compile("(?i)" + regexp.QuoteMeta(filter.Subject))
	if compileErr != nil {
		return result, compileErr
	}
	filter.textPattern, compileErr = regexp.Compile("(?i)" + regexp.QuoteMeta(filter.Text))
	if compileErr != nil {
		return result, compileErr
	}
	if count < 1 || count > 100 || timeout <= 0 || timeout > 2*time.Minute {
		return result, fmt.Errorf("count must be 1..100 and timeout must be positive and at most 120 seconds")
	}
	ms.storeMutex.Lock()
	ms.initDeliveriesLocked()
	sequence, err := ms.parseCursorLocked(after)
	if err != nil {
		ms.storeMutex.Unlock()
		return result, err
	}
	if ms.deliveryClosed {
		ms.storeMutex.Unlock()
		return result, ErrMailboxClosed
	}
	if ms.deliveryWaiters >= 64 {
		ms.storeMutex.Unlock()
		return result, ErrWaitCapacity
	}
	ms.deliveryWaiters++
	ms.storeMutex.Unlock()
	defer func() { ms.storeMutex.Lock(); ms.deliveryWaiters--; ms.storeMutex.Unlock() }()
	started := time.Now()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		ms.storeMutex.RLock()
		if ms.deliveryClosed {
			ms.storeMutex.RUnlock()
			return result, ErrMailboxClosed
		}
		result.Emails = result.Emails[:0]
		scannedSequence := ms.deliverySequence
		matching := make([]string, 0)
		for id, delivery := range ms.deliverySequences {
			if delivery > sequence && filter.Matches(ms.storeByID[id]) {
				matching = append(matching, id)
			}
		}
		sort.Slice(matching, func(i, j int) bool { return ms.deliverySequences[matching[i]] < ms.deliverySequences[matching[j]] })
		for _, id := range matching {
			result.Emails = append(result.Emails, cloneEmail(ms.storeByID[id]))
			if len(result.Emails) == count {
				scannedSequence = ms.deliverySequences[id]
				break
			}
		}
		result.Cursor = ms.deliveryEpoch + ":" + strconv.FormatUint(scannedSequence, 10)
		changed := ms.deliveryChanged
		ms.storeMutex.RUnlock()
		result.WaitedMilliseconds = time.Since(started).Milliseconds()
		if len(result.Emails) >= count {
			return result, nil
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return result, ctx.Err()
		case <-timer.C:
			result.TimedOut = true
			result.WaitedMilliseconds = time.Since(started).Milliseconds()
			return result, nil
		}
	}
}
