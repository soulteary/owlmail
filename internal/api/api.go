package api

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"sync"

	_ "github.com/emersion/go-message/charset"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/soulteary/owlmail/internal/mailserver"
	"github.com/soulteary/owlmail/internal/types"
)

// API represents the REST API server
type API struct {
	mailServer    *mailserver.MailServer
	router        *gin.Engine
	port          int
	host          string
	wsUpgrader    websocket.Upgrader
	wsClients     map[*websocket.Conn]*sync.Mutex
	wsClientsLock sync.RWMutex
	authUser      string
	authPassword  string
	httpsEnabled  bool
	httpsCertFile string
	httpsKeyFile  string
}

// NewAPI creates a new API server instance
func NewAPI(mailServer *mailserver.MailServer, port int, host string) *API {
	return NewAPIWithAuth(mailServer, port, host, "", "")
}

// NewAPIWithAuth creates a new API server instance with HTTP Basic Auth
func NewAPIWithAuth(mailServer *mailserver.MailServer, port int, host, user, password string) *API {
	return NewAPIWithHTTPS(mailServer, port, host, user, password, false, "", "")
}

// NewAPIWithHTTPS creates a new API server instance with HTTP Basic Auth and HTTPS support
func NewAPIWithHTTPS(mailServer *mailserver.MailServer, port int, host, user, password string, httpsEnabled bool, certFile, keyFile string) *API {
	api := &API{
		mailServer:    mailServer,
		port:          port,
		host:          host,
		wsClients:     make(map[*websocket.Conn]*sync.Mutex),
		authUser:      user,
		authPassword:  password,
		httpsEnabled:  httpsEnabled,
		httpsCertFile: certFile,
		httpsKeyFile:  keyFile,
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
// This function sets up both MailDev-compatible routes (for backward compatibility)
// and new improved RESTful API routes
func (api *API) setupRoutes() {
	router := gin.Default()

	// Enable CORS
	router.Use(corsMiddleware())

	// HTTP Basic Auth middleware if configured
	if api.authUser != "" && api.authPassword != "" {
		router.Use(basicAuthMiddleware(api.authUser, api.authPassword, "/healthz", "/api/v1/health"))
	}

	// Static files (web UI)
	router.StaticFile("/style.css", "./web/style.css")
	router.StaticFile("/app.js", "./web/app.js")

	// ============================================================================
	// MailDev-compatible API routes (maintains backward compatibility)
	// All MailDev compatibility code is in maildev.go
	// ============================================================================
	api.setupMailDevCompatibleRoutes(router)

	// ============================================================================
	// New improved RESTful API routes
	// ============================================================================
	api.setupImprovedAPIRoutes(router)

	// Serve index.html for root and all non-API routes
	router.NoRoute(func(c *gin.Context) {
		// Check if it's an API route
		if strings.HasPrefix(c.Request.URL.Path, "/email") ||
			strings.HasPrefix(c.Request.URL.Path, "/config") ||
			strings.HasPrefix(c.Request.URL.Path, "/healthz") ||
			strings.HasPrefix(c.Request.URL.Path, "/socket.io") ||
			strings.HasPrefix(c.Request.URL.Path, "/api/") ||
			strings.HasPrefix(c.Request.URL.Path, "/style.css") ||
			strings.HasPrefix(c.Request.URL.Path, "/app.js") {
			c.Next()
			return
		}
		// Serve index.html for all other routes
		c.File("./web/index.html")
	})

	// Root route - serve index.html
	router.GET("/", func(c *gin.Context) {
		c.File("./web/index.html")
	})

	api.router = router
}

// setupImprovedAPIRoutes sets up improved RESTful API routes
// These routes follow better RESTful design principles
func (api *API) setupImprovedAPIRoutes(router *gin.Engine) {
	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Emails resource (plural, more RESTful)
		emailsGroup := v1.Group("/emails")
		{
			// GET /api/v1/emails - Get all emails with pagination and filtering
			emailsGroup.GET("", api.getAllEmails)

			// GET /api/v1/emails/stats - Get email statistics
			emailsGroup.GET("/stats", api.getEmailStats)

			// GET /api/v1/emails/preview - Get email previews (lightweight)
			emailsGroup.GET("/preview", api.getEmailPreviews)

			// GET /api/v1/emails/export - Export emails as ZIP
			emailsGroup.GET("/export", api.exportEmails)

			// DELETE /api/v1/emails - Delete all emails (more RESTful than /email/all)
			emailsGroup.DELETE("", api.deleteAllEmails)

			// PATCH /api/v1/emails/read - Mark all emails as read (clearer than /read-all)
			emailsGroup.PATCH("/read", api.readAllEmails)

			// DELETE /api/v1/emails/batch - Batch delete emails (more RESTful)
			emailsGroup.DELETE("/batch", api.batchDeleteEmails)

			// PATCH /api/v1/emails/batch/read - Batch mark emails as read
			emailsGroup.PATCH("/batch/read", api.batchReadEmails)

			// POST /api/v1/emails/reload - Reload emails from directory (POST is more appropriate)
			emailsGroup.POST("/reload", api.reloadMailsFromDirectory)

			// Individual email routes
			emailsGroup.GET("/:id", api.getEmailByID)
			emailsGroup.DELETE("/:id", api.deleteEmail)
			emailsGroup.PATCH("/:id/read", api.readEmail)

			// Email content routes
			emailsGroup.GET("/:id/html", api.getEmailHTML)
			emailsGroup.GET("/:id/source", api.getEmailSource)
			emailsGroup.GET("/:id/raw", api.downloadEmail) // More semantic than /download

			// Email attachments (plural, more RESTful)
			emailsGroup.GET("/:id/attachments/:filename", api.getAttachment)

			// Email actions
			emailsGroup.POST("/:id/actions/relay", api.relayEmail)
			emailsGroup.POST("/:id/actions/relay/:relayTo", api.relayEmailWithParam)
		}

		// Settings resource (more semantic than /config)
		settingsGroup := v1.Group("/settings")
		{
			// GET /api/v1/settings - Get all settings
			settingsGroup.GET("", api.getConfig)

			// Outgoing mail settings
			settingsGroup.GET("/outgoing", api.getOutgoingConfig)
			settingsGroup.PUT("/outgoing", api.updateOutgoingConfig)
			settingsGroup.PATCH("/outgoing", api.patchOutgoingConfig)
		}

		// Health check (more standard than /healthz)
		v1.GET("/health", api.healthCheck)

		// WebSocket (clearer path)
		v1.GET("/ws", api.handleWebSocket)
	}
}

// Start starts the API server
func (api *API) Start() error {
	addr := fmt.Sprintf("%s:%d", api.host, api.port)

	if api.httpsEnabled {
		if api.httpsCertFile == "" || api.httpsKeyFile == "" {
			return fmt.Errorf("HTTPS enabled but certificate or key file not provided")
		}

		// Create HTTP server with TLS config
		srv := &http.Server{
			Addr:    addr,
			Handler: api.router,
			TLSConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		}

		// Logging is handled in main.go
		return srv.ListenAndServeTLS(api.httpsCertFile, api.httpsKeyFile)
	}

	// Logging is handled in main.go
	return api.router.Run(addr)
}

// setupEventListeners sets up event listeners for WebSocket broadcasting
func (api *API) setupEventListeners() {
	api.mailServer.On("new", func(email *types.Email) {
		api.broadcastMessage(gin.H{
			"type":  "new",
			"email": email,
		})
	})

	api.mailServer.On("delete", func(email *types.Email) {
		api.broadcastMessage(gin.H{
			"type": "delete",
			"id":   email.ID,
		})
	})
}

// setupMailDevCompatibleRoutes sets up MailDev-compatible API routes
// These routes maintain backward compatibility with MailDev
func (api *API) setupMailDevCompatibleRoutes(router *gin.Engine) {
	// Email routes (MailDev compatible)
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

		// POST /email/:id/relay/:relayTo - Relay email to SMTP server with specific recipient
		emailGroup.POST("/:id/relay/:relayTo", api.relayEmailWithParam)

		// GET /email/stats - Get email statistics
		emailGroup.GET("/stats", api.getEmailStats)

		// GET /email/preview - Get email previews (lightweight)
		emailGroup.GET("/preview", api.getEmailPreviews)

		// POST /email/batch/delete - Batch delete emails
		emailGroup.POST("/batch/delete", api.batchDeleteEmails)

		// POST /email/batch/read - Batch mark emails as read
		emailGroup.POST("/batch/read", api.batchReadEmails)

		// GET /email/export - Export emails as ZIP
		emailGroup.GET("/export", api.exportEmails)
	}

	// WebSocket route (MailDev compatible)
	router.GET("/socket.io", api.handleWebSocket)

	// Config routes (MailDev compatible)
	configGroup := router.Group("/config")
	{
		// GET /config - Get all configuration
		configGroup.GET("", api.getConfig)

		// GET /config/outgoing - Get outgoing mail configuration
		configGroup.GET("/outgoing", api.getOutgoingConfig)

		// PUT /config/outgoing - Update outgoing mail configuration
		configGroup.PUT("/outgoing", api.updateOutgoingConfig)

		// PATCH /config/outgoing - Partially update outgoing mail configuration
		configGroup.PATCH("/outgoing", api.patchOutgoingConfig)
	}

	// Health check route (MailDev compatible)
	router.GET("/healthz", api.healthCheck)

	// Reload mails from directory route (MailDev compatible)
	router.GET("/reloadMailsFromDirectory", api.reloadMailsFromDirectory)
}
