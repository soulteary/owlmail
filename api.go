package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// API represents the REST API server
type API struct {
	mailServer    *MailServer
	router        *gin.Engine
	port          int
	host          string
	wsUpgrader    websocket.Upgrader
	wsClients     map[*websocket.Conn]bool
	wsClientsLock sync.RWMutex
	authUser      string
	authPassword  string
}

// NewAPI creates a new API server instance
func NewAPI(mailServer *MailServer, port int, host string) *API {
	return NewAPIWithAuth(mailServer, port, host, "", "")
}

// NewAPIWithAuth creates a new API server instance with HTTP Basic Auth
func NewAPIWithAuth(mailServer *MailServer, port int, host, user, password string) *API {
	api := &API{
		mailServer:   mailServer,
		port:         port,
		host:         host,
		wsClients:    make(map[*websocket.Conn]bool),
		authUser:     user,
		authPassword: password,
		wsUpgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins
			},
		},
	}
	api.setupRoutes()
	api.setupEventListeners()
	return api
}

// setupRoutes configures all API routes
func (api *API) setupRoutes() {
	router := gin.Default()

	// Enable CORS
	router.Use(corsMiddleware())

	// HTTP Basic Auth middleware if configured
	if api.authUser != "" && api.authPassword != "" {
		router.Use(basicAuthMiddleware(api.authUser, api.authPassword))
	}

	// Static files (web UI)
	router.StaticFile("/style.css", "./web/style.css")
	router.StaticFile("/app.js", "./web/app.js")

	// Serve index.html for root and all non-API routes
	router.NoRoute(func(c *gin.Context) {
		// Check if it's an API route
		if strings.HasPrefix(c.Request.URL.Path, "/email") ||
			strings.HasPrefix(c.Request.URL.Path, "/config") ||
			strings.HasPrefix(c.Request.URL.Path, "/healthz") ||
			strings.HasPrefix(c.Request.URL.Path, "/socket.io") ||
			strings.HasPrefix(c.Request.URL.Path, "/style.css") ||
			strings.HasPrefix(c.Request.URL.Path, "/app.js") {
			c.Next()
			return
		}
		// Serve index.html for all other routes
		c.File("./web/index.html")
	})

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

		// PATCH /email/:id/read - Mark single email as read
		emailGroup.PATCH("/:id/read", api.readEmail)

		// POST /email/:id/relay - Relay email to SMTP server
		emailGroup.POST("/:id/relay", api.relayEmail)

		// GET /email/stats - Get email statistics
		emailGroup.GET("/stats", api.getEmailStats)
	}

	// WebSocket route
	router.GET("/socket.io", api.handleWebSocket)

	// Config route
	router.GET("/config", api.getConfig)

	// Health check route
	router.GET("/healthz", api.healthCheck)

	// Root route - serve index.html
	router.GET("/", func(c *gin.Context) {
		c.File("./web/index.html")
	})

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

	emails = filtered

	// Apply sorting
	if sortBy != "" {
		switch sortBy {
		case "time":
			if sortOrder == "asc" {
				// Sort by time ascending
				for i := 0; i < len(emails)-1; i++ {
					for j := i + 1; j < len(emails); j++ {
						if emails[i].Time.After(emails[j].Time) {
							emails[i], emails[j] = emails[j], emails[i]
						}
					}
				}
			} else {
				// Sort by time descending (default)
				for i := 0; i < len(emails)-1; i++ {
					for j := i + 1; j < len(emails); j++ {
						if emails[i].Time.Before(emails[j].Time) {
							emails[i], emails[j] = emails[j], emails[i]
						}
					}
				}
			}
		case "subject":
			if sortOrder == "asc" {
				// Sort by subject ascending
				for i := 0; i < len(emails)-1; i++ {
					for j := i + 1; j < len(emails); j++ {
						if strings.ToLower(emails[i].Subject) > strings.ToLower(emails[j].Subject) {
							emails[i], emails[j] = emails[j], emails[i]
						}
					}
				}
			} else {
				// Sort by subject descending
				for i := 0; i < len(emails)-1; i++ {
					for j := i + 1; j < len(emails); j++ {
						if strings.ToLower(emails[i].Subject) < strings.ToLower(emails[j].Subject) {
							emails[i], emails[j] = emails[j], emails[i]
						}
					}
				}
			}
		}
	} else {
		// Default: sort by time descending
		for i := 0; i < len(emails)-1; i++ {
			for j := i + 1; j < len(emails); j++ {
				if emails[i].Time.Before(emails[j].Time) {
					emails[i], emails[j] = emails[j], emails[i]
				}
			}
		}
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

// readEmail handles PATCH /email/:id/read
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

// getEmailStats handles GET /email/stats
func (api *API) getEmailStats(c *gin.Context) {
	stats := api.mailServer.GetEmailStats()
	c.JSON(http.StatusOK, stats)
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

// relayEmail handles POST /email/:id/relay
func (api *API) relayEmail(c *gin.Context) {
	id := c.Param("id")

	// Get optional relayTo parameter from query or body
	relayTo := c.Query("relayTo")
	if relayTo == "" {
		var body struct {
			RelayTo string `json:"relayTo"`
		}
		if err := c.ShouldBindJSON(&body); err == nil {
			relayTo = body.RelayTo
		}
	}

	// Get email
	email, err := api.mailServer.GetEmail(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email not found"})
		return
	}

	// Relay email
	var relayErr error
	if relayTo != "" {
		// Relay to specific address
		relayErr = api.mailServer.RelayMailTo(email, relayTo, func(err error) {
			if err != nil {
				log.Printf("Error relaying email %s to %s: %v", id, relayTo, err)
			}
		})
	} else {
		// Relay to configured SMTP server
		relayErr = api.mailServer.RelayMail(email, false, func(err error) {
			if err != nil {
				log.Printf("Error relaying email %s: %v", id, err)
			}
		})
	}

	if relayErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": relayErr.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Email relayed successfully",
		"relayTo": relayTo,
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

// setupEventListeners sets up event listeners for WebSocket broadcasting
func (api *API) setupEventListeners() {
	api.mailServer.On("new", func(email *Email) {
		api.broadcastMessage(gin.H{
			"type":  "new",
			"email": email,
		})
	})

	api.mailServer.On("delete", func(email *Email) {
		api.broadcastMessage(gin.H{
			"type": "delete",
			"id":   email.ID,
		})
	})
}

// handleWebSocket handles WebSocket connections
func (api *API) handleWebSocket(c *gin.Context) {
	conn, err := api.wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	// Add client
	api.wsClientsLock.Lock()
	api.wsClients[conn] = true
	api.wsClientsLock.Unlock()

	// Remove client on disconnect
	defer func() {
		api.wsClientsLock.Lock()
		delete(api.wsClients, conn)
		api.wsClientsLock.Unlock()
	}()

	// Send initial connection message
	conn.WriteJSON(gin.H{
		"type":    "connected",
		"message": "WebSocket connection established",
	})

	// Keep connection alive and handle incoming messages
	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Handle ping/pong
		if msgType, ok := msg["type"].(string); ok && msgType == "ping" {
			conn.WriteJSON(gin.H{"type": "pong"})
		}
	}
}

// broadcastMessage broadcasts a message to all connected WebSocket clients
func (api *API) broadcastMessage(message interface{}) {
	api.wsClientsLock.RLock()
	defer api.wsClientsLock.RUnlock()

	for conn := range api.wsClients {
		if err := conn.WriteJSON(message); err != nil {
			log.Printf("WebSocket write error: %v", err)
			// Remove failed client
			delete(api.wsClients, conn)
			conn.Close()
		}
	}
}

// basicAuthMiddleware creates HTTP Basic Auth middleware
func basicAuthMiddleware(username, password string) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" {
			c.Header("WWW-Authenticate", `Basic realm="OwlMail"`)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		// Parse Basic Auth
		const prefix = "Basic "
		if !strings.HasPrefix(auth, prefix) {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		decoded, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		credentials := strings.SplitN(string(decoded), ":", 2)
		if len(credentials) != 2 {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		if credentials[0] != username || credentials[1] != password {
			c.AbortWithStatus(http.StatusUnauthorized)
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
