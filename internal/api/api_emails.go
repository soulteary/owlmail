package api

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/common"
	"github.com/soulteary/owlmail/internal/types"
)

// EmailPreview represents a lightweight email preview
type EmailPreview struct {
	ID            string    `json:"id"`
	Time          time.Time `json:"time"`
	Read          bool      `json:"read"`
	Subject       string    `json:"subject"`
	From          string    `json:"from"`
	To            []string  `json:"to"`
	Size          int64     `json:"size"`
	SizeHuman     string    `json:"sizeHuman"`
	HasAttachment bool      `json:"hasAttachment"`
	Preview       string    `json:"preview"` // First 200 chars of text
}

// getAllEmails handles GET /api/v1/emails
func (api *API) getAllEmails(c fiber.Ctx) error {
	limitStr := c.Query("limit", "50")
	offsetStr := c.Query("offset", "0")
	query := c.Query("q")
	from := c.Query("from")
	to := c.Query("to")
	dateFrom := c.Query("dateFrom")
	dateTo := c.Query("dateTo")
	read := c.Query("read")
	sortBy := c.Query("sortBy", "")
	sortOrder := c.Query("sortOrder", "desc")

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

	emails := api.mailServer.GetAllEmail()
	filtered := applyEmailFilters(emails, query, from, to, dateFrom, dateTo, read)

	if sortBy != "" {
		applyEmailSorting(filtered, sortBy, sortOrder)
	} else {
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].Time.After(filtered[j].Time)
		})
	}

	total := len(filtered)
	start := offset
	end := offset + limit
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	var paginatedEmails []*types.Email
	if start < end {
		paginatedEmails = filtered[start:end]
	} else {
		paginatedEmails = make([]*types.Email, 0)
	}

	return c.JSON(fiber.Map{
		"total":  total,
		"limit":  limit,
		"offset": offset,
		"emails": paginatedEmails,
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

	attachmentPath, contentType, err := api.mailServer.GetEmailAttachment(id, filename)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse(ErrorCodeEmailNotFound, err.Error()))
	}

	c.Set("Content-Type", contentType)
	return c.SendFile(attachmentPath)
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
	if err := api.mailServer.DeleteEmail(id); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse(ErrorCodeEmailNotFound, err.Error()))
	}
	return c.JSON(SuccessResponse(SuccessCodeEmailDeleted, "Email deleted", nil))
}

// deleteAllEmails handles DELETE /api/v1/emails
func (api *API) deleteAllEmails(c fiber.Ctx) error {
	if err := api.mailServer.DeleteAllEmail(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse(ErrorCodeInvalidRequest, err.Error()))
	}
	return c.JSON(SuccessResponse(SuccessCodeAllEmailsDeleted, "All emails deleted", nil))
}

// readAllEmails handles PATCH /api/v1/emails/read
func (api *API) readAllEmails(c fiber.Ctx) error {
	count := api.mailServer.ReadAllEmail()
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
	limitStr := c.Query("limit", "50")
	offsetStr := c.Query("offset", "0")
	query := c.Query("q")
	from := c.Query("from")
	to := c.Query("to")
	dateFrom := c.Query("dateFrom")
	dateTo := c.Query("dateTo")
	read := c.Query("read")
	sortBy := c.Query("sortBy", "")
	sortOrder := c.Query("sortOrder", "desc")

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

	emails := api.mailServer.GetAllEmail()
	filtered := applyEmailFilters(emails, query, from, to, dateFrom, dateTo, read)

	if sortBy != "" {
		applyEmailSorting(filtered, sortBy, sortOrder)
	} else {
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].Time.After(filtered[j].Time)
		})
	}

	total := len(filtered)
	start := offset
	end := offset + limit
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	var paginatedEmails []*types.Email
	if start < end {
		paginatedEmails = filtered[start:end]
	} else {
		paginatedEmails = make([]*types.Email, 0)
	}

	previews := make([]*EmailPreview, 0, len(paginatedEmails))
	for _, email := range paginatedEmails {
		preview := &EmailPreview{
			ID:            email.ID,
			Time:          email.Time,
			Read:          email.Read,
			Subject:       email.Subject,
			Size:          email.Size,
			SizeHuman:     email.SizeHuman,
			HasAttachment: len(email.Attachments) > 0,
		}

		if len(email.From) > 0 {
			preview.From = email.From[0].Address
		}

		preview.To = make([]string, 0, len(email.To))
		for _, addr := range email.To {
			preview.To = append(preview.To, addr.Address)
		}

		previewText := email.Text
		if previewText == "" {
			previewText = email.HTML
			previewText = strings.ReplaceAll(previewText, "<", " <")
			previewText = strings.ReplaceAll(previewText, ">", "> ")
			previewText = strings.ReplaceAll(previewText, "\n", " ")
			previewText = strings.ReplaceAll(previewText, "\r", " ")
			for strings.Contains(previewText, "  ") {
				previewText = strings.ReplaceAll(previewText, "  ", " ")
			}
			previewText = strings.TrimSpace(previewText)
		}
		if len(previewText) > 200 {
			previewText = previewText[:200] + "..."
		}
		preview.Preview = previewText

		previews = append(previews, preview)
	}

	return c.JSON(fiber.Map{
		"total":    total,
		"limit":    limit,
		"offset":   offset,
		"previews": previews,
	})
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

	for _, id := range request.IDs {
		if err := api.mailServer.DeleteEmail(id); err != nil {
			failedCount++
			failedIDs = append(failedIDs, id)
		} else {
			successCount++
		}
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

	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	for _, email := range filtered {
		emlPath, err := api.mailServer.GetRawEmail(email.ID)
		if err != nil {
			continue
		}

		emailFile, err := os.Open(emlPath)
		if err != nil {
			continue
		}

		filename := fmt.Sprintf("%s_%s.eml", email.ID, sanitizeFilename(email.Subject))
		fileWriter, err := zipWriter.Create(filename)
		if err != nil {
			_ = emailFile.Close()
			continue
		}

		_, err = io.Copy(fileWriter, emailFile)
		_ = emailFile.Close()
		if err != nil {
			continue
		}
	}

	if err := zipWriter.Close(); err != nil {
		common.Verbose("Failed to close zip writer: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse(ErrorCodeInvalidRequest, "Failed to create export"))
	}

	c.Set("Content-Type", "application/zip")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=emails_%s.zip", time.Now().Format("20060102_150405")))
	return c.Send(buf.Bytes())
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
