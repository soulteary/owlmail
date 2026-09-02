// Package openapi embeds OwlMail's canonical OpenAPI 3.1 documents.
package openapi

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"strconv"
)

//go:embed openapi.json openapi.yaml
var documents embed.FS

// JSON returns the JSON contract with its first server URL set to the
// effective, base-path-aware API prefix.
func JSON(serverURL string) ([]byte, error) {
	raw, err := documents.ReadFile("openapi.json")
	if err != nil {
		return nil, fmt.Errorf("read embedded OpenAPI JSON: %w", err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, fmt.Errorf("parse embedded OpenAPI JSON: %w", err)
	}
	servers, ok := document["servers"].([]any)
	if !ok || len(servers) == 0 {
		return nil, fmt.Errorf("embedded OpenAPI JSON has no servers")
	}
	server, ok := servers[0].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("embedded OpenAPI JSON has an invalid server")
	}
	server["url"] = serverURL
	return json.MarshalIndent(document, "", "  ")
}

// YAML returns the YAML contract with its first server URL set to the
// effective, base-path-aware API prefix. SetBasePathname only accepts a
// normalized URL path; quoting it also keeps the emitted YAML unambiguous.
func YAML(serverURL string) ([]byte, error) {
	raw, err := documents.ReadFile("openapi.yaml")
	if err != nil {
		return nil, fmt.Errorf("read embedded OpenAPI YAML: %w", err)
	}
	const original = "- url: /api/v1"
	replacement := "- url: " + strconv.Quote(serverURL)
	updated := bytes.Replace(raw, []byte(original), []byte(replacement), 1)
	if bytes.Equal(updated, raw) {
		return nil, fmt.Errorf("embedded OpenAPI YAML has no root server URL")
	}
	return updated, nil
}
