package mailserver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReplyToParsingAndRecovery(t *testing.T) {
	cases := []struct {
		name, header string
		want         []string
		nameFirst    string
	}{
		{"multiple", "Reply-To: =?UTF-8?Q?Support_=E2=9C=93?= <reply@example.test>, backup@example.test\r\n", []string{"reply@example.test", "backup@example.test"}, "Support ✓"},
		{"single", "Reply-To: reply@example.test\r\n", []string{"reply@example.test"}, ""},
		{"absent", "", nil, ""},
		{"invalid", "Reply-To: broken <\r\n", nil, ""},
		{"duplicate", "Reply-To: first@example.test\r\nReply-To: second@example.test\r\n", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			server, err := NewMailServer(1025, "localhost", dir)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = server.Close() })
			raw := "From: sender@example.test\r\nTo: receiver@example.test\r\nSubject: reply\r\n" + tc.header + "In-Reply-To: <parent@example.test>\r\n\r\nbody"
			parsed, err := server.parseEmail("reply-id", strings.NewReader(raw), nil, false, false)
			if err != nil {
				t.Fatal(err)
			}
			checkReplyTo := func(email *Email) {
				t.Helper()
				if len(email.ReplyTo) != len(tc.want) {
					t.Fatalf("ReplyTo = %#v, want %v", email.ReplyTo, tc.want)
				}
				for i, want := range tc.want {
					if email.ReplyTo[i].Address != want {
						t.Fatalf("ReplyTo[%d] = %#v", i, email.ReplyTo[i])
					}
				}
				if len(tc.want) > 0 && email.ReplyTo[0].Name != tc.nameFirst {
					t.Fatalf("decoded name = %q", email.ReplyTo[0].Name)
				}
				if tc.header != "" && email.Headers["Reply-To"] == nil {
					t.Fatal("raw Reply-To missing")
				}
				if email.Headers["In-Reply-To"] != "<parent@example.test>" {
					t.Fatal("In-Reply-To changed")
				}
				serialized, err := json.Marshal(email)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(serialized), `"replyTo"`) != (len(tc.want) > 0) {
					t.Fatalf("optional field = %s", serialized)
				}
			}
			checkReplyTo(parsed)
			if err := os.WriteFile(filepath.Join(dir, "reply-id.eml"), []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			if err := server.LoadMailsFromDirectory(); err != nil {
				t.Fatal(err)
			}
			recovered, err := server.GetEmail("reply-id")
			if err != nil {
				t.Fatal(err)
			}
			checkReplyTo(recovered)
			if len(recovered.ReplyTo) > 0 {
				recovered.ReplyTo[0].Address = "mutated@example.test"
				again, err := server.GetEmail("reply-id")
				if err != nil {
					t.Fatal(err)
				}
				checkReplyTo(again)
			}
		})
	}
}
