package api

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/maildev"
	"github.com/soulteary/owlmail/internal/mailserver"
	"github.com/soulteary/version-kit/v2"
)

var mailDevRelayAddress = regexp.MustCompile(`^(([^<>()[\]\\.,;:\s@"]+(\.[^<>()[\]\\.,;:\s@"]+)*)|(".+"))@((\[[0-9]{1,3}(\.[0-9]{1,3}){3}\])|(([a-zA-Z\-0-9]+\.)+[a-zA-Z]{2,}))$`)

const mailDevRelayTimeout = 30 * time.Second

// setupMailDevRESTCompatRoutes registers only the current MailDev /api REST
// surface. It intentionally does not register a Socket.IO endpoint.
func (api *API) setupMailDevRESTCompatRoutes(app *fiber.App) {
	compat := app.Group(api.route("/api"))
	compat.Get("/healthz", api.mailDevHealth)
	compat.Get("/config", api.mailDevConfig)
	compat.Get("/email/summary", api.mailDevEmailSummary)
	compat.Get("/email", api.mailDevEmails)
	compat.Post("/email/delete", api.mailDevDeleteEmails)
	compat.Delete("/email/all", api.mailDevDeleteAllEmails)
	compat.Patch("/email/read-all", api.mailDevReadAllEmails)
	compat.Get("/email/:id", api.mailDevEmail)
	compat.Delete("/email/:id", api.mailDevDeleteEmail)
	compat.Get("/email/:id/html", api.mailDevEmailHTML)
	compat.Get("/email/:id/source", api.mailDevEmailSource)
	compat.Get("/email/:id/download", api.mailDevDownloadEmail)
	compat.Get("/email/:id/attachment/:filename", api.mailDevAttachment)
	compat.Post("/email/:id/relay", api.mailDevRelayEmail)
	compat.Post("/email/:id/relay/:relayTo", api.mailDevRelayEmail)
	compat.Get("/reloadMailsFromDirectory", api.mailDevReloadEmails)
}

func (api *API) mailDevHealth(c fiber.Ctx) error {
	return c.JSON(true)
}

func (api *API) mailDevConfig(c fiber.Ctx) error {
	outgoing := api.mailServer.GetOutgoingConfig()
	response := maildev.ConfigResponse{
		Version: version.Default().Version, SMTPPort: api.mailServer.GetPort(),
		OutgoingEnabled: outgoing != nil && outgoing.Host != "",
	}
	if outgoing != nil && outgoing.Host != "" {
		host := outgoing.Host
		response.OutgoingHost = &host
	}
	return c.JSON(response)
}

func (api *API) mailDevEmails(c fiber.Ctx) error {
	queries := c.Queries()
	filters := make(map[string]string, len(queries))
	for key, value := range queries {
		if key != "skip" && key != "limit" && key != "sort" {
			filters[key] = value
		}
	}
	skip, ok := maildev.ParseNonNegativeInt(queries["skip"])
	if !ok {
		skip = 0
	}
	var limit *int
	if parsed, valid := maildev.ParseNonNegativeInt(queries["limit"]); valid {
		limit = &parsed
	}
	pageLimit := int(^uint(0) >> 1)
	if limit != nil {
		pageLimit = *limit
	}
	query := mailserver.EmailQuery{SortBy: "store", Offset: skip, Limit: pageLimit}
	if queries["sort"] == "asc" || queries["sort"] == "desc" {
		query.SortBy = "time"
		query.SortOrder = queries["sort"]
	}
	mailDir := api.mailServer.GetMailDir()
	if len(filters) > 0 {
		query.MatchStoreEmail = func(email *mailserver.Email) bool {
			return maildev.MatchesFilters(maildev.FromEmail(email, mailDir), filters)
		}
	}
	emails, _ := api.mailServer.QueryEmails(query)
	compatEmails := make([]maildev.Email, 0, len(emails))
	for _, email := range emails {
		compatEmails = append(compatEmails, maildev.FromEmail(email, mailDir))
	}
	return c.JSON(compatEmails)
}

func (api *API) mailDevEmailSummary(c fiber.Ctx) error {
	skip, ok := maildev.ParseNonNegativeInt(c.Query("skip"))
	if !ok {
		skip = 0
	}
	limit, ok := maildev.ParseNonNegativeInt(c.Query("limit"))
	if !ok || limit == 0 {
		limit = maildev.DefaultPageSize
	}
	if limit > maildev.MaxPageSize {
		limit = maildev.MaxPageSize
	}
	order := "desc"
	if c.Query("sort") == "asc" {
		order = "asc"
	}
	query := mailserver.EmailQuery{
		Text: c.Query("search"), SearchAddresses: true, ExcludeHTML: true,
		SortBy: "time", SortOrder: order, Offset: skip, Limit: limit,
	}
	if c.Query("unread") == "true" {
		unread := false
		query.Read = &unread
	}
	emails, total := api.mailServer.QueryEmailSummaries(query)
	items := make([]maildev.Summary, 0, len(emails))
	for _, email := range emails {
		items = append(items, maildev.FromEmailSummary(email))
	}
	stats := api.mailServer.GetEmailStats()
	storeTotal, _ := stats["total"].(int)
	unread, _ := stats["unread"].(int)
	return c.JSON(maildev.SummaryResponse{
		Items: items, Total: total, StoreTotal: storeTotal, Unread: unread,
		Skip: skip, Limit: limit,
	})
}

func (api *API) mailDevEmail(c fiber.Ctx) error {
	id := c.Params("id")
	email, err := api.mailServer.GetEmail(id)
	if err != nil {
		return mailDevError(c, http.StatusNotFound, "Email was not found")
	}
	if !email.Read {
		if err := api.mailServer.ReadEmail(id); err != nil {
			return mailDevError(c, http.StatusInternalServerError, err.Error())
		}
		email.Read = true
	}
	return c.JSON(maildev.FromEmail(email, api.mailServer.GetMailDir()))
}

func (api *API) mailDevDeleteEmail(c fiber.Ctx) error {
	id := c.Params("id")
	if _, err := api.mailServer.GetEmail(id); err != nil {
		return mailDevError(c, http.StatusNotFound, "Email was not found")
	}
	if err := api.mailServer.DeleteEmail(id); err != nil {
		return mailDevError(c, http.StatusInternalServerError, err.Error())
	}
	return c.JSON(true)
}

func (api *API) mailDevDeleteEmails(c fiber.Ctx) error {
	var request struct {
		IDs []string `json:"ids"`
	}
	if err := c.Bind().Body(&request); err != nil || request.IDs == nil {
		return mailDevError(c, http.StatusBadRequest, "Request body must include an ids array of email IDs")
	}
	for _, id := range request.IDs {
		if strings.TrimSpace(id) == "" {
			return mailDevError(c, http.StatusBadRequest, "Request body must include an ids array of email IDs")
		}
	}
	seen := make(map[string]struct{}, len(request.IDs))
	response := maildev.BulkDeleteResponse{Deleted: []string{}, NotFound: []string{}}
	for _, id := range request.IDs {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		if _, err := api.mailServer.GetEmail(id); err != nil {
			response.NotFound = append(response.NotFound, id)
			continue
		}
		if err := api.mailServer.DeleteEmail(id); err != nil {
			return mailDevError(c, http.StatusInternalServerError, err.Error())
		}
		response.Deleted = append(response.Deleted, id)
	}
	return c.JSON(response)
}

func (api *API) mailDevDeleteAllEmails(c fiber.Ctx) error {
	if err := api.mailServer.DeleteAllEmail(); err != nil {
		return mailDevError(c, http.StatusInternalServerError, err.Error())
	}
	return c.JSON(true)
}

func (api *API) mailDevReadAllEmails(c fiber.Ctx) error {
	count, err := api.mailServer.ReadAllEmail()
	if err != nil {
		return mailDevError(c, http.StatusInternalServerError, err.Error())
	}
	return c.JSON(count)
}

func (api *API) mailDevEmailHTML(c fiber.Ctx) error {
	email, err := api.mailServer.GetEmail(c.Params("id"))
	if err != nil {
		return mailDevError(c, http.StatusNotFound, "Email was not found")
	}
	if email.HTML == "" {
		return mailDevError(c, http.StatusNotFound, "Email has no HTML content")
	}
	compat := maildev.FromEmail(email, api.mailServer.GetMailDir())
	c.Set(fiber.HeaderContentType, "text/html; charset=utf-8")
	return c.SendString(maildev.EmbedAttachmentURLs(email.HTML, api.basePathname, email.ID, compat.Attachments))
}

func (api *API) mailDevEmailSource(c fiber.Ctx) error {
	id := c.Params("id")
	if _, err := api.mailServer.GetEmail(id); err != nil {
		return mailDevError(c, http.StatusNotFound, "Email was not found")
	}
	content, err := api.mailServer.GetRawEmailContent(id)
	if err != nil {
		return mailDevError(c, http.StatusNotFound, err.Error())
	}
	c.Set(fiber.HeaderContentType, "application/octet-stream")
	return c.Send(content)
}

func (api *API) mailDevDownloadEmail(c fiber.Ctx) error {
	id := c.Params("id")
	if _, err := api.mailServer.GetEmail(id); err != nil {
		return mailDevError(c, http.StatusNotFound, "Email was not found")
	}
	path, err := api.mailServer.GetRawEmail(id)
	if err != nil {
		return mailDevError(c, http.StatusNotFound, err.Error())
	}
	c.Set(fiber.HeaderContentDisposition, "attachment; filename="+id+".eml")
	c.Set(fiber.HeaderContentType, "message/rfc822")
	return c.SendFile(path)
}

func (api *API) mailDevAttachment(c fiber.Ctx) error {
	id := c.Params("id")
	email, err := api.mailServer.GetEmail(id)
	if err != nil {
		return mailDevError(c, http.StatusNotFound, "Email was not found")
	}
	if len(email.Attachments) == 0 {
		return mailDevError(c, http.StatusNotFound, "Email has no attachments")
	}
	found := false
	for _, candidate := range email.Attachments {
		if candidate != nil && candidate.GeneratedFileName == c.Params("filename") {
			found = true
			break
		}
	}
	if !found {
		return mailDevError(c, http.StatusNotFound, "Attachment not found")
	}
	attachment, err := api.mailServer.OpenEmailAttachment(id, c.Params("filename"))
	if err != nil {
		return mailDevError(c, http.StatusNotFound, err.Error())
	}
	contentType := attachment.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	setAttachmentResponseHeaders(c, contentType, c.Params("filename"))
	maxInt := int64(^uint(0) >> 1)
	if attachment.Size >= 0 && attachment.Size <= maxInt {
		return c.SendStream(attachment.Body, int(attachment.Size))
	}
	return c.SendStream(attachment.Body)
}

func (api *API) mailDevRelayEmail(c fiber.Ctx) error {
	id := c.Params("id")
	relayTo := c.Params("relayTo")
	if outgoing := api.mailServer.GetOutgoingConfig(); outgoing == nil || outgoing.Host == "" {
		return mailDevError(c, http.StatusInternalServerError, "Outgoing mail not configured")
	}
	email, err := api.mailServer.GetEmail(id)
	if err != nil {
		return mailDevError(c, http.StatusInternalServerError, "Email was not found")
	}
	if relayTo != "" && !mailDevRelayAddress.MatchString(relayTo) {
		return mailDevError(c, http.StatusBadRequest, fmt.Sprintf("Incorrect email address provided: %s", relayTo))
	}
	ctx, cancel := context.WithTimeout(c.RequestCtx(), mailDevRelayTimeout)
	defer cancel()
	if err := api.mailServer.RelayMailAndWait(ctx, email, relayTo); err != nil {
		return mailDevError(c, http.StatusInternalServerError, err.Error())
	}
	return c.JSON(true)
}

func (api *API) mailDevReloadEmails(c fiber.Ctx) error {
	if err := api.mailServer.LoadMailsFromDirectory(); err != nil {
		return mailDevError(c, http.StatusInternalServerError, err.Error())
	}
	return c.JSON(true)
}

func mailDevError(c fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(fiber.Map{"error": message})
}
