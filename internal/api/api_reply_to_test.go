package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestReplyToNativeAndMailDevResponses(t *testing.T) {
	for _, tc := range []struct {
		name, header string
		present      bool
	}{
		{"valid", "Reply-To: Support <reply@example.test>, backup@example.test\r\n", true},
		{"absent", "", false},
		{"invalid", "Reply-To: broken <\r\n", false},
		{"duplicate", "Reply-To: a@example.test\r\nReply-To: b@example.test\r\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api, server, dir := setupTestAPI(t)
			defer func() { _ = server.Close() }()
			api.SetMailDevRESTCompat(true)
			raw := "From: sender@example.test\r\nTo: receiver@example.test\r\nSubject: Reply\r\n" + tc.header + "In-Reply-To: <parent@example.test>\r\n\r\nbody"
			if err := os.WriteFile(filepath.Join(dir, "reply-api.eml"), []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			if err := server.LoadMailsFromDirectory(); err != nil {
				t.Fatal(err)
			}
			for _, route := range []string{"/api/v1/emails/reply-api", "/api/email/reply-api"} {
				req, err := http.NewRequest(http.MethodGet, route, nil)
				if err != nil {
					t.Fatal(err)
				}
				response, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
				if err != nil {
					t.Fatal(err)
				}
				var body map[string]json.RawMessage
				err = json.NewDecoder(response.Body).Decode(&body)
				_ = response.Body.Close()
				if err != nil {
					t.Fatal(err)
				}
				if response.StatusCode != http.StatusOK {
					t.Fatalf("%s: %d", route, response.StatusCode)
				}
				reply, ok := body["replyTo"]
				if ok != tc.present {
					t.Fatalf("%s replyTo presence = %t, body=%v", route, ok, body)
				}
				if ok {
					var addresses []map[string]string
					if err := json.Unmarshal(reply, &addresses); err != nil {
						t.Fatal(err)
					}
					addressKey, nameKey := "Address", "Name"
					if route == "/api/email/reply-api" {
						addressKey, nameKey = "address", "name"
					}
					if len(addresses) != 2 || addresses[0][addressKey] != "reply@example.test" || addresses[0][nameKey] != "Support" || addresses[1][addressKey] != "backup@example.test" {
						t.Fatalf("%s replyTo=%s", route, reply)
					}
				}
				if route == "/api/email/reply-api" {
					var inReplyTo string
					if err := json.Unmarshal(body["inReplyTo"], &inReplyTo); err != nil {
						t.Fatal(err)
					}
					if inReplyTo != "<parent@example.test>" {
						t.Fatalf("inReplyTo changed: %s", body["inReplyTo"])
					}
				}
			}
		})
	}
}
