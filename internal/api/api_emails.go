package api

import (
	"archive/zip"
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/common"
	"github.com/soulteary/owlmail/internal/mailserver"
	"github.com/soulteary/owlmail/internal/types"
)

const (
	maxExportMessages = 1000
	maxExportBytes    = 256 << 20
)

// EmailPreview represents a lightweight email preview.
type EmailPreview = mailserver.EmailPreview

// getAllEmails handles GET /api/v1/emails
func (api *API) getAllEmails(c fiber.Ctx) error {
	query := parseEmailQuery(c)
	emails, total := api.mailServer.QueryEmails(query)

	return c.JSON(fiber.Map{
		"total":  total,
		"limit":  query.Limit,
		"offset": query.Offset,
		"emails": emails,
	})
}

// getEmailByID handles GET /api/v1/emails/:id
func (api *API) getEmailByID(c fiber.Ctx) error {
	id := c.Params("id")
	email, err := api.mailServer.GetEmail(id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse(ErrorCodeEmailNotFound, "Email not found"))
	}
	return c.JSON(email)
}

// getEmailHTML handles GET /api/v1/emails/:id/html
func (api *API) getEmailHTML(c fiber.Ctx) error {
	id := c.Params("id")
	html, err := api.mailServer.GetEmailHTML(id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse(ErrorCodeEmailNotFound, "Email not found"))
	}
	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.SendString(html)
}

// getAttachment handles GET /api/v1/emails/:id/attachments/:filename
func (api *API) getAttachment(c fiber.Ctx) error {
	id := c.Params("id")
	filename := c.Params("filename")

	attachment, err := api.mailServer.OpenEmailAttachment(id, filename)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse(ErrorCodeEmailNotFound, err.Error()))
	}

	c.Set("Content-Type", attachment.ContentType)
	maxInt := int64(^uint(0) >> 1)
	if attachment.Size >= 0 && attachment.Size <= maxInt {
		return c.SendStream(attachment.Body, int(attachment.Size))
	}
	return c.SendStream(attachment.Body)
}

// downloadEmail handles GET /api/v1/emails/:id/raw
func (api *API) downloadEmail(c fiber.Ctx) error {
	id := c.Params("id")

	email, err := api.mailServer.GetEmail(id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse(ErrorCodeEmailNotFound, "Email not found"))
	}

	emlPath, err := api.mailServer.GetRawEmail(id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse(ErrorCodeEmailFileNotFound, "Email file not found"))
	}

	filename := fmt.Sprintf("%s.eml", email.ID)
	if email.Subject != "" {
		filename = sanitizeFilename(fmt.Sprintf("%s-%s", email.ID, email.Subject)) + ".eml"
	}

	c.Set("Content-Type", "message/rfc822")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	return c.SendFile(emlPath)
}

// getEmailSource handles GET /api/v1/emails/:id/source
func (api *API) getEmailSource(c fiber.Ctx) error {
	id := c.Params("id")

	content, err := api.mailServer.GetRawEmailContent(id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse(ErrorCodeEmailNotFound, err.Error()))
	}

	c.Set("Content-Type", "text/plain; charset=utf-8")
	return c.Send(content)
}

// deleteEmail handles DELETE /api/v1/emails/:id
func (api *API) deleteEmail(c fiber.Ctx) error {
	id := c.Params("id")
	err := api.relayJobs.protectSourceDeletion([]string{id}, func() error { return api.mailServer.DeleteEmail(id) })
	if errors.Is(err, errRelaySourceInUse) || errors.Is(err, mailserver.ErrEmailSourceInUse) {
		return c.Status(fiber.StatusConflict).JSON(ErrorResponse(ErrorCodeRelayFailed, "Email has a pending relay job"))
	}
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse(ErrorCodeEmailNotFound, err.Error()))
	}
	return c.JSON(SuccessResponse(SuccessCodeEmailDeleted, "Email deleted", nil))
}

// deleteAllEmails handles DELETE /api/v1/emails
func (api *API) deleteAllEmails(c fiber.Ctx) error {
	err := api.relayJobs.protectSourceDeletion(nil, api.mailServer.DeleteAllEmail)
	if errors.Is(err, errRelaySourceInUse) || errors.Is(err, mailserver.ErrEmailSourceInUse) {
		return c.Status(fiber.StatusConflict).JSON(ErrorResponse(ErrorCodeRelayFailed, "An email has a pending relay job"))
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse(ErrorCodeInvalidRequest, err.Error()))
	}
	return c.JSON(SuccessResponse(SuccessCodeAllEmailsDeleted, "All emails deleted", nil))
}

// readAllEmails handles PATCH /api/v1/emails/read
func (api *API) readAllEmails(c fiber.Ctx) error {
	count, err := api.mailServer.ReadAllEmail()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse(ErrorCodeInvalidRequest, err.Error()))
	}
	return c.JSON(SuccessResponse(SuccessCodeAllEmailsMarkedRead, "All emails marked as read", fiber.Map{"count": count}))
}

// readEmail handles PATCH /api/v1/emails/:id/read
func (api *API) readEmail(c fiber.Ctx) error {
	id := c.Params("id")
	if err := api.mailServer.ReadEmail(id); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse(ErrorCodeEmailNotFound, err.Error()))
	}
	return c.JSON(SuccessResponse(SuccessCodeEmailMarkedRead, "Email marked as read", fiber.Map{"id": id}))
}

// getEmailStats handles GET /api/v1/emails/stats
func (api *API) getEmailStats(c fiber.Ctx) error {
	stats := api.mailServer.GetEmailStats()
	return c.JSON(stats)
}

// reloadMailsFromDirectory handles POST /api/v1/emails/reload
func (api *API) reloadMailsFromDirectory(c fiber.Ctx) error {
	if err := api.mailServer.LoadMailsFromDirectory(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse(ErrorCodeInvalidRequest, "Failed to reload mails from directory: "+err.Error()))
	}
	return c.JSON(SuccessResponse(SuccessCodeMailsReloaded, "Mails reloaded from directory successfully", nil))
}

// getEmailPreviews handles GET /api/v1/emails/preview
func (api *API) getEmailPreviews(c fiber.Ctx) error {
	query := parseEmailQuery(c)
	previews, total := api.mailServer.QueryEmailPreviews(query)

	return c.JSON(fiber.Map{
		"total":    total,
		"limit":    query.Limit,
		"offset":   query.Offset,
		"previews": previews,
	})
}

func parseEmailQuery(c fiber.Ctx) mailserver.EmailQuery {
	limitStr := c.Query("limit", "50")
	offsetStr := c.Query("offset", "0")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	query := mailserver.EmailQuery{
		Text:      c.Query("q"),
		From:      c.Query("from"),
		To:        c.Query("to"),
		SortBy:    c.Query("sortBy", ""),
		SortOrder: c.Query("sortOrder", "desc"),
		Offset:    offset,
		Limit:     limit,
	}
	if dateFrom, err := time.Parse("2006-01-02", c.Query("dateFrom")); err == nil {
		query.DateFrom = &dateFrom
	}
	if dateTo, err := time.Parse("2006-01-02", c.Query("dateTo")); err == nil {
		dateTo = dateTo.Add(24 * time.Hour)
		query.DateTo = &dateTo
	}
	if read := c.Query("read"); read != "" {
		readValue := read == "true"
		query.Read = &readValue
	}
	return query
}

// batchDeleteEmails handles DELETE /api/v1/emails/batch
func (api *API) batchDeleteEmails(c fiber.Ctx) error {
	var request struct {
		IDs []string `json:"ids"`
	}

	if err := c.Bind().Body(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse(ErrorCodeInvalidRequest, "Invalid request: "+err.Error()))
	}

	if len(request.IDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse(ErrorCodeNoEmailIDsProvided, "No email IDs provided"))
	}
	successCount := 0
	failedCount := 0
	failedIDs := make([]string, 0)
	if err := api.relayJobs.protectSourceDeletion(request.IDs, func() error {
		for _, id := range request.IDs {
			if err := api.mailServer.DeleteEmail(id); err != nil {
				failedCount++
				failedIDs = append(failedIDs, id)
			} else {
				successCount++
			}
		}
		return nil
	}); errors.Is(err, errRelaySourceInUse) || errors.Is(err, mailserver.ErrEmailSourceInUse) {
		return c.Status(fiber.StatusConflict).JSON(ErrorResponse(ErrorCodeRelayFailed, "An email has a pending relay job"))
	}

	return c.JSON(fiber.Map{
		"code":      SuccessCodeBatchDeleteCompleted,
		"message":   "Batch delete completed",
		"success":   successCount,
		"failed":    failedCount,
		"failedIDs": failedIDs,
		"total":     len(request.IDs),
	})
}

// batchReadEmails handles PATCH /api/v1/emails/batch/read
func (api *API) batchReadEmails(c fiber.Ctx) error {
	var request struct {
		IDs []string `json:"ids"`
	}

	if err := c.Bind().Body(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse(ErrorCodeInvalidRequest, "Invalid request: "+err.Error()))
	}

	if len(request.IDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse(ErrorCodeNoEmailIDsProvided, "No email IDs provided"))
	}

	successCount := 0
	failedCount := 0
	failedIDs := make([]string, 0)

	for _, id := range request.IDs {
		email, err := api.mailServer.GetEmail(id)
		if err != nil {
			failedCount++
			failedIDs = append(failedIDs, id)
			continue
		}

		if !email.Read {
			if err := api.mailServer.ReadEmail(id); err != nil {
				failedCount++
				failedIDs = append(failedIDs, id)
				continue
			}
			successCount++
		}
	}

	return c.JSON(fiber.Map{
		"code":      SuccessCodeBatchReadCompleted,
		"message":   "Batch read completed",
		"success":   successCount,
		"failed":    failedCount,
		"failedIDs": failedIDs,
		"total":     len(request.IDs),
	})
}

// exportEmails handles GET /api/v1/emails/export
func (api *API) exportEmails(c fiber.Ctx) error {
	idsParam := c.Query("ids")
	query := c.Query("q")
	from := c.Query("from")
	to := c.Query("to")
	dateFrom := c.Query("dateFrom")
	dateTo := c.Query("dateTo")
	read := c.Query("read")

	emails := api.mailServer.GetAllEmail()
	var filtered []*types.Email

	if idsParam != "" {
		ids := strings.Split(idsParam, ",")
		idMap := make(map[string]bool)
		for _, id := range ids {
			idMap[strings.TrimSpace(id)] = true
		}
		for _, email := range emails {
			if idMap[email.ID] {
				filtered = append(filtered, email)
			}
		}
	} else {
		filtered = applyEmailFilters(emails, query, from, to, dateFrom, dateTo, read)
	}

	if len(filtered) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse(ErrorCodeNoEmailsToExport, "No emails found to export"))
	}
	if len(filtered) > maxExportMessages {
		return c.Status(fiber.StatusRequestEntityTooLarge).JSON(ErrorResponse(ErrorCodeInvalidRequest, fmt.Sprintf("Export is limited to %d emails", maxExportMessages)))
	}
	type exportItem struct {
		email *types.Email
		path  string
	}
	items := make([]exportItem, 0, len(filtered))
	var totalBytes int64
	for _, email := range filtered {
		emlPath, err := api.mailServer.GetRawEmail(email.ID)
		if err != nil {
			continue
		}
		stat, err := os.Stat(emlPath)
		if err != nil {
			continue
		}
		totalBytes += stat.Size()
		if totalBytes > maxExportBytes {
			return c.Status(fiber.StatusRequestEntityTooLarge).JSON(ErrorResponse(ErrorCodeInvalidRequest, fmt.Sprintf("Export source is limited to %d bytes", maxExportBytes)))
		}
		items = append(items, exportItem{email: email, path: emlPath})
	}
	if len(items) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse(ErrorCodeNoEmailsToExport, "No readable emails found to export"))
	}

	c.Set("Content-Type", "application/zip")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=emails_%s.zip", time.Now().Format("20060102_150405")))
	return c.SendStreamWriter(func(writer *bufio.Writer) {
		zipWriter := zip.NewWriter(writer)
		for _, item := range items {
			emailFile, err := os.Open(item.path)
			if err != nil {
				common.Verbose("Failed to open email during export: %v", err)
				continue
			}
			filename := fmt.Sprintf("%s_%s.eml", item.email.ID, sanitizeFilename(item.email.Subject))
			fileWriter, err := zipWriter.Create(filename)
			if err == nil {
				_, err = io.Copy(fileWriter, emailFile)
			}
			_ = emailFile.Close()
			if err != nil {
				common.Verbose("Failed to stream email into export: %v", err)
			}
		}
		if err := zipWriter.Close(); err != nil {
			common.Verbose("Failed to finish ZIP export: %v", err)
		}
	})
}

// applyEmailFilters applies filters to email list
func applyEmailFilters(emails []*types.Email, query, from, to, dateFrom, dateTo, read string) []*types.Email {
	filtered := make([]*types.Email, 0)
	for _, email := range emails {
		if query != "" {
			queryLower := strings.ToLower(query)
			matched := strings.Contains(strings.ToLower(email.Subject), queryLower) ||
				strings.Contains(strings.ToLower(email.Text), queryLower) ||
				strings.Contains(strings.ToLower(email.HTML), queryLower)
			if !matched {
				continue
			}
		}

		if from != "" {
			fromLower := strings.ToLower(from)
			matched := false
			for _, addr := range email.From {
				if strings.Contains(strings.ToLower(addr.Address), fromLower) ||
					strings.Contains(strings.ToLower(addr.Name), fromLower) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		if to != "" {
			toLower := strings.ToLower(to)
			matched := false
			for _, addr := range email.To {
				if strings.Contains(strings.ToLower(addr.Address), toLower) ||
					strings.Contains(strings.ToLower(addr.Name), toLower) {
					matched = true
					break
				}
			}
			if !matched {
				for _, addr := range email.CC {
					if strings.Contains(strings.ToLower(addr.Address), toLower) ||
						strings.Contains(strings.ToLower(addr.Name), toLower) {
						matched = true
						break
					}
				}
			}
			if !matched {
				for _, addr := range email.CalculatedBCC {
					if strings.Contains(strings.ToLower(addr.Address), toLower) {
						matched = true
						break
					}
				}
			}
			if !matched {
				continue
			}
		}

		if dateFrom != "" {
			dateFromTime, err := time.Parse("2006-01-02", dateFrom)
			if err == nil {
				if email.Time.Before(dateFromTime) {
					continue
				}
			}
		}
		if dateTo != "" {
			dateToTime, err := time.Parse("2006-01-02", dateTo)
			if err == nil {
				dateToTime = dateToTime.Add(24 * time.Hour)
				if email.Time.After(dateToTime) {
					continue
				}
			}
		}

		if read != "" {
			readBool := read == "true"
			if email.Read != readBool {
				continue
			}
		}

		filtered = append(filtered, email)
	}
	return filtered
}

// applyEmailSorting applies sorting to email list
func applyEmailSorting(emails []*types.Email, sortBy, sortOrder string) {
	switch sortBy {
	case "time":
		sort.Slice(emails, func(i, j int) bool {
			if sortOrder == "asc" {
				return emails[i].Time.Before(emails[j].Time)
			}
			return emails[i].Time.After(emails[j].Time)
		})
	case "subject":
		sort.Slice(emails, func(i, j int) bool {
			subjectI := strings.ToLower(emails[i].Subject)
			subjectJ := strings.ToLower(emails[j].Subject)
			if sortOrder == "asc" {
				return subjectI < subjectJ
			}
			return subjectI > subjectJ
		})
	case "from":
		sort.Slice(emails, func(i, j int) bool {
			fromI := ""
			fromJ := ""
			if len(emails[i].From) > 0 {
				fromI = strings.ToLower(emails[i].From[0].Address)
			}
			if len(emails[j].From) > 0 {
				fromJ = strings.ToLower(emails[j].From[0].Address)
			}
			if sortOrder == "asc" {
				return fromI < fromJ
			}
			return fromI > fromJ
		})
	case "size":
		sort.Slice(emails, func(i, j int) bool {
			if sortOrder == "asc" {
				return emails[i].Size < emails[j].Size
			}
			return emails[i].Size > emails[j].Size
		})
	}
}
