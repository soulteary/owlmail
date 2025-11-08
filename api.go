package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// API represents the REST API server
type API struct {
	mailServer *MailServer
	router     *gin.Engine
	port       int
	host       string
}

// NewAPI creates a new API server instance
func NewAPI(mailServer *MailServer, port int, host string) *API {
	api := &API{
		mailServer: mailServer,
		port:       port,
		host:       host,
	}
	api.setupRoutes()
	return api
}

// setupRoutes configures all API routes
func (api *API) setupRoutes() {
	router := gin.Default()

	// Enable CORS
	router.Use(corsMiddleware())

	// Email routes
	emailGroup := router.Group("/email")
	{
		// GET /email - Get all emails with pagination and filtering
		emailGroup.GET("", api.getAllEmails)

		// GET /email/:id - Get single email by ID
		emailGroup.GET("/:id", api.getEmailByID)

		// GET /email/:id/html - Get email HTML content
		emailGroup.GET("/:id/html", api.getEmailHTML)

		// GET /email/:id/attachment/:filename - Download attachment
		emailGroup.GET("/:id/attachment/:filename", api.getAttachment)

		// GET /email/:id/download - Download raw .eml file
		emailGroup.GET("/:id/download", api.downloadEmail)

		// GET /email/:id/source - Get email raw source
		emailGroup.GET("/:id/source", api.getEmailSource)

		// DELETE /email/:id - Delete single email
		emailGroup.DELETE("/:id", api.deleteEmail)

		// DELETE /email/all - Delete all emails
		emailGroup.DELETE("/all", api.deleteAllEmails)

		// PATCH /email/read-all - Mark all emails as read
		emailGroup.PATCH("/read-all", api.readAllEmails)
	}

	// Config route
	router.GET("/config", api.getConfig)

	// Health check route
	router.GET("/healthz", api.healthCheck)

	api.router = router
}

// Start starts the API server
func (api *API) Start() error {
	addr := fmt.Sprintf("%s:%d", api.host, api.port)
	return api.router.Run(addr)
}

// getAllEmails handles GET /email
func (api *API) getAllEmails(c *gin.Context) {
	// Get query parameters
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")
	query := c.Query("q") // Search query

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

	// Apply search filter if provided
	if query != "" {
		filtered := make([]*Email, 0)
		queryLower := strings.ToLower(query)
		for _, email := range emails {
			if strings.Contains(strings.ToLower(email.Subject), queryLower) ||
				strings.Contains(strings.ToLower(email.Text), queryLower) ||
				strings.Contains(strings.ToLower(email.HTML), queryLower) {
				filtered = append(filtered, email)
			}
		}
		emails = filtered
	}

	// Apply pagination
	total := len(emails)
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
		paginatedEmails = emails[start:end]
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

// getEmailByID handles GET /email/:id
func (api *API) getEmailByID(c *gin.Context) {
	id := c.Param("id")
	email, err := api.mailServer.GetEmail(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email not found"})
		return
	}
	c.JSON(http.StatusOK, email)
}

// getEmailHTML handles GET /email/:id/html
func (api *API) getEmailHTML(c *gin.Context) {
	id := c.Param("id")
	html, err := api.mailServer.GetEmailHTML(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email not found"})
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// getAttachment handles GET /email/:id/attachment/:filename
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

// downloadEmail handles GET /email/:id/download
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

// getEmailSource handles GET /email/:id/source
func (api *API) getEmailSource(c *gin.Context) {
	id := c.Param("id")

	content, err := api.mailServer.GetRawEmailContent(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Data(http.StatusOK, "text/plain; charset=utf-8", content)
}

// deleteEmail handles DELETE /email/:id
func (api *API) deleteEmail(c *gin.Context) {
	id := c.Param("id")
	if err := api.mailServer.DeleteEmail(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Email deleted"})
}

// deleteAllEmails handles DELETE /email/all
func (api *API) deleteAllEmails(c *gin.Context) {
	if err := api.mailServer.DeleteAllEmail(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "All emails deleted"})
}

// readAllEmails handles PATCH /email/read-all
func (api *API) readAllEmails(c *gin.Context) {
	count := api.mailServer.ReadAllEmail()
	c.JSON(http.StatusOK, gin.H{
		"message": "All emails marked as read",
		"count":   count,
	})
}

// getConfig handles GET /config
func (api *API) getConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"version": "1.0.0",
		"smtp": gin.H{
			"host": api.mailServer.host,
			"port": api.mailServer.port,
		},
		"web": gin.H{
			"host": api.host,
			"port": api.port,
		},
		"mailDir": api.mailServer.mailDir,
	})
}

// healthCheck handles GET /healthz
func (api *API) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

// corsMiddleware adds CORS headers
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// sanitizeFilename sanitizes a filename for safe download
func sanitizeFilename(filename string) string {
	// Remove or replace invalid characters
	filename = strings.ReplaceAll(filename, "/", "_")
	filename = strings.ReplaceAll(filename, "\\", "_")
	filename = strings.ReplaceAll(filename, ":", "_")
	filename = strings.ReplaceAll(filename, "*", "_")
	filename = strings.ReplaceAll(filename, "?", "_")
	filename = strings.ReplaceAll(filename, "\"", "_")
	filename = strings.ReplaceAll(filename, "<", "_")
	filename = strings.ReplaceAll(filename, ">", "_")
	filename = strings.ReplaceAll(filename, "|", "_")

	// Limit length
	if len(filename) > 100 {
		filename = filename[:100]
	}

	return filename
}
