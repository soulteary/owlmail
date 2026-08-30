package webassets

import (
	"bytes"
	"testing"
)

func TestRequiredAssetsAreEmbedded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		contains []byte
	}{
		{name: "index.html", contains: []byte("OwlMail")},
		{name: "app.js", contains: []byte("connectWebSocket")},
		{name: "style.css", contains: []byte(".header")},
		{name: "help.html", contains: []byte("OwlMail Help")},
		{name: "help.css", contains: []byte(".help-shell")},
		{name: "help.js", contains: []byte("applyLanguage")},
		{name: "webhooks.html", contains: []byte("Webhook Configurator")},
		{name: "webhooks.css", contains: []byte(".webhook-workspace")},
		{name: "webhooks.js", contains: []byte("parseConfigText")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asset, err := ReadFile(tt.name)
			if err != nil {
				t.Fatalf("ReadFile(%q) error: %v", tt.name, err)
			}
			if !bytes.Contains(asset, tt.contains) {
				t.Fatalf("ReadFile(%q) does not contain %q", tt.name, tt.contains)
			}
		})
	}
}

func TestMissingAsset(t *testing.T) {
	t.Parallel()

	if _, err := ReadFile("missing.txt"); err == nil {
		t.Fatal("ReadFile(missing.txt) unexpectedly succeeded")
	}
}
