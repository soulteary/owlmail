package api

import (
	"fmt"
	"html"
	"net/http"
	"strings"
	"sync"
	"time"

	_ "github.com/emersion/go-message/charset"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gorilla/websocket"
	"github.com/soulteary/health-kit/v2"
	"github.com/soulteary/owlmail/internal/attachmentstore"
	"github.com/soulteary/owlmail/internal/common"
	"github.com/soulteary/owlmail/internal/config"
	"github.com/soulteary/owlmail/internal/mailserver"
	"github.com/soulteary/owlmail/internal/types"
	webassets "github.com/soulteary/owlmail/web"
	"github.com/soulteary/version-kit/v2"
)

// API represents the REST API server
type API struct {
	mailServer              *mailserver.MailServer
	app                     *fiber.App
	port                    int
	host                    string
	wsUpgrader              websocket.Upgrader
	wsClients               map[*websocket.Conn]*sync.Mutex
	wsClientsLock           sync.RWMutex
	authUser                string
	authPassword            string
	httpsEnabled            bool
	httpsCertFile           string
	httpsKeyFile            string
	externalScheme          string
	basePathname            string
	mailDevRESTCompat       bool
	mailCatcherRESTCompat   bool
	metricsEnabled          bool
	metrics                 *prometheusMetrics
	mcpHandler              http.Handler
	relayJobs               *relayJobStore
	relayJobsPersistenceErr error
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
	relayJobs, err := newPersistentRelayJobStore(mailServer.GetMailDir())
	persistenceErr := err
	if err != nil {
		common.Error("Load persisted relay jobs: %v; relay status will be process-local", err)
		relayJobs = newRelayJobStore()
	}
	api := &API{
		mailServer:              mailServer,
		port:                    port,
		host:                    host,
		wsClients:               make(map[*websocket.Conn]*sync.Mutex),
		metrics:                 newPrometheusMetrics(),
		authUser:                user,
		authPassword:            password,
		httpsEnabled:            httpsEnabled,
		httpsCertFile:           certFile,
		httpsKeyFile:            keyFile,
		wsUpgrader:              websocket.Upgrader{},
		relayJobs:               relayJobs,
		relayJobsPersistenceErr: persistenceErr,
	}
	api.wsUpgrader.CheckOrigin = func(r *http.Request) bool {
		return !authEnabled || originMatchesRequest(r.Header.Get("Origin"), r.Host, api.requestScheme())
	}
	api.setupRoutes()
	api.setupEventListeners()
	if api.relayJobs.hasQueued() {
		go api.recoverRelayJobs()
	}
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

// SetBasePathname configures the browser-visible URL prefix and rebuilds the
// router before the server starts. Root is represented by an empty string.
func (api *API) SetBasePathname(basePathname string) error {
	normalized, err := config.NormalizeBasePathname(basePathname)
	if err != nil {
		return err
	}
	api.basePathname = normalized
	api.setupRoutes()
	return nil
}

// SetMailDevRESTCompat enables or disables the opt-in MailDev REST facade.
// It must be called before the API server starts.
func (api *API) SetMailDevRESTCompat(enabled bool) {
	api.mailDevRESTCompat = enabled
	api.mailServer.SetRetainAllHeaders(enabled)
	api.setupRoutes()
}

// SetMailCatcherRESTCompat enables or disables the opt-in MailCatcher facade.
func (api *API) SetMailCatcherRESTCompat(enabled bool) {
	api.mailCatcherRESTCompat = enabled
	api.setupRoutes()
}

// SetMetricsEnabled controls the opt-in Prometheus endpoint. The endpoint
// follows the configured base pathname and existing HTTP Basic Auth policy.
func (api *API) SetMetricsEnabled(enabled bool) {
	api.metricsEnabled = enabled
	api.setupRoutes()
}

// SetMCPHandler enables the optional MCP endpoint using the same listener,
// HTTPS configuration, base pathname, and Basic Auth middleware as the Web API.
func (api *API) SetMCPHandler(handler http.Handler) error {
	if handler == nil {
		return fmt.Errorf("MCP handler cannot be nil")
	}
	api.mcpHandler = handler
	api.setupRoutes()
	return nil
}

func (api *API) route(path string) string {
	return api.basePathname + path
}

// setupRoutes configures OwlMail's historical and versioned APIs, plus the
// default-off MailDev REST facade when explicitly enabled.
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
		healthRoutes := []string{api.route("/healthz"), api.route("/readyz"), api.route("/api/v1/health"), api.route("/api/v1/ready")}
		if api.mailDevRESTCompat {
			healthRoutes = append(healthRoutes, api.route("/api/healthz"))
		}
		if api.basePathname != "" {
			healthRoutes = append(healthRoutes, "/healthz")
		}
		app.Use(basicAuthMiddleware(api.authUser, api.authPassword, healthRoutes...))
	}
	if api.basePathname != "" {
		// Register the fixed image health check before the bare-base redirect.
		// This ordering matters when the configured base pathname is /healthz.
		app.Get("/healthz", adaptor.HTTPHandler(health.LivenessHandler("owlmail")))
	}

	// Static files are embedded in the executable so the UI and help page work
	// regardless of the process working directory.
	app.Get(api.route("/style.css"), api.serveWebAsset("style.css", "text/css; charset=utf-8"))
	app.Get(api.route("/app.js"), api.serveWebAsset("app.js", "text/javascript; charset=utf-8"))
	app.Get(api.route("/favicon.svg"), api.serveWebAsset("favicon.svg", "image/svg+xml"))
	app.Get(api.route("/service-worker.js"), func(c fiber.Ctx) error {
		c.Set("Service-Worker-Allowed", api.route("/"))
		return api.serveWebAsset("service-worker.js", "text/javascript; charset=utf-8")(c)
	})
	app.Get(api.route("/help.css"), api.serveWebAsset("help.css", "text/css; charset=utf-8"))
	app.Get(api.route("/help.js"), api.serveWebAsset("help.js", "text/javascript; charset=utf-8"))
	app.Get(api.route("/webhooks.css"), api.serveWebAsset("webhooks.css", "text/css; charset=utf-8"))
	app.Get(api.route("/webhooks.js"), api.serveWebAsset("webhooks.js", "text/javascript; charset=utf-8"))
	if api.basePathname != "" {
		app.Use(func(c fiber.Ctx) error {
			if c.Method() == http.MethodGet && c.Path() == api.basePathname {
				location := api.route("/")
				if query := c.Request().URI().QueryString(); len(query) != 0 {
					location += "?" + string(query)
				}
				return c.Redirect().Status(http.StatusPermanentRedirect).To(location)
			}
			return c.Next()
		})
	}

	// ============================================================================
	// MailDev-compatible API routes (maintains backward compatibility)
	// ============================================================================
	api.setupMailDevCompatibleRoutes(app)
	if api.mailDevRESTCompat {
		api.setupMailDevRESTCompatRoutes(app)
	}
	if api.mailCatcherRESTCompat {
		api.setupMailCatcherRESTCompatRoutes(app)
	}

	// ============================================================================
	// New improved RESTful API routes
	// ============================================================================
	api.setupImprovedAPIRoutes(app)
	if api.metricsEnabled {
		app.Get(api.route("/metrics"), api.prometheusMetrics)
	}
	if api.mcpHandler != nil {
		app.All(api.route("/mcp"), adaptor.HTTPHandler(api.mcpHandler))
	}

	// Browser UI and local help.
	app.Get(api.route("/"), api.serveWebAsset("index.html", "text/html; charset=utf-8"))
	app.Get(api.route("/help"), api.serveWebAsset("help.html", "text/html; charset=utf-8"))
	app.Get(api.route("/help/"), api.serveWebAsset("help.html", "text/html; charset=utf-8"))
	app.Get(api.route("/webhooks"), api.serveWebAsset("webhooks.html", "text/html; charset=utf-8"))
	app.Get(api.route("/webhooks/"), api.serveWebAsset("webhooks.html", "text/html; charset=utf-8"))

	// Serve index.html for all non-API routes (NoRoute equivalent)
	fallbackRoute := "*"
	if api.basePathname != "" {
		fallbackRoute = api.route("/*")
	}
	app.All(fallbackRoute, func(c fiber.Ctx) error {
		path := strings.TrimPrefix(c.Path(), api.basePathname)
		if strings.HasPrefix(path, "/email") ||
			strings.HasPrefix(path, "/config") ||
			strings.HasPrefix(path, "/healthz") ||
			strings.HasPrefix(path, "/readyz") ||
			strings.HasPrefix(path, "/socket.io") ||
			strings.HasPrefix(path, "/mcp") ||
			strings.HasPrefix(path, "/metrics") ||
			strings.HasPrefix(path, "/api/") ||
			strings.HasPrefix(path, "/messages") ||
			strings.HasPrefix(path, "/style.css") ||
			strings.HasPrefix(path, "/app.js") ||
			strings.HasPrefix(path, "/service-worker.js") ||
			strings.HasPrefix(path, "/favicon.svg") ||
			strings.HasPrefix(path, "/help.css") ||
			strings.HasPrefix(path, "/help.js") ||
			strings.HasPrefix(path, "/webhooks") {
			return c.Next()
		}
		return api.serveWebAsset("index.html", "text/html; charset=utf-8")(c)
	})

	api.app = app
}

// serveWebAsset returns a handler for a build-time embedded browser asset.
func (api *API) serveWebAsset(name, contentType string) fiber.Handler {
	return func(c fiber.Ctx) error {
		content, err := webassets.ReadFile(name)
		if err != nil {
			return c.Status(http.StatusInternalServerError).SendString("embedded web asset unavailable")
		}
		basePathname := api.basePathname
		if strings.HasSuffix(name, ".html") {
			basePathname = html.EscapeString(basePathname)
		}
		content = []byte(strings.ReplaceAll(string(content), "{{OWLMAIL_BASE_PATHNAME}}", basePathname))
		c.Set(fiber.HeaderContentType, contentType)
		return c.Send(content)
	}
}

// setupImprovedAPIRoutes sets up improved RESTful API routes
func (api *API) setupImprovedAPIRoutes(app *fiber.App) {
	v1 := app.Group(api.route("/api/v1"))

	// Read-only, machine-verifiable API contracts.
	v1.Get("/openapi.json", api.getOpenAPIJSON)
	v1.Get("/openapi.yaml", api.getOpenAPIYAML)

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
	emailsGroup.Post("/:id/actions/relay", api.relayEmailAsync)
	emailsGroup.Post("/:id/actions/relay/:relayTo", api.relayEmailWithParamAsync)
	v1.Get("/relay-jobs/:jobID", api.getRelayJob)

	// Settings resource
	settingsGroup := v1.Group("/settings")
	settingsGroup.Get("", api.getConfig)
	settingsGroup.Get("/outgoing", api.getOutgoingConfig)
	settingsGroup.Put("/outgoing", api.updateOutgoingConfig)
	settingsGroup.Patch("/outgoing", api.patchOutgoingConfig)

	// Health check (adaptor for health-kit)
	v1.Get("/health", adaptor.HTTPHandler(health.LivenessHandler("owlmail")))
	v1.Get("/ready", api.readiness)
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

// setupMailDevCompatibleRoutes sets up OwlMail's historical unversioned routes.
// These aliases resemble older MailDev workflows but are not the opt-in,
// contract-compatible /api facade.
func (api *API) setupMailDevCompatibleRoutes(app *fiber.App) {
	// Historical OwlMail email aliases.
	emailGroup := app.Group(api.route("/email"))
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

	// Historical native WebSocket alias. This is not Socket.IO compatible.
	app.Get(api.route("/socket.io"), adaptor.HTTPHandlerFunc(api.handleWebSocketHTTP))

	// Config routes (MailDev compatible)
	configGroup := app.Group(api.route("/config"))
	configGroup.Get("", api.getConfig)
	configGroup.Get("/outgoing", api.getOutgoingConfig)
	configGroup.Put("/outgoing", api.updateOutgoingConfig)
	configGroup.Patch("/outgoing", api.patchOutgoingConfig)

	// Health check route (MailDev compatible)
	app.Get(api.route("/healthz"), adaptor.HTTPHandler(health.LivenessHandler("owlmail")))
	app.Get(api.route("/readyz"), api.readiness)

	// Reload mails from directory route (MailDev compatible)
	app.Get(api.route("/reloadMailsFromDirectory"), api.reloadMailsFromDirectory)
}

type readinessCheck struct {
	Status        string                              `json:"status"`
	ErrorCategory attachmentstore.HealthErrorCategory `json:"error_category,omitempty"`
	CheckedAt     string                              `json:"checked_at,omitempty"`
}

type readinessResponse struct {
	Status string                    `json:"status"`
	Checks map[string]readinessCheck `json:"checks"`
}

func (api *API) readiness(c fiber.Ctx) error {
	response := readinessResponse{
		Status: "ready",
		Checks: map[string]readinessCheck{
			"attachment_store": {Status: "disabled"},
		},
	}
	status, enabled := api.mailServer.GetAttachmentHealth()
	if !enabled {
		return c.Status(http.StatusOK).JSON(response)
	}
	check := readinessCheck{Status: string(status.State), ErrorCategory: status.ErrorCategory}
	if !status.CheckedAt.IsZero() {
		check.CheckedAt = status.CheckedAt.Format(time.RFC3339Nano)
	}
	response.Checks["attachment_store"] = check
	if !status.Ready() {
		response.Status = "unready"
		return c.Status(http.StatusServiceUnavailable).JSON(response)
	}
	return c.Status(http.StatusOK).JSON(response)
}
