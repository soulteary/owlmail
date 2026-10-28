package mailhtml

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestCIDReferenceFixtures(t *testing.T) {
	data, err := os.ReadFile("../../testdata/cid/references.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name        string `json:"name"`
		HTML        string `json:"html"`
		Attachments []struct {
			ContentID string `json:"contentId"`
			Filename  string `json:"generatedFileName"`
		} `json:"attachments"`
		Rewritten int      `json:"rewritten"`
		Targets   []string `json:"targets"`
		Preserved string   `json:"preserved"`
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			refs := make([]AttachmentReference, 0, len(fixture.Attachments))
			for _, attachment := range fixture.Attachments {
				refs = append(refs, AttachmentReference{ContentID: attachment.ContentID, URL: "/attachments/" + attachment.Filename})
			}
			result := RewriteAttachmentReferences(fixture.HTML, refs)
			if count := strings.Count(result, "/attachments/"); count != fixture.Rewritten {
				t.Fatalf("rewritten count %d want %d: %s", count, fixture.Rewritten, result)
			}
			if fixture.Rewritten == 0 && result != fixture.HTML {
				t.Fatalf("unmatched HTML changed: %s", result)
			}
			tokenizer := html.NewTokenizer(strings.NewReader(result))
			var targets []string
			for tokenizer.Next() != html.ErrorToken {
				token := tokenizer.Token()
				for _, attr := range token.Attr {
					if (attr.Key == "src" || attr.Key == "href") && strings.HasPrefix(attr.Val, "/attachments/") {
						targets = append(targets, strings.TrimPrefix(attr.Val, "/attachments/"))
					}
				}
			}
			if len(targets) != len(fixture.Targets) {
				t.Fatalf("targets %v want %v", targets, fixture.Targets)
			}
			for i := range targets {
				if targets[i] != fixture.Targets[i] {
					t.Fatalf("targets %v want %v", targets, fixture.Targets)
				}
			}
			if fixture.Preserved != "" && !strings.Contains(result, fixture.Preserved) {
				t.Fatalf("unmatched attribute changed: %s", result)
			}
		})
	}
}

func FuzzRewriteAttachmentReferences(f *testing.F) {
	for _, input := range []string{`<img src="cid:id">`, `<script>"cid:id"</script>`, `<img src='cid:%ZZ'>`, `<img src="cid:id"`} {
		f.Add(input)
	}
	f.Fuzz(func(t *testing.T, input string) {
		refs := []AttachmentReference{{ContentID: "<id>", URL: "/attachments/file"}}
		first := RewriteAttachmentReferences(input, refs)
		if second := RewriteAttachmentReferences(first, refs); first != second {
			t.Fatalf("rewrite is not idempotent: %q -> %q", first, second)
		}
	})
}
