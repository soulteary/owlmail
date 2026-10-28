// Package mailhtml resolves attachment references without changing stored MIME data.
package mailhtml

import (
	"io"
	"net/url"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
)

// AttachmentReference identifies one stored attachment and its rendering URL.
type AttachmentReference struct {
	ContentID string
	URL       string
}

// NormalizeContentID removes MIME's optional outer brackets and whitespace.
// It deliberately preserves case and URI escapes in the stored identifier.
func NormalizeContentID(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "<")
	value = strings.TrimSuffix(value, ">")
	return strings.TrimSpace(value)
}

// RewriteAttachmentReferences rewrites only src/href CID attributes. Unknown
// and ambiguous identifiers remain untouched. Exact matches precede a unique
// case-insensitive fallback for compatibility with older previews.
func RewriteAttachmentReferences(input string, attachments []AttachmentReference) string {
	exact := make(map[string]map[string]struct{})
	folded := make(map[string]map[string]struct{})
	add := func(index map[string]map[string]struct{}, key, target string) {
		if index[key] == nil {
			index[key] = make(map[string]struct{})
		}
		index[key][target] = struct{}{}
	}
	for _, attachment := range attachments {
		id := NormalizeContentID(attachment.ContentID)
		if id == "" || attachment.URL == "" {
			continue
		}
		add(exact, id, attachment.URL)
		add(folded, strings.ToLower(id), attachment.URL)
	}
	lookup := func(value string) (string, bool) {
		value = strings.TrimSpace(value)
		if len(value) < 4 || !strings.EqualFold(value[:4], "cid:") {
			return "", false
		}
		decoded, err := url.PathUnescape(value[4:])
		if err != nil || !utf8.ValidString(decoded) {
			return "", false
		}
		id := NormalizeContentID(decoded)
		matches, exists := exact[id]
		if !exists {
			matches = folded[strings.ToLower(id)]
		}
		if len(matches) != 1 {
			return "", false
		}
		for target := range matches {
			return target, true
		}
		return "", false
	}
	var output strings.Builder
	tokenizer := html.NewTokenizer(strings.NewReader(input))
	for {
		kind := tokenizer.Next()
		if kind == html.ErrorToken {
			if tokenizer.Err() != io.EOF {
				return input
			}
			// Raw includes an unfinished trailing tag when EOF interrupts tokenization.
			output.Write(tokenizer.Raw())
			return output.String()
		}
		raw := string(tokenizer.Raw())
		if kind != html.StartTagToken && kind != html.SelfClosingTagToken {
			output.WriteString(raw)
			continue
		}
		token := tokenizer.Token()
		changed := false
		for i := range token.Attr {
			attr := &token.Attr[i]
			if attr.Namespace != "" || (attr.Key != "src" && attr.Key != "href") {
				continue
			}
			if target, ok := lookup(attr.Val); ok {
				attr.Val = target
				changed = true
			}
		}
		if changed {
			output.WriteString(token.String())
		} else {
			output.WriteString(raw)
		}
	}
}
