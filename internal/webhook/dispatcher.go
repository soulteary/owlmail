package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/soulteary/owlmail/internal/types"
)

const (
	maxPayloadBytes = 2 << 20
	maxDrainBytes   = 64 << 10
)

// Dispatcher sends matching email events to configured targets.
type Dispatcher struct {
	targets []compiledTarget
	client  *http.Client
}

// Result describes one target delivery attempt.
type Result struct {
	Target     string
	StatusCode int
	Attempts   int
	Err        error
}

// Load creates a dispatcher from a JSON configuration file.
func Load(filePath string, client *http.Client) (*Dispatcher, error) {
	config, err := LoadConfig(filePath)
	if err != nil {
		return nil, err
	}
	return NewDispatcher(config, client)
}

// NewDispatcher validates configuration and creates a dispatcher. Redirects
// are intentionally not followed so a configured target cannot redirect a
// payload or credentials to another host.
func NewDispatcher(config Config, client *http.Client) (*Dispatcher, error) {
	targets, err := compileConfig(config)
	if err != nil {
		return nil, err
	}
	if client == nil {
		transport := http.DefaultTransport
		if defaultTransport, ok := http.DefaultTransport.(*http.Transport); ok {
			clonedTransport := defaultTransport.Clone()
			clonedTransport.MaxIdleConnsPerHost = 4
			transport = clonedTransport
		}
		client = &http.Client{Transport: transport}
	} else {
		copy := *client
		client = &copy
	}
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return &Dispatcher{targets: targets, client: client}, nil
}

// TargetCount returns the number of configured destinations.
func (dispatcher *Dispatcher) TargetCount() int {
	if dispatcher == nil {
		return 0
	}
	return len(dispatcher.targets)
}

// Dispatch synchronously sends an email to every matching target. MailServer
// invokes event listeners asynchronously, so this never blocks SMTP storage.
func (dispatcher *Dispatcher) Dispatch(ctx context.Context, email *types.Email) []Result {
	if dispatcher == nil {
		return nil
	}
	if email == nil {
		return []Result{{Err: fmt.Errorf("email is nil")}}
	}
	if ctx == nil {
		ctx = context.Background()
	}

	payload := newEmailPayload(email)
	results := make([]Result, 0, len(dispatcher.targets))
	for _, target := range dispatcher.targets {
		if !target.matches(payload) {
			continue
		}
		results = append(results, dispatcher.deliver(ctx, target, payload))
	}
	return results
}

func (dispatcher *Dispatcher) deliver(ctx context.Context, target compiledTarget, payload EmailPayload) Result {
	result := Result{Target: target.name}
	body, err := renderBody(target, payload)
	if err != nil {
		result.Err = err
		return result
	}
	if len(body) > maxPayloadBytes {
		result.Err = fmt.Errorf("rendered payload exceeds %d bytes", maxPayloadBytes)
		return result
	}

	for attempt := 0; attempt <= target.retries; attempt++ {
		result.Attempts = attempt + 1
		statusCode, retry, requestErr := dispatcher.sendAttempt(ctx, target, payload.ID, body)
		result.StatusCode = statusCode
		if requestErr == nil {
			result.Err = nil
			return result
		}
		result.Err = requestErr
		if !retry || attempt == target.retries || ctx.Err() != nil {
			return result
		}
		if !waitForRetry(ctx, attempt) {
			result.Err = ctx.Err()
			return result
		}
	}
	return result
}

func renderBody(target compiledTarget, payload EmailPayload) ([]byte, error) {
	if target.template == nil {
		body, err := json.Marshal(eventPayload{
			Event:   "email.received",
			Message: defaultMessage(payload),
			Email:   payload,
		})
		if err != nil {
			return nil, fmt.Errorf("encode default payload: %w", err)
		}
		return body, nil
	}

	var body bytes.Buffer
	if err := target.template.Execute(&body, payload); err != nil {
		return nil, fmt.Errorf("render body template: %w", err)
	}
	return body.Bytes(), nil
}

func (dispatcher *Dispatcher) sendAttempt(ctx context.Context, target compiledTarget, emailID string, body []byte) (int, bool, error) {
	attemptContext, cancel := context.WithTimeout(ctx, target.timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(attemptContext, target.method, target.url, bytes.NewReader(body))
	if err != nil {
		return 0, false, fmt.Errorf("create request: %w", err)
	}
	request.Header = target.headers.Clone()
	if request.Header.Get("Content-Type") == "" {
		request.Header.Set("Content-Type", target.contentType)
	}
	if request.Header.Get("User-Agent") == "" {
		request.Header.Set("User-Agent", "OwlMail-Webhook/1")
	}
	request.Header.Set("X-OwlMail-Event", "email.received")
	request.Header.Set("X-OwlMail-Email-ID", emailID)
	if target.secret != "" {
		request.Header.Set("X-OwlMail-Signature", signPayload(target.secret, body))
	}

	response, err := dispatcher.client.Do(request)
	if err != nil {
		return 0, !errors.Is(err, context.Canceled), sanitizeRequestError(err)
	}
	defer func() { _ = response.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxDrainBytes))

	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		return response.StatusCode, false, nil
	}
	retry := response.StatusCode == http.StatusRequestTimeout ||
		response.StatusCode == http.StatusTooEarly ||
		response.StatusCode == http.StatusTooManyRequests ||
		response.StatusCode >= http.StatusInternalServerError
	return response.StatusCode, retry, fmt.Errorf("target returned HTTP %d", response.StatusCode)
}

func sanitizeRequestError(err error) error {
	for {
		var urlError *url.Error
		if !errors.As(err, &urlError) || urlError.Err == nil || urlError.Err == err {
			break
		}
		err = urlError.Err
	}
	return fmt.Errorf("request failed: %v", err)
}

func signPayload(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func waitForRetry(ctx context.Context, attempt int) bool {
	delay := 100 * time.Millisecond * time.Duration(1<<attempt)
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
