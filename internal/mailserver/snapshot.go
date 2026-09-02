package mailserver

import (
	"github.com/emersion/go-message/mail"
	"github.com/soulteary/owlmail/internal/types"
)

// cloneEmail returns a detached snapshot of every mutable field in Email.
// Store callers and asynchronous event listeners must never share the internal
// object graph.
func cloneEmail(email *types.Email) *types.Email {
	if email == nil {
		return nil
	}

	cloned := *email
	cloned.From = cloneAddresses(email.From)
	cloned.To = cloneAddresses(email.To)
	cloned.CC = cloneAddresses(email.CC)
	cloned.BCC = cloneAddresses(email.BCC)
	cloned.CalculatedBCC = cloneAddresses(email.CalculatedBCC)
	cloned.Attachments = cloneAttachments(email.Attachments)
	cloned.Envelope = cloneEnvelope(email.Envelope)
	cloned.Headers = cloneHeaders(email.Headers)
	cloned.AllHeaders = cloneHeaders(email.AllHeaders)
	return &cloned
}

func cloneAddresses(addresses []*mail.Address) []*mail.Address {
	if addresses == nil {
		return nil
	}
	cloned := make([]*mail.Address, len(addresses))
	for i, address := range addresses {
		if address != nil {
			copyOfAddress := *address
			cloned[i] = &copyOfAddress
		}
	}
	return cloned
}

func cloneAttachments(attachments []*types.Attachment) []*types.Attachment {
	if attachments == nil {
		return nil
	}
	cloned := make([]*types.Attachment, len(attachments))
	for i, attachment := range attachments {
		if attachment != nil {
			copyOfAttachment := *attachment
			cloned[i] = &copyOfAttachment
		}
	}
	return cloned
}

func cloneEnvelope(envelope *types.Envelope) *types.Envelope {
	if envelope == nil {
		return nil
	}
	cloned := *envelope
	cloned.To = append([]string(nil), envelope.To...)
	cloned.CC = append([]string(nil), envelope.CC...)
	cloned.BCC = append([]string(nil), envelope.BCC...)
	cloned.CalculatedBCC = append([]string(nil), envelope.CalculatedBCC...)
	return &cloned
}

func cloneHeaders(headers map[string]interface{}) map[string]interface{} {
	if headers == nil {
		return nil
	}
	cloned := make(map[string]interface{}, len(headers))
	for key, value := range headers {
		cloned[key] = cloneHeaderValue(value)
	}
	return cloned
}

func cloneHeaderValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []interface{}:
		cloned := make([]interface{}, len(typed))
		for i, item := range typed {
			cloned[i] = cloneHeaderValue(item)
		}
		return cloned
	case map[string]string:
		cloned := make(map[string]string, len(typed))
		for key, item := range typed {
			cloned[key] = item
		}
		return cloned
	case map[string]interface{}:
		return cloneHeaders(typed)
	default:
		return typed
	}
}
