package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"sync"

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
	httpsEnabled  bool
	httpsCertFile string
	httpsKeyFile  string
}

// NewAPI creates a new API server instance
func NewAPI(mailServer *MailServer, port int, host string) *API {
	return NewAPIWithAuth(mailServer, port, host, "", "")
}

// NewAPIWithAuth creates a new API server instance with HTTP Basic Auth
func NewAPIWithAuth(mailServer *MailServer, port int, host, user, password string) *API {
	return NewAPIWithHTTPS(mailServer, port, host, user, password, false, "", "")
}

// NewAPIWithHTTPS creates a new API server instance with HTTP Basic Auth and HTTPS support
func NewAPIWithHTTPS(mailServer *MailServer, port int, host, user, password string, httpsEnabled bool, certFile, keyFile string) *API {
	api := &API{
		mailServer:    mailServer,
		port:          port,
		host:          host,
		wsClients:     make(map[*websocket.Conn]bool),
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
		router.Use(basicAuthMiddleware(api.authUser, api.authPassword))
	}

	// Static files (web UI)
	router.StaticFile("/style.css", "./web/style.css")
	router.StaticFile("/app.js", "./web/app.js")

	// ============================================================================
	// MailDev-compatible API routes (maintains backward compatibility)
	// All MailDev compatibility code is in maildev.go
	// ============================================================================
	setupMailDevCompatibleRoutes(api, router)

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
