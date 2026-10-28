package api

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/mailserver"
	"golang.org/x/net/html"
)

func TestMailDevCIDSMTPAttachmentRoundTrip(t *testing.T) {
	const svg = `<svg xmlns="http://www.w3.org/2000/svg"><text>inline CID</text></svg>`
	for _, base := range []string{"", "/mail"} {
		t.Run("base="+base, func(t *testing.T) {
			directory := t.TempDir()
			server, err := mailserver.NewMailServerWithOptions(0, "127.0.0.1", directory, mailserver.ServerOptions{AllowEphemeralPort: true})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = server.Close() })
			api := NewAPI(server, 1080, "127.0.0.1")
			api.SetMailDevRESTCompat(true)
			if err := api.SetBasePathname(base); err != nil {
				t.Fatal(err)
			}
			ready := make(chan struct{})
			finished := make(chan error, 1)
			go func() { finished <- server.ListenWithReady(func() { close(ready) }) }()
			select {
			case <-ready:
			case err := <-finished:
				t.Fatal(err)
			case <-time.After(5 * time.Second):
				t.Fatal("SMTP startup timeout")
			}
			address, ok := server.BoundAddress()
			if !ok {
				t.Fatal("missing actual SMTP address")
			}
			conn, err := net.DialTimeout("tcp", net.JoinHostPort(address.Host, fmt.Sprint(address.Port)), 3*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
			client, err := smtp.NewClient(conn, "localhost")
			if err != nil {
				_ = conn.Close()
				t.Fatal(err)
			}
			defer func() { _ = client.Close() }()
			if err := client.Mail("sender@example.test"); err != nil {
				t.Fatal(err)
			}
			if err := client.Rcpt("recipient@example.test"); err != nil {
				t.Fatal(err)
			}
			writer, err := client.Data()
			if err != nil {
				t.Fatal(err)
			}
			message := "From: sender@example.test\r\nTo: recipient@example.test\r\nSubject: CID round trip\r\nMIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=outer-boundary\r\n\r\n--outer-boundary\r\nContent-Type: text/plain\r\n\r\nPlain fallback\r\n--outer-boundary\r\nContent-Type: multipart/related; boundary=cid-boundary\r\n\r\n" +
				"--cid-boundary\r\nContent-Type: text/html; charset=utf-8\r\n\r\n<img src=\"cid:logo%40mail\"><a href=\"CID:logo@mail\">download</a>\r\n" +
				"--cid-boundary\r\nContent-Type: image/svg+xml\r\nContent-Disposition: inline; filename=\"logo image.svg\"\r\nContent-ID: <logo@mail>\r\n\r\n" + svg + "\r\n--cid-boundary--\r\n--outer-boundary--\r\n"
			if _, err := io.WriteString(writer, message); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			_ = client.Quit()
			emails := server.GetAllEmail()
			if len(emails) != 1 {
				t.Fatalf("received %d emails", len(emails))
			}
			if emails[0].Text != "Plain fallback" {
				t.Fatalf("nested MIME lost text: %q", emails[0].Text)
			}
			id := emails[0].ID
			check := func(api *API) {
				t.Helper()
				response := compatRequest(t, api, http.MethodGet, base+"/api/email/"+id+"/html", nil, "")
				body, err := io.ReadAll(response.Body)
				_ = response.Body.Close()
				if err != nil || response.StatusCode != 200 {
					t.Fatalf("HTML status %d: %s, %v", response.StatusCode, body, err)
				}
				var targets []string
				tokenizer := html.NewTokenizer(strings.NewReader(string(body)))
				for tokenizer.Next() != html.ErrorToken {
					token := tokenizer.Token()
					for _, attr := range token.Attr {
						if attr.Key == "src" || attr.Key == "href" {
							targets = append(targets, attr.Val)
						}
					}
				}
				if len(targets) != 2 || targets[0] != targets[1] || !strings.HasPrefix(targets[0], base+"/api/email/"+id+"/attachment/") {
					t.Fatalf("bad CID targets: %v in %s", targets, body)
				}
				attachment := compatRequest(t, api, http.MethodGet, targets[0], nil, "")
				bytes, err := io.ReadAll(attachment.Body)
				_ = attachment.Body.Close()
				if err != nil || attachment.StatusCode != 200 || string(bytes) != svg || !strings.HasPrefix(attachment.Header.Get("Content-Type"), "image/svg+xml") {
					t.Fatalf("attachment status %d content %q type %q err %v", attachment.StatusCode, bytes, attachment.Header.Get("Content-Type"), err)
				}
			}
			check(api)
			if err := server.Close(); err != nil {
				t.Fatal(err)
			}
			select {
			case <-finished:
			case <-time.After(5 * time.Second):
				t.Fatal("SMTP shutdown timeout")
			}
			restored, err := mailserver.NewMailServer(1025, "127.0.0.1", directory)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = restored.Close() }()
			if err := restored.LoadMailsFromDirectory(); err != nil {
				t.Fatal(err)
			}
			restoredAPI := NewAPI(restored, 1080, "127.0.0.1")
			restoredAPI.SetMailDevRESTCompat(true)
			if err := restoredAPI.SetBasePathname(base); err != nil {
				t.Fatal(err)
			}
			check(restoredAPI)
		})
	}
}
