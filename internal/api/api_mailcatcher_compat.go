package api

import (
	"fmt"
	"mime"
	"net/http"
	"regexp"
	"strings"

	"github.com/emersion/go-message/mail"
	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/mailserver"
	"github.com/soulteary/owlmail/internal/types"
)

var mailCatcherCIDReference = regexp.MustCompile(`(?i)cid:([^'" >]+)`)

func (api *API) setupMailCatcherRESTCompatRoutes(app *fiber.App) {
	messages := app.Group(api.route("/messages"))
	messages.Get("", api.mailCatcherMessages)
	messages.Delete("", api.mailCatcherDeleteAll)
	messages.Get("/:id.json", api.mailCatcherMessage)
	messages.Get("/:id.html", api.mailCatcherHTML)
	messages.Get("/:id.plain", api.mailCatcherPlain)
	messages.Get("/:id.source", api.mailCatcherSource)
	messages.Get("/:id.eml", api.mailCatcherEML)
	messages.Get("/:id/parts/:cid", api.mailCatcherPart)
	messages.Delete("/:id", api.mailCatcherDelete)
}

func (api *API) mailCatcherMessages(c fiber.Ctx) error {
	summaries, _ := api.mailServer.QueryEmailSummaries(mailserver.EmailQuery{SortBy: "time", SortOrder: "asc", Limit: 1000})
	result := make([]fiber.Map, 0, len(summaries))
	for _, summary := range summaries {
		email, err := api.mailServer.GetEmail(summary.ID)
		if err == nil {
			result = append(result, mailCatcherMessageDTO(email, false))
		}
	}
	return c.JSON(result)
}

func (api *API) mailCatcherMessage(c fiber.Ctx) error {
	email, err := api.mailServer.GetEmail(c.Params("id"))
	if err != nil {
		return c.Status(http.StatusNotFound).SendString("Message does not exist")
	}
	return c.JSON(mailCatcherMessageDTO(email, true))
}

func mailCatcherMessageDTO(email *types.Email, detail bool) fiber.Map {
	sender := ""
	recipients := make([]string, 0)
	if email.Envelope != nil {
		sender = angleAddress(email.Envelope.From)
		for _, recipient := range append(append([]string{}, email.Envelope.To...), append(email.Envelope.CC, email.Envelope.BCC...)...) {
			recipients = append(recipients, angleAddress(recipient))
		}
	} else {
		if len(email.From) > 0 && email.From[0] != nil {
			sender = angleAddress(email.From[0].Address)
		}
		for _, recipient := range append(append([]*mail.Address{}, email.To...), email.CC...) {
			if recipient != nil {
				recipients = append(recipients, angleAddress(recipient.Address))
			}
		}
	}
	result := fiber.Map{"id": email.ID, "sender": sender, "recipients": recipients, "subject": email.Subject, "size": email.Size, "created_at": email.Time}
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
	for _, attachment := range email.Attachments {
		if attachment == nil {
			continue
		}
		attachments = append(attachments, fiber.Map{"cid": strings.Trim(attachment.ContentID, "<>"), "type": attachment.ContentType, "filename": attachment.FileName, "size": attachment.Size})
	}
	result["formats"] = formats
	result["attachments"] = attachments
	return result
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
	body := mailCatcherCIDReference.ReplaceAllString(email.HTML, prefix+"$1")
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
	path, err := api.mailServer.GetRawEmail(id)
	if err != nil {
		return c.Status(http.StatusNotFound).SendString("Message does not exist")
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
	cid := strings.Trim(c.Params("cid"), "<>")
	var selected *types.Attachment
	for _, attachment := range email.Attachments {
		if attachment != nil && strings.Trim(attachment.ContentID, "<>") == cid {
			selected = attachment
			break
		}
	}
	if selected == nil {
		return c.Status(http.StatusNotFound).SendString("Message part does not exist")
	}
	reader, err := api.mailServer.OpenEmailAttachment(email.ID, selected.GeneratedFileName)
	if err != nil {
		return c.Status(http.StatusNotFound).SendString("Message part does not exist")
	}
	contentType := reader.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Set(fiber.HeaderContentType, contentType)
	c.Set("X-Content-Type-Options", "nosniff")
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
	if err := api.mailServer.DeleteEmail(c.Params("id")); err != nil {
		return c.Status(http.StatusNotFound).SendString("Message does not exist")
	}
	return c.SendStatus(http.StatusNoContent)
}

func (api *API) mailCatcherDeleteAll(c fiber.Ctx) error {
	if err := api.mailServer.DeleteAllEmail(); err != nil {
		return c.Status(http.StatusInternalServerError).SendString(fmt.Sprintf("Delete failed: %v", err))
	}
	return c.SendStatus(http.StatusNoContent)
}
