package mailserver

import (
	"fmt"

	"github.com/soulteary/owlmail/internal/types"
)

// On registers an event listener
func (ms *MailServer) On(event string, handler func(*types.Email)) {
	_ = ms.OnWithConcurrency(event, 0, handler)
}

// OnSynchronous registers a lightweight handoff listener that completes
// before emit returns. It is intended for durable queue writes, not network
// delivery or other long-running work.
func (ms *MailServer) OnSynchronous(event string, handler func(*types.Email)) error {
	if handler == nil {
		return fmt.Errorf("event handler cannot be nil")
	}
	ms.listenersMutex.Lock()
	defer ms.listenersMutex.Unlock()
	ms.listeners[event] = append(ms.listeners[event], eventListener{handler: handler, synchronous: true})
	return nil
}

// OnWithConcurrency registers an event listener with an optional concurrency
// limit. A limit of zero preserves the original unlimited behavior. Limited
// listeners acquire a slot before their goroutine is created, providing real
// backpressure instead of accumulating goroutines that only wait on a lock.
func (ms *MailServer) OnWithConcurrency(event string, maxConcurrency int, handler func(*types.Email)) error {
	if maxConcurrency < 0 {
		return fmt.Errorf("max concurrency must be zero or greater")
	}
	if handler == nil {
		return fmt.Errorf("event handler cannot be nil")
	}

	listener := eventListener{handler: handler}
	if maxConcurrency > 0 {
		listener.slots = make(chan struct{}, maxConcurrency)
	}
	ms.listenersMutex.Lock()
	defer ms.listenersMutex.Unlock()
	ms.listeners[event] = append(ms.listeners[event], listener)
	return nil
}

// emit sends an event to all listeners
func (ms *MailServer) emit(event string, email *types.Email) {
	ms.listenersMutex.RLock()
	listeners := append([]eventListener(nil), ms.listeners[event]...)
	ms.listenersMutex.RUnlock()
	for _, listener := range listeners {
		if listener.synchronous {
			listener.handler(cloneEmail(email))
		}
	}

	// Start unlimited listeners first so a saturated bounded listener cannot
	// delay UI broadcasts or lightweight logging handlers registered after it.
	for _, listener := range listeners {
		if !listener.synchronous && listener.slots == nil {
			emailSnapshot := cloneEmail(email)
			go listener.handler(emailSnapshot)
		}
	}
	for _, listener := range listeners {
		if listener.synchronous || listener.slots == nil {
			continue
		}
		listener.slots <- struct{}{}
		emailSnapshot := cloneEmail(email)
		go func(listener eventListener) {
			defer func() { <-listener.slots }()
			listener.handler(emailSnapshot)
		}(listener)
	}
}
