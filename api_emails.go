package main

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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
func (api *API) getAllEmails(c *gin.Context) {
	// Get query parameters
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")
	query := c.Query("q")                            // Full text search query
	from := c.Query("from")                          // Filter by sender
	to := c.Query("to")                              // Filter by recipient
	dateFrom := c.Query("dateFrom")                  // Filter by date from (YYYY-MM-DD)
	dateTo := c.Query("dateTo")                      // Filter by date to (YYYY-MM-DD)
	read := c.Query("read")                          // Filter by read status (true/false)
	sortBy := c.DefaultQuery("sortBy", "")           // Sort by: time, subject
	sortOrder := c.DefaultQuery("sortOrder", "desc") // Sort order: asc, desc

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

	// Get all emails
	emails := api.mailServer.GetAllEmail()

	// Apply filters
	filtered := applyEmailFilters(emails, query, from, to, dateFrom, dateTo, read)

	// Apply sorting
	if sortBy != "" {
		applyEmailSorting(filtered, sortBy, sortOrder)
	} else {
		// Default: sort by time descending
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].Time.After(filtered[j].Time)
		})
	}

	// Apply pagination
	total := len(filtered)
	start := offset
	end := offset + limit
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	var paginatedEmails []*Email
	if start < end {
		paginatedEmails = filtered[start:end]
	} else {
		paginatedEmails = make([]*Email, 0)
	}

	c.JSON(http.StatusOK, gin.H{
		"total":  total,
		"limit":  limit,
		"offset": offset,
		"emails": paginatedEmails,
	})
}

// getEmailByID handles GET /api/v1/emails/:id
func (api *API) getEmailByID(c *gin.Context) {
	id := c.Param("id")
	email, err := api.mailServer.GetEmail(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email not found"})
		return
	}
	c.JSON(http.StatusOK, email)
}

// getEmailHTML handles GET /api/v1/emails/:id/html
func (api *API) getEmailHTML(c *gin.Context) {
	id := c.Param("id")
	html, err := api.mailServer.GetEmailHTML(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email not found"})
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// getAttachment handles GET /api/v1/emails/:id/attachments/:filename
func (api *API) getAttachment(c *gin.Context) {
	id := c.Param("id")
	filename := c.Param("filename")

	attachmentPath, contentType, err := api.mailServer.GetEmailAttachment(id, filename)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.File(attachmentPath)
	c.Header("Content-Type", contentType)
}

// downloadEmail handles GET /api/v1/emails/:id/raw
func (api *API) downloadEmail(c *gin.Context) {
	id := c.Param("id")

	email, err := api.mailServer.GetEmail(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email not found"})
		return
	}

	emlPath, err := api.mailServer.GetRawEmail(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email file not found"})
		return
	}

	// Set download headers
	filename := fmt.Sprintf("%s.eml", email.ID)
	if email.Subject != "" {
		// Sanitize filename
		filename = sanitizeFilename(email.Subject) + ".eml"
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.File(emlPath)
}

// getEmailSource handles GET /api/v1/emails/:id/source
func (api *API) getEmailSource(c *gin.Context) {
	id := c.Param("id")

	content, err := api.mailServer.GetRawEmailContent(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Data(http.StatusOK, "text/plain; charset=utf-8", content)
}

// deleteEmail handles DELETE /api/v1/emails/:id
func (api *API) deleteEmail(c *gin.Context) {
	id := c.Param("id")
	if err := api.mailServer.DeleteEmail(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Email deleted"})
}

// deleteAllEmails handles DELETE /api/v1/emails
func (api *API) deleteAllEmails(c *gin.Context) {
	if err := api.mailServer.DeleteAllEmail(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "All emails deleted"})
}

// readAllEmails handles PATCH /api/v1/emails/read
func (api *API) readAllEmails(c *gin.Context) {
	count := api.mailServer.ReadAllEmail()
	c.JSON(http.StatusOK, gin.H{
		"message": "All emails marked as read",
		"count":   count,
	})
}

// readEmail handles PATCH /api/v1/emails/:id/read
func (api *API) readEmail(c *gin.Context) {
	id := c.Param("id")
	if err := api.mailServer.ReadEmail(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Email marked as read",
		"id":      id,
	})
}

// getEmailStats handles GET /api/v1/emails/stats
func (api *API) getEmailStats(c *gin.Context) {
	stats := api.mailServer.GetEmailStats()
	c.JSON(http.StatusOK, stats)
}

// reloadMailsFromDirectory handles POST /api/v1/emails/reload
func (api *API) reloadMailsFromDirectory(c *gin.Context) {
	if err := api.mailServer.LoadMailsFromDirectory(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to reload mails from directory: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Mails reloaded from directory successfully",
	})
}

// getEmailPreviews handles GET /api/v1/emails/preview
func (api *API) getEmailPreviews(c *gin.Context) {
	// Get query parameters (same as getAllEmails but return previews)
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")
	query := c.Query("q")
	from := c.Query("from")
	to := c.Query("to")
	dateFrom := c.Query("dateFrom")
	dateTo := c.Query("dateTo")
	read := c.Query("read")
	sortBy := c.DefaultQuery("sortBy", "")
	sortOrder := c.DefaultQuery("sortOrder", "desc")

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

	// Get all emails
	emails := api.mailServer.GetAllEmail()

	// Apply filters (same logic as getAllEmails)
	filtered := applyEmailFilters(emails, query, from, to, dateFrom, dateTo, read)

	// Apply sorting (same as getAllEmails)
	if sortBy != "" {
		applyEmailSorting(filtered, sortBy, sortOrder)
	} else {
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].Time.After(filtered[j].Time)
		})
	}

	// Apply pagination
	total := len(filtered)
	start := offset
	end := offset + limit
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	var paginatedEmails []*Email
	if start < end {
		paginatedEmails = filtered[start:end]
	} else {
		paginatedEmails = make([]*Email, 0)
	}

	// Convert to previews
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

		// Get from address
		if len(email.From) > 0 {
			preview.From = email.From[0].Address
		}

		// Get to addresses
		preview.To = make([]string, 0, len(email.To))
		for _, addr := range email.To {
			preview.To = append(preview.To, addr.Address)
		}

		// Get preview text (first 200 chars)
		previewText := email.Text
		if previewText == "" {
			// Strip HTML tags for preview
			previewText = email.HTML
			previewText = strings.ReplaceAll(previewText, "<", " <")
			previewText = strings.ReplaceAll(previewText, ">", "> ")
			previewText = strings.ReplaceAll(previewText, "\n", " ")
			previewText = strings.ReplaceAll(previewText, "\r", " ")
			// Remove multiple spaces
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

	c.JSON(http.StatusOK, gin.H{
		"total":    total,
		"limit":    limit,
		"offset":   offset,
		"previews": previews,
	})
}

// batchDeleteEmails handles DELETE /api/v1/emails/batch
func (api *API) batchDeleteEmails(c *gin.Context) {
	var request struct {
		IDs []string `json:"ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	if len(request.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No email IDs provided"})
		return
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

	c.JSON(http.StatusOK, gin.H{
		"message":   "Batch delete completed",
		"success":   successCount,
		"failed":    failedCount,
		"failedIDs": failedIDs,
		"total":     len(request.IDs),
	})
}

// batchReadEmails handles PATCH /api/v1/emails/batch/read
func (api *API) batchReadEmails(c *gin.Context) {
	var request struct {
		IDs []string `json:"ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	if len(request.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No email IDs provided"})
		return
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
			email.Read = true
			successCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Batch read completed",
		"success":   successCount,
		"failed":    failedCount,
		"failedIDs": failedIDs,
		"total":     len(request.IDs),
	})
}

// exportEmails handles GET /api/v1/emails/export
func (api *API) exportEmails(c *gin.Context) {
	// Get query parameters for filtering
	idsParam := c.Query("ids") // Comma-separated list of IDs
	query := c.Query("q")
	from := c.Query("from")
	to := c.Query("to")
	dateFrom := c.Query("dateFrom")
	dateTo := c.Query("dateTo")
	read := c.Query("read")

	// Get all emails
	emails := api.mailServer.GetAllEmail()

	// Filter emails
	var filtered []*Email

	// If IDs are specified, only export those
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
		// Apply filters (same logic as getAllEmails)
		filtered = applyEmailFilters(emails, query, from, to, dateFrom, dateTo, read)
	}

	if len(filtered) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No emails found to export"})
		return
	}

	// Create ZIP file in memory
	c.Writer.Header().Set("Content-Type", "application/zip")
	c.Writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=emails_%s.zip", time.Now().Format("20060102_150405")))
	c.Writer.Header().Set("Content-Transfer-Encoding", "binary")

	zipWriter := zip.NewWriter(c.Writer)
	defer zipWriter.Close()

	// Add each email file to ZIP
	for _, email := range filtered {
		emlPath, err := api.mailServer.GetRawEmail(email.ID)
		if err != nil {
			continue // Skip if file not found
		}

		// Read email file
		emailFile, err := os.Open(emlPath)
		if err != nil {
			continue
		}

		// Create file in ZIP
		filename := fmt.Sprintf("%s_%s.eml", email.ID, sanitizeFilename(email.Subject))
		fileWriter, err := zipWriter.Create(filename)
		if err != nil {
			emailFile.Close()
			continue
		}

		// Copy file content
		_, err = io.Copy(fileWriter, emailFile)
		emailFile.Close()
		if err != nil {
			continue
		}
	}

	c.Writer.Flush()
}

// applyEmailFilters applies filters to email list
func applyEmailFilters(emails []*Email, query, from, to, dateFrom, dateTo, read string) []*Email {
	filtered := make([]*Email, 0)
	for _, email := range emails {
		// Full text search
		if query != "" {
			queryLower := strings.ToLower(query)
			matched := strings.Contains(strings.ToLower(email.Subject), queryLower) ||
				strings.Contains(strings.ToLower(email.Text), queryLower) ||
				strings.Contains(strings.ToLower(email.HTML), queryLower)
			if !matched {
				continue
			}
		}

		// Filter by sender
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

		// Filter by recipient
		if to != "" {
			toLower := strings.ToLower(to)
			matched := false
			// Check To addresses
			for _, addr := range email.To {
				if strings.Contains(strings.ToLower(addr.Address), toLower) ||
					strings.Contains(strings.ToLower(addr.Name), toLower) {
					matched = true
					break
				}
			}
			// Check CC addresses
			if !matched {
				for _, addr := range email.CC {
					if strings.Contains(strings.ToLower(addr.Address), toLower) ||
						strings.Contains(strings.ToLower(addr.Name), toLower) {
						matched = true
						break
					}
				}
			}
			// Check BCC addresses
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

		// Filter by date range
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
				// Add one day to include the end date
				dateToTime = dateToTime.Add(24 * time.Hour)
				if email.Time.After(dateToTime) {
					continue
				}
			}
		}

		// Filter by read status
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
func applyEmailSorting(emails []*Email, sortBy, sortOrder string) {
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
