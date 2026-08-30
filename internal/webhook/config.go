// Package webhook delivers newly received emails to configurable HTTP endpoints.
package webhook

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"text/template"
	"time"
)

const (
	configVersion        = 1
	maxConfigBytes       = 1 << 20
	maxTargets           = 32
	maxRetries           = 5
	defaultTargetTimeout = 5 * time.Second
	maxTargetTimeout     = time.Minute
)

var environmentVariablePattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// Config is the top-level webhook forwarding configuration.
type Config struct {
	Version int      `json:"version,omitempty"`
	Targets []Target `json:"targets"`
}

// Target describes one HTTP destination and the emails it should receive.
type Target struct {
	Name         string            `json:"name"`
	URL          string            `json:"url"`
	Method       string            `json:"method,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	ContentType  string            `json:"contentType,omitempty"`
	BodyTemplate string            `json:"bodyTemplate,omitempty"`
	Secret       string            `json:"secret,omitempty"`
	Timeout      string            `json:"timeout,omitempty"`
	Retries      int               `json:"retries,omitempty"`
	Match        Match             `json:"match,omitempty"`
}

// Match filters a target. Non-empty fields are ANDed, while patterns inside a
// field are ORed. Patterns use shell-style '*' and '?' wildcards and are
// matched case-insensitively.
type Match struct {
	From    []string `json:"from,omitempty"`
	To      []string `json:"to,omitempty"`
	Subject []string `json:"subject,omitempty"`
	Text    []string `json:"text,omitempty"`
}

type compiledTarget struct {
	name        string
	url         string
	method      string
	headers     http.Header
	contentType string
	secret      string
	timeout     time.Duration
	retries     int
	match       Match
	template    *template.Template
}

// LoadConfig reads a strict JSON webhook configuration. Unknown fields and
// trailing JSON values are rejected so configuration mistakes fail at startup.
func LoadConfig(filePath string) (Config, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return Config{}, fmt.Errorf("open webhook config: %w", err)
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(io.LimitReader(file, maxConfigBytes+1))
	if err != nil {
		return Config{}, fmt.Errorf("read webhook config: %w", err)
	}
	if len(data) > maxConfigBytes {
		return Config{}, fmt.Errorf("webhook config exceeds %d bytes", maxConfigBytes)
	}

	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()

	var config Config
	if err := decoder.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("decode webhook config: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return Config{}, err
	}

	return config, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("decode webhook config: multiple JSON values are not allowed")
		}
		return fmt.Errorf("decode webhook config: trailing data: %w", err)
	}
	return nil
}

func compileConfig(config Config) ([]compiledTarget, error) {
	if config.Version != 0 && config.Version != configVersion {
		return nil, fmt.Errorf("unsupported webhook config version %d", config.Version)
	}
	if len(config.Targets) == 0 {
		return nil, fmt.Errorf("webhook config must contain at least one target")
	}
	if len(config.Targets) > maxTargets {
		return nil, fmt.Errorf("webhook config contains %d targets; maximum is %d", len(config.Targets), maxTargets)
	}

	compiled := make([]compiledTarget, 0, len(config.Targets))
	names := make(map[string]struct{}, len(config.Targets))
	for index, target := range config.Targets {
		item, err := compileTarget(target)
		if err != nil {
			return nil, fmt.Errorf("target %d: %w", index+1, err)
		}
		if _, exists := names[item.name]; exists {
			return nil, fmt.Errorf("target %d: duplicate name %q", index+1, item.name)
		}
		names[item.name] = struct{}{}
		compiled = append(compiled, item)
	}

	return compiled, nil
}

func compileTarget(target Target) (compiledTarget, error) {
	name := strings.TrimSpace(target.Name)
	if name == "" {
		return compiledTarget{}, fmt.Errorf("name is required")
	}
	if len(name) > 100 || strings.ContainsAny(name, "\r\n") {
		return compiledTarget{}, fmt.Errorf("name must be at most 100 UTF-8 bytes and contain no newlines")
	}

	resolvedURL, err := expandEnvironmentVariables(target.URL)
	if err != nil {
		return compiledTarget{}, fmt.Errorf("url: %w", err)
	}
	parsedURL, err := url.Parse(resolvedURL)
	if err != nil {
		return compiledTarget{}, fmt.Errorf("url is invalid")
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return compiledTarget{}, fmt.Errorf("url scheme must be http or https")
	}
	if parsedURL.Hostname() == "" {
		return compiledTarget{}, fmt.Errorf("url host is required")
	}
	if parsedURL.User != nil {
		return compiledTarget{}, fmt.Errorf("url user information is not allowed; use headers for authentication")
	}
	if parsedURL.Fragment != "" {
		return compiledTarget{}, fmt.Errorf("url fragments are not allowed")
	}

	method := strings.ToUpper(strings.TrimSpace(target.Method))
	if method == "" {
		method = http.MethodPost
	}
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
	default:
		return compiledTarget{}, fmt.Errorf("method %q is not supported", method)
	}

	timeout := defaultTargetTimeout
	if target.Timeout != "" {
		timeout, err = time.ParseDuration(target.Timeout)
		if err != nil {
			return compiledTarget{}, fmt.Errorf("invalid timeout: %w", err)
		}
	}
	if timeout <= 0 || timeout > maxTargetTimeout {
		return compiledTarget{}, fmt.Errorf("timeout must be greater than zero and no more than %s", maxTargetTimeout)
	}
	if target.Retries < 0 || target.Retries > maxRetries {
		return compiledTarget{}, fmt.Errorf("retries must be between 0 and %d", maxRetries)
	}

	contentType := strings.TrimSpace(target.ContentType)
	if contentType == "" {
		contentType = "application/json"
	}
	if strings.ContainsAny(contentType, "\r\n") {
		return compiledTarget{}, fmt.Errorf("contentType contains a newline")
	}

	headers := make(http.Header, len(target.Headers))
	for headerName, headerValue := range target.Headers {
		canonicalName := http.CanonicalHeaderKey(strings.TrimSpace(headerName))
		if canonicalName == "" || !validHeaderName(headerName) {
			return compiledTarget{}, fmt.Errorf("invalid header name %q", headerName)
		}
		if canonicalName == "Host" || canonicalName == "Content-Length" {
			return compiledTarget{}, fmt.Errorf("header %q is managed by the HTTP client", canonicalName)
		}
		resolvedValue, expandErr := expandEnvironmentVariables(headerValue)
		if expandErr != nil {
			return compiledTarget{}, fmt.Errorf("header %q: %w", canonicalName, expandErr)
		}
		if strings.ContainsAny(resolvedValue, "\r\n") {
			return compiledTarget{}, fmt.Errorf("header %q contains a newline", canonicalName)
		}
		headers.Set(canonicalName, resolvedValue)
	}

	secret, err := expandEnvironmentVariables(target.Secret)
	if err != nil {
		return compiledTarget{}, fmt.Errorf("secret: %w", err)
	}

	if err := validateMatch(target.Match); err != nil {
		return compiledTarget{}, err
	}

	var bodyTemplate *template.Template
	if target.BodyTemplate != "" {
		bodyTemplate, err = template.New(name).
			Option("missingkey=error").
			Funcs(templateFunctions()).
			Parse(target.BodyTemplate)
		if err != nil {
			return compiledTarget{}, fmt.Errorf("invalid bodyTemplate: %w", err)
		}
	}

	return compiledTarget{
		name:        name,
		url:         parsedURL.String(),
		method:      method,
		headers:     headers,
		contentType: contentType,
		secret:      secret,
		timeout:     timeout,
		retries:     target.Retries,
		match:       target.Match,
		template:    bodyTemplate,
	}, nil
}

func validHeaderName(name string) bool {
	if name == "" {
		return false
	}
	for index := 0; index < len(name); index++ {
		character := name[index]
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') {
			continue
		}
		switch character {
		case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
			continue
		default:
			return false
		}
	}
	return true
}

func validateMatch(match Match) error {
	fields := []struct {
		name     string
		patterns []string
	}{
		{name: "from", patterns: match.From},
		{name: "to", patterns: match.To},
		{name: "subject", patterns: match.Subject},
		{name: "text", patterns: match.Text},
	}
	for _, field := range fields {
		for _, pattern := range field.patterns {
			if strings.TrimSpace(pattern) == "" {
				return fmt.Errorf("match.%s contains an empty pattern", field.name)
			}
			if _, err := path.Match(strings.ToLower(pattern), ""); err != nil {
				return fmt.Errorf("match.%s contains invalid pattern %q: %w", field.name, pattern, err)
			}
		}
	}
	return nil
}

func expandEnvironmentVariables(value string) (string, error) {
	var invalidName string
	var invalidReason string
	expanded := environmentVariablePattern.ReplaceAllStringFunc(value, func(token string) string {
		name := environmentVariablePattern.FindStringSubmatch(token)[1]
		resolved, exists := os.LookupEnv(name)
		if invalidName == "" {
			switch {
			case !exists:
				invalidName = name
				invalidReason = "is not set"
			case resolved == "":
				invalidName = name
				invalidReason = "is empty"
			}
		}
		return resolved
	})
	if invalidName != "" {
		return "", fmt.Errorf("environment variable %s %s", invalidName, invalidReason)
	}
	return expanded, nil
}
