// Package webassets exposes OwlMail's browser UI as build-time embedded files.
package webassets

import "embed"

// files contains every asset required by OwlMail's embedded browser pages.
//
//go:embed *.css *.html *.js *.svg
var files embed.FS

// ReadFile returns an embedded web asset by its base name.
func ReadFile(name string) ([]byte, error) {
	return files.ReadFile(name)
}
