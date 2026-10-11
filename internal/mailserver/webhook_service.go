package mailserver

import "github.com/soulteary/owlmail/internal/webhook"

// SetWebhookService registers diagnostics for the server-owned delivery service.
func (ms *MailServer) SetWebhookService(service *webhook.Service) {
	ms.storeMutex.Lock()
	defer ms.storeMutex.Unlock()
	ms.webhookService = service
}

// WebhookService returns the server-owned delivery service.
func (ms *MailServer) WebhookService() *webhook.Service {
	ms.storeMutex.RLock()
	defer ms.storeMutex.RUnlock()
	return ms.webhookService
}
