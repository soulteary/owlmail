package api

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/emersion/go-message/mail"
	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/mailserver"
	"github.com/soulteary/owlmail/internal/types"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var mailCatcherURIAttributes = map[string]struct{}{
	"action": {}, "background": {}, "cite": {}, "data": {}, "formaction": {},
	"href": {}, "poster": {}, "src": {},
}

func (api *API) setupMailCatcherRESTCompatRoutes(app *fiber.App) {
	messages := app.Group(api.route("/messages"))
	messages.Get("", api.mailCatcherMessages)
	messages.Delete("", api.mailCatcherDeleteAll)
	messages.Get("/:id.json", api.mailCatcherMessage)
	messages.Get("/:id.html", api.mailCatcherHTML)
	messages.Get("/:id.plain", api.mailCatcherPlain)
	messages.Get("/:id.source", api.mailCatcherSource)
	messages.Get("/:id.eml", api.mailCatcherEML)
	messages.Get("/:id/parts/*", api.mailCatcherPart)
	messages.Delete("/:id", api.mailCatcherDelete)
}

func (api *API) mailCatcherMessages(c fiber.Ctx) error {
	summaries, _ := api.mailServer.QueryEmailSummaries(mailserver.EmailQuery{SortBy: "received", SortOrder: "desc", Limit: int(^uint(0) >> 1)})
	result := make([]fiber.Map, 0, len(summaries))
	for _, summary := range summaries {
		result = append(result, mailCatcherSummaryDTO(summary))
	}
	return c.JSON(result)
}

func (api *API) mailCatcherMessage(c fiber.Ctx) error {
	email, receivedAt, err := api.mailServer.GetEmailWithReceivedAt(c.Params("id"))
	if err != nil {
		return c.Status(http.StatusNotFound).SendString("Message does not exist")
	}
	return c.JSON(mailCatcherMessageDTO(email, receivedAt, true))
}

func mailCatcherSummaryDTO(email mailserver.EmailSummary) fiber.Map {
	sender := email.EnvelopeFrom
	recipients := append([]string(nil), email.EnvelopeTo...)
	if sender == "" && !email.SMTPEnvelope && len(email.From) > 0 {
		sender = email.From[0].Address
	}
	if !email.SMTPEnvelope {
		recipients = recipients[:0]
		for _, address := range append(append([]mailserver.EmailSummaryAddress{}, email.To...), email.CC...) {
			recipients = append(recipients, address.Address)
		}
	}
	for i := range recipients {
		recipients[i] = angleAddress(recipients[i])
	}
	return fiber.Map{"id": email.ID, "sender": angleAddress(sender), "recipients": recipients, "subject": email.Subject, "size": email.Size, "created_at": email.ReceivedAt}
}

func mailCatcherMessageDTO(email *types.Email, createdAt time.Time, detail bool) fiber.Map {
	sender := ""
	recipients := make([]string, 0)
	if email.Envelope != nil {
		sender = angleAddress(email.Envelope.From)
		for _, recipient := range append(append([]string{}, email.Envelope.To...), append(email.Envelope.CC, email.Envelope.BCC...)...) {
			recipients = append(recipients, angleAddress(recipient))
		}
	}
	if sender == "" && (email.Envelope == nil || !email.Envelope.SMTPTransaction) && len(email.From) > 0 && email.From[0] != nil {
		sender = angleAddress(email.From[0].Address)
	}
	if email.Envelope == nil || !email.Envelope.SMTPTransaction {
		recipients = recipients[:0]
		for _, recipient := range append(append([]*mail.Address{}, email.To...), email.CC...) {
			if recipient != nil {
				recipients = append(recipients, angleAddress(recipient.Address))
			}
		}
	}
	result := fiber.Map{"id": email.ID, "sender": sender, "recipients": recipients, "subject": email.Subject, "size": email.Size, "created_at": createdAt}
	if !detail {
		return result
	}
	formats := []string{"source"}
	if email.HTML != "" {
		formats = append(formats, "html")
	}
	if email.Text != "" {
		formats = append(formats, "plain")
	}
	attachments := make([]fiber.Map, 0, len(email.Attachments))
	for index, attachment := range email.Attachments {
		if attachment == nil {
			continue
		}
		attachments = append(attachments, fiber.Map{"cid": mailCatcherPartID(attachment, index), "type": attachment.ContentType, "filename": attachment.FileName, "size": attachment.Size})
	}
	result["formats"] = formats
	result["attachments"] = attachments
	return result
}

func mailCatcherPartID(attachment *types.Attachment, index int) string {
	if cid := strings.Trim(attachment.ContentID, "<>"); cid != "" {
		return cid
	}
	if attachment.GeneratedFileName != "" {
		return "owlmail-attachment-" + attachment.GeneratedFileName
	}
	return fmt.Sprintf("owlmail-attachment-%d", index+1)
}

func rewriteMailCatcherCIDReferences(markup, prefix string) (string, error) {
	contextNode := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, err := html.ParseFragment(strings.NewReader(markup), contextNode)
	if err != nil {
		return "", err
	}
	var rewrite func(*html.Node)
	rewrite = func(node *html.Node) {
		if node.Type == html.ElementNode {
			for index := range node.Attr {
				attribute := &node.Attr[index]
				if _, ok := mailCatcherURIAttributes[strings.ToLower(attribute.Key)]; !ok {
					continue
				}
				value := strings.TrimSpace(attribute.Val)
				if len(value) < 4 || !strings.EqualFold(value[:4], "cid:") {
					continue
				}
				cid := value[4:]
				if decoded, decodeErr := url.PathUnescape(cid); decodeErr == nil {
					cid = decoded
				}
				attribute.Val = prefix + url.PathEscape(strings.Trim(cid, "<>"))
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			rewrite(child)
		}
	}
	var body strings.Builder
	for _, node := range nodes {
		rewrite(node)
		if err := html.Render(&body, node); err != nil {
			return "", err
		}
	}
	return body.String(), nil
}

func angleAddress(address string) string {
	address = strings.Trim(strings.TrimSpace(address), "<>")
	if address == "" {
		return ""
	}
	return "<" + address + ">"
}

func (api *API) mailCatcherHTML(c fiber.Ctx) error {
	email, err := api.mailServer.GetEmail(c.Params("id"))
	if err != nil || email.HTML == "" {
		return c.Status(http.StatusNotFound).SendString("Message format does not exist")
	}
	prefix := api.route("/messages/" + email.ID + "/parts/")
	body, err := rewriteMailCatcherCIDReferences(email.HTML, prefix)
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("Message format could not be rendered")
	}
	c.Set(fiber.HeaderContentType, "text/html; charset=utf-8")
	return c.SendString(body)
}

func (api *API) mailCatcherPlain(c fiber.Ctx) error {
	email, err := api.mailServer.GetEmail(c.Params("id"))
	if err != nil || email.Text == "" {
		return c.Status(http.StatusNotFound).SendString("Message format does not exist")
	}
	c.Set(fiber.HeaderContentType, "text/plain; charset=utf-8")
	return c.SendString(email.Text)
}

func (api *API) mailCatcherSource(c fiber.Ctx) error {
	return api.mailCatcherRaw(c, "text/plain; charset=utf-8", false)
}

func (api *API) mailCatcherEML(c fiber.Ctx) error {
	return api.mailCatcherRaw(c, "message/rfc822", true)
}

func (api *API) mailCatcherRaw(c fiber.Ctx, contentType string, download bool) error {
	id := c.Params("id")
	if _, err := api.mailServer.GetEmail(id); err != nil {
		return c.Status(http.StatusNotFound).SendString("Message does not exist")
	}
	path, err := api.mailServer.GetRawEmail(id)
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("Message source could not be read")
	}
	c.Set(fiber.HeaderContentType, contentType)
	if download {
		c.Set(fiber.HeaderContentDisposition, mime.FormatMediaType("attachment", map[string]string{"filename": id + ".eml"}))
	}
	return c.SendFile(path)
}

func (api *API) mailCatcherPart(c fiber.Ctx) error {
	email, err := api.mailServer.GetEmail(c.Params("id"))
	if err != nil {
		return c.Status(http.StatusNotFound).SendString("Message does not exist")
	}
	cid := c.Params("*")
	if decoded, decodeErr := url.PathUnescape(cid); decodeErr == nil {
		cid = decoded
	}
	cid = strings.Trim(cid, "<>")
	var selected *types.Attachment
	for index, attachment := range email.Attachments {
		if attachment != nil && mailCatcherPartID(attachment, index) == cid {
			selected = attachment
			break
		}
	}
	if selected == nil {
		return c.Status(http.StatusNotFound).SendString("Message part does not exist")
	}
	reader, err := api.mailServer.OpenEmailAttachment(email.ID, selected.GeneratedFileName)
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("Message part could not be read")
	}
	contentType := reader.ContentType
	setAttachmentResponseHeaders(c, contentType, selected.FileName)
	if strings.EqualFold(selected.ContentDisposition, "attachment") {
		c.Set(fiber.HeaderContentDisposition, mime.FormatMediaType("attachment", map[string]string{"filename": selected.FileName}))
	}
	maxInt := int64(^uint(0) >> 1)
	if reader.Size >= 0 && reader.Size <= maxInt {
		return c.SendStream(reader.Body, int(reader.Size))
	}
	return c.SendStream(reader.Body)
}

func (api *API) mailCatcherDelete(c fiber.Ctx) error {
	if _, err := api.mailServer.GetEmail(c.Params("id")); err != nil {
		return c.Status(http.StatusNotFound).SendString("Message does not exist")
	}
	if err := api.mailServer.DeleteEmail(c.Params("id")); err != nil {
		if errors.Is(err, mailserver.ErrEmailNotFound) {
			return c.Status(http.StatusNotFound).SendString("Message does not exist")
		}
		return c.Status(http.StatusInternalServerError).SendString("Message deletion failed")
	}
	return c.SendStatus(http.StatusNoContent)
}

func (api *API) mailCatcherDeleteAll(c fiber.Ctx) error {
	if err := api.mailServer.DeleteAllEmail(); err != nil {
		return c.Status(http.StatusInternalServerError).SendString(fmt.Sprintf("Delete failed: %v", err))
	}
	return c.SendStatus(http.StatusNoContent)
}
