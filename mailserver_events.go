package main

// On registers an event listener
func (ms *MailServer) On(event string, handler func(*Email)) {
	ms.listenersMutex.Lock()
	defer ms.listenersMutex.Unlock()
	ms.listeners[event] = append(ms.listeners[event], handler)
}

// emit sends an event to all listeners
func (ms *MailServer) emit(event string, email *Email) {
	ms.listenersMutex.RLock()
	defer ms.listenersMutex.RUnlock()
	handlers := ms.listeners[event]
	for _, handler := range handlers {
		go handler(email)
	}
}
