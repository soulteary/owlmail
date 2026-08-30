package api

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	_ "github.com/emersion/go-message/charset"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gorilla/websocket"
	"github.com/soulteary/health-kit/v2"
	"github.com/soulteary/owlmail/internal/mailserver"
	"github.com/soulteary/owlmail/internal/types"
	webassets "github.com/soulteary/owlmail/web"
	"github.com/soulteary/version-kit/v2"
)

// API represents the REST API server
type API struct {
	mailServer     *mailserver.MailServer
	app            *fiber.App
	port           int
	host           string
	wsUpgrader     websocket.Upgrader
	wsClients      map[*websocket.Conn]*sync.Mutex
	wsClientsLock  sync.RWMutex
	authUser       string
	authPassword   string
	httpsEnabled   bool
	httpsCertFile  string
	httpsKeyFile   string
	externalScheme string
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
	authEnabled := user != "" && password != ""
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
		wsUpgrader:    websocket.Upgrader{},
	}
	api.wsUpgrader.CheckOrigin = func(r *http.Request) bool {
		return !authEnabled || originMatchesRequest(r.Header.Get("Origin"), r.Host, api.requestScheme())
	}
	api.setupRoutes()
	api.setupEventListeners()
	return api
}

func (api *API) requestScheme() string {
	if api.externalScheme != "" {
		return api.externalScheme
	}
	if api.httpsEnabled {
		return "https"
	}
	return "http"
}

// SetExternalScheme configures the browser-visible scheme when TLS terminates
// at a reverse proxy. It must be called before the API server starts.
func (api *API) SetExternalScheme(scheme string) error {
	scheme = strings.ToLower(strings.TrimSpace(scheme))
	if scheme != "" && scheme != "http" && scheme != "https" {
		return fmt.Errorf("external web scheme must be http or https")
	}
	api.externalScheme = scheme
	return nil
}

// setupRoutes configures all API routes
// This function sets up both MailDev-compatible routes (for backward compatibility)
// and new improved RESTful API routes
func (api *API) setupRoutes() {
	app := fiber.New(fiber.Config{})

	authEnabled := api.authUser != "" && api.authPassword != ""
	if authEnabled {
		// Browsers must not reuse cached Basic Auth credentials from an unrelated
		// origin. Non-browser API clients normally omit Origin and remain allowed.
		app.Use(func(c fiber.Ctx) error {
			return sameOriginMiddleware(api.requestScheme())(c)
		})
	} else {
		// Preserve the open development API's cross-origin compatibility. There
		// are no browser credentials to expose when authentication is disabled.
		app.Use(cors.New(cors.Config{
			AllowOrigins: []string{"*"},
			AllowHeaders: []string{"Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization", "accept", "origin", "Cache-Control", "X-Requested-With"},
			AllowMethods: []string{"POST", "OPTIONS", "GET", "PUT", "DELETE", "PATCH"},
		}))
	}

	// HTTP Basic Auth middleware if configured
	if authEnabled {
		app.Use(basicAuthMiddleware(api.authUser, api.authPassword, "/healthz", "/api/v1/health"))
	}

	// Static files are embedded in the executable so the UI and help page work
	// regardless of the process working directory.
	app.Get("/style.css", serveWebAsset("style.css", "text/css; charset=utf-8"))
	app.Get("/app.js", serveWebAsset("app.js", "text/javascript; charset=utf-8"))
	app.Get("/help.css", serveWebAsset("help.css", "text/css; charset=utf-8"))
	app.Get("/help.js", serveWebAsset("help.js", "text/javascript; charset=utf-8"))
	app.Get("/webhooks.css", serveWebAsset("webhooks.css", "text/css; charset=utf-8"))
	app.Get("/webhooks.js", serveWebAsset("webhooks.js", "text/javascript; charset=utf-8"))

	// ============================================================================
	// MailDev-compatible API routes (maintains backward compatibility)
	// ============================================================================
	api.setupMailDevCompatibleRoutes(app)

	// ============================================================================
	// New improved RESTful API routes
	// ============================================================================
	api.setupImprovedAPIRoutes(app)

	// Browser UI and local help.
	app.Get("/", serveWebAsset("index.html", "text/html; charset=utf-8"))
	app.Get("/help", serveWebAsset("help.html", "text/html; charset=utf-8"))
	app.Get("/help/", serveWebAsset("help.html", "text/html; charset=utf-8"))
	app.Get("/webhooks", serveWebAsset("webhooks.html", "text/html; charset=utf-8"))
	app.Get("/webhooks/", serveWebAsset("webhooks.html", "text/html; charset=utf-8"))

	// Serve index.html for all non-API routes (NoRoute equivalent)
	app.All("*", func(c fiber.Ctx) error {
		path := c.Path()
		if strings.HasPrefix(path, "/email") ||
			strings.HasPrefix(path, "/config") ||
			strings.HasPrefix(path, "/healthz") ||
			strings.HasPrefix(path, "/socket.io") ||
			strings.HasPrefix(path, "/api/") ||
			strings.HasPrefix(path, "/style.css") ||
			strings.HasPrefix(path, "/app.js") ||
			strings.HasPrefix(path, "/help.css") ||
			strings.HasPrefix(path, "/help.js") ||
			strings.HasPrefix(path, "/webhooks") {
			return c.Next()
		}
		return serveWebAsset("index.html", "text/html; charset=utf-8")(c)
	})

	api.app = app
}

// serveWebAsset returns a handler for a build-time embedded browser asset.
func serveWebAsset(name, contentType string) fiber.Handler {
	return func(c fiber.Ctx) error {
		content, err := webassets.ReadFile(name)
		if err != nil {
			return c.Status(http.StatusInternalServerError).SendString("embedded web asset unavailable")
		}
		c.Set(fiber.HeaderContentType, contentType)
		return c.Send(content)
	}
}

// setupImprovedAPIRoutes sets up improved RESTful API routes
func (api *API) setupImprovedAPIRoutes(app *fiber.App) {
	v1 := app.Group("/api/v1")

	// Emails resource
	emailsGroup := v1.Group("/emails")
	emailsGroup.Get("", api.getAllEmails)
	emailsGroup.Get("/stats", api.getEmailStats)
	emailsGroup.Get("/preview", api.getEmailPreviews)
	emailsGroup.Get("/export", api.exportEmails)
	emailsGroup.Delete("", api.deleteAllEmails)
	emailsGroup.Patch("/read", api.readAllEmails)
	emailsGroup.Delete("/batch", api.batchDeleteEmails)
	emailsGroup.Patch("/batch/read", api.batchReadEmails)
	emailsGroup.Post("/reload", api.reloadMailsFromDirectory)
	emailsGroup.Get("/:id", api.getEmailByID)
	emailsGroup.Delete("/:id", api.deleteEmail)
	emailsGroup.Patch("/:id/read", api.readEmail)
	emailsGroup.Get("/:id/html", api.getEmailHTML)
	emailsGroup.Get("/:id/source", api.getEmailSource)
	emailsGroup.Get("/:id/raw", api.downloadEmail)
	emailsGroup.Get("/:id/attachments/:filename", api.getAttachment)
	emailsGroup.Post("/:id/actions/relay", api.relayEmail)
	emailsGroup.Post("/:id/actions/relay/:relayTo", api.relayEmailWithParam)

	// Settings resource
	settingsGroup := v1.Group("/settings")
	settingsGroup.Get("", api.getConfig)
	settingsGroup.Get("/outgoing", api.getOutgoingConfig)
	settingsGroup.Put("/outgoing", api.updateOutgoingConfig)
	settingsGroup.Patch("/outgoing", api.patchOutgoingConfig)

	// Health check (adaptor for health-kit)
	v1.Get("/health", adaptor.HTTPHandler(health.LivenessHandler("owlmail")))
	// Version info (adaptor for version-kit)
	v1.Get("/version", adaptor.HTTPHandler(version.Handler()))
	// WebSocket (adaptor for gorilla/websocket Upgrade)
	v1.Get("/ws", adaptor.HTTPHandlerFunc(api.handleWebSocketHTTP))
}

// Start starts the API server
func (api *API) Start() error {
	addr := fmt.Sprintf("%s:%d", api.host, api.port)

	if api.httpsEnabled {
		if api.httpsCertFile == "" || api.httpsKeyFile == "" {
			return fmt.Errorf("HTTPS enabled but certificate or key file not provided")
		}
		return api.app.Listen(addr, fiber.ListenConfig{
			DisableStartupMessage: true,
			CertFile:              api.httpsCertFile,
			CertKeyFile:           api.httpsKeyFile,
		})
	}

	return api.app.Listen(addr, fiber.ListenConfig{DisableStartupMessage: true})
}

// setupEventListeners sets up event listeners for WebSocket broadcasting
func (api *API) setupEventListeners() {
	api.mailServer.On("new", func(email *types.Email) {
		api.broadcastMessage(fiber.Map{
			"type":  "new",
			"email": email,
		})
	})

	api.mailServer.On("delete", func(email *types.Email) {
		api.broadcastMessage(fiber.Map{
			"type": "delete",
			"id":   email.ID,
		})
	})
}

// setupMailDevCompatibleRoutes sets up MailDev-compatible API routes
func (api *API) setupMailDevCompatibleRoutes(app *fiber.App) {
	// Email routes (MailDev compatible)
	emailGroup := app.Group("/email")
	emailGroup.Get("", api.getAllEmails)
	emailGroup.Get("/:id", api.getEmailByID)
	emailGroup.Get("/:id/html", api.getEmailHTML)
	emailGroup.Get("/:id/attachment/:filename", api.getAttachment)
	emailGroup.Get("/:id/download", api.downloadEmail)
	emailGroup.Get("/:id/source", api.getEmailSource)
	emailGroup.Delete("/:id", api.deleteEmail)
	emailGroup.Delete("/all", api.deleteAllEmails)
	emailGroup.Patch("/read-all", api.readAllEmails)
	emailGroup.Patch("/:id/read", api.readEmail)
	emailGroup.Post("/:id/relay", api.relayEmail)
	emailGroup.Post("/:id/relay/:relayTo", api.relayEmailWithParam)
	emailGroup.Get("/stats", api.getEmailStats)
	emailGroup.Get("/preview", api.getEmailPreviews)
	emailGroup.Post("/batch/delete", api.batchDeleteEmails)
	emailGroup.Post("/batch/read", api.batchReadEmails)
	emailGroup.Get("/export", api.exportEmails)

	// WebSocket route (MailDev compatible)
	app.Get("/socket.io", adaptor.HTTPHandlerFunc(api.handleWebSocketHTTP))

	// Config routes (MailDev compatible)
	configGroup := app.Group("/config")
	configGroup.Get("", api.getConfig)
	configGroup.Get("/outgoing", api.getOutgoingConfig)
	configGroup.Put("/outgoing", api.updateOutgoingConfig)
	configGroup.Patch("/outgoing", api.patchOutgoingConfig)

	// Health check route (MailDev compatible)
	app.Get("/healthz", adaptor.HTTPHandler(health.LivenessHandler("owlmail")))

	// Reload mails from directory route (MailDev compatible)
	app.Get("/reloadMailsFromDirectory", api.reloadMailsFromDirectory)
}
