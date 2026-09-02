package mailserver

import "github.com/emersion/go-smtp"

var errSMTPDataConcurrencyLimit = &smtp.SMTPError{
	Code:         451,
	EnhancedCode: smtp.EnhancedCode{4, 3, 2},
	Message:      "temporary resource limit reached; try again later",
}

// dataLimiter is owned by one MailServer and therefore shared by its ordinary
// SMTP, STARTTLS, and SMTPS sessions. A nil limiter is the explicit unlimited
// mode. Acquisition is intentionally non-blocking so clients receive a
// retryable SMTP response instead of waiting indefinitely.
type dataLimiter struct {
	slots chan struct{}
}

func newDataLimiter(maxConcurrency int) *dataLimiter {
	if maxConcurrency == 0 {
		return nil
	}
	return &dataLimiter{slots: make(chan struct{}, maxConcurrency)}
}

func (limiter *dataLimiter) tryAcquire() bool {
	if limiter == nil {
		return true
	}
	select {
	case limiter.slots <- struct{}{}:
		return true
	default:
		return false
	}
}

func (limiter *dataLimiter) release() {
	if limiter != nil {
		<-limiter.slots
	}
}

func (ms *MailServer) tryAcquireDataSlot() bool {
	return ms.dataLimiter.tryAcquire()
}

func (ms *MailServer) releaseDataSlot() {
	// Keep the slot held while a deterministic test hook accounts for the
	// completed transaction. The deferred release still prevents a hook panic
	// from leaking capacity.
	defer ms.dataLimiter.release()
	if ms.beforeDataRelease != nil {
		ms.beforeDataRelease()
	}
}
