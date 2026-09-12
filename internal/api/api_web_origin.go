package api

import (
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// webCORSAllowedHeaders are the request headers the browser UI and the
// documented REST clients send. They are the same set the wildcard CORS policy
// used to advertise, so an operator who names an origin gets a client that
// works rather than one that fails on its first preflight.
const webCORSAllowedHeaders = "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, Accept, Origin, Cache-Control, X-Requested-With"

// webCORSAllowedMethods covers every method the REST API registers.
const webCORSAllowedMethods = "GET, POST, PUT, PATCH, DELETE, OPTIONS"

// webCORSMaxAge caps how long a browser may reuse one preflight result.
const webCORSMaxAge = "600"

// webCORSExposedHeaders names the response headers a cross-origin browser
// client may read. Only CORS-safelisted response headers are readable by
// default, and the relay routes answer 429 and 503 with Retry-After; without
// this the allowed client receives the body telling it to back off but not the
// interval it is supposed to wait.
const webCORSExposedHeaders = "Retry-After"

// SetWebAllowedOrigins records extra browser origins accepted on the Web UI and
// REST API, in addition to OwlMail's own browser-visible origins. A single "*"
// entry disables origin validation entirely and is an explicit, documented
// opt-out. It must be called before the API server starts.
func (api *API) SetWebAllowedOrigins(origins []string) error {
	normalized, err := normalizeAllowedOrigins(origins, "Web")
	if err != nil {
		return err
	}
	api.webAllowedOrigins = normalized
	return nil
}

// WebAllowedOrigins returns the configured extra origins in the canonical form
// the guard compares against, which is also the form a browser sends. It is
// what an operator needs to read back when a request is refused; the values as
// written in configuration may be percent-encoded or spelled differently.
func (api *API) WebAllowedOrigins() []string {
	origins := make([]string, len(api.webAllowedOrigins))
	copy(origins, api.webAllowedOrigins)
	return origins
}

func (api *API) webAllowsAnyOrigin() bool {
	return allowsAnyOrigin(api.webAllowedOrigins)
}

// webOriginAllowList returns the extra origins accepted on the Web UI and REST
// API beyond the request's own origin, which webOriginAllowed checks first.
func (api *API) webOriginAllowList() []string {
	return api.originAllowList(api.webAllowedOrigins)
}

// webOriginAllowed reports whether a browser origin may read this API. The
// request's own origin is accepted without being configured, which is what
// keeps the browser UI working behind a reverse proxy whose hostname OwlMail
// was never told about.
func (api *API) webOriginAllowed(origin, host string) bool {
	if originMatchesRequest(origin, host, api.requestScheme()) {
		return true
	}
	return originAllowed(origin, api.webOriginAllowList())
}

// webSocketOriginAllowed applies the same policy to the WebSocket upgrade.
// Browsers do not apply CORS to a WebSocket, so this handshake check is the
// only thing that stops a page the developer visits from subscribing to the
// live mail stream, which carries the same bodies the REST API returns.
func (api *API) webSocketOriginAllowed(request *http.Request) bool {
	origin := strings.TrimSpace(request.Header.Get(fiber.HeaderOrigin))
	if origin == "" || api.webAllowsAnyOrigin() {
		return true
	}
	return api.webOriginAllowed(origin, request.Host)
}

// webOriginGuard is the browser policy for everything this process serves
// outside /mcp: the UI, the REST API, the compatibility facades, the metrics
// and health endpoints, and the WebSocket upgrade.
//
// It validates the Origin header whether or not Basic Auth is configured.
// Basic Auth is off by default, so gating the check on it left the default
// deployment answering every origin with Access-Control-Allow-Origin: *, and a
// wildcard is the whole protection when there are no credentials to demand:
// any page the developer happens to visit could read the captured mailbox --
// password-reset links, verification codes, and tokens included -- repoint the
// outgoing SMTP relay at a host it controls, forward captured mail to an
// address it chose, and empty the mailbox. Binding to loopback does not help,
// because the browser making the request runs on that same host.
//
// Requests without an Origin header are non-browser clients -- curl, an HTTP
// library, a CI script, another server -- and stay allowed. That is what makes
// the check safe to turn on by default: only browsers send the header, and a
// browser is the only caller this policy is about.
func (api *API) webOriginGuard() fiber.Handler {
	return func(c fiber.Ctx) error {
		// The MCP endpoint owns a strictly narrower policy of its own. Answering
		// first here would overrule an origin the operator allowed there on
		// purpose, which is the same mistake the old Basic Auth branch made.
		if api.mcpHandler != nil && api.isMCPPath(c.Path()) {
			return c.Next()
		}

		// Every response below depends on the Origin header, the refusals
		// included, so a shared cache must never reuse one origin's outcome for
		// another.
		c.Response().Header.Add(fiber.HeaderVary, fiber.HeaderOrigin)

		origin := strings.TrimSpace(c.Get(fiber.HeaderOrigin))
		if origin == "" {
			return c.Next()
		}

		if api.webAllowsAnyOrigin() {
			// Validation is off, so no origin has been vouched for. Echoing the
			// caller and allowing credentials would hand every site a
			// credentialed grant, which is strictly more than the wildcard this
			// opt-out restores; browsers refuse to use a wildcard with
			// credentials at all.
			c.Set(fiber.HeaderAccessControlAllowOrigin, allowAnyOrigin)
			return webCORSPreflight(c)
		}

		sameOrigin := originMatchesRequest(origin, c.Host(), api.requestScheme())
		if !sameOrigin && !originAllowed(origin, api.webOriginAllowList()) {
			return c.Status(http.StatusForbidden).
				SendString("Web request origin is not allowed; configure -web-allowed-origins to permit this browser origin")
		}
		if !sameOrigin {
			// The origin is one the operator named, so it is echoed exactly
			// rather than with a wildcard. That keeps the response unreadable by
			// any other site and is also what allows credentials to be sent, so
			// an allowed browser client can authenticate against a deployment
			// that has Basic Auth on.
			c.Set(fiber.HeaderAccessControlAllowOrigin, origin)
			c.Set(fiber.HeaderAccessControlAllowCredentials, "true")
			c.Set(fiber.HeaderAccessControlExposeHeaders, webCORSExposedHeaders)
		}
		return webCORSPreflight(c)
	}
}

// webCORSPreflight answers an allowed preflight and otherwise hands the request
// on. The guard runs before Basic Auth so this answer is reached at all: a
// preflight carries no credentials, and a 401 would stop a browser origin the
// operator allowed on purpose from ever issuing the real request.
func webCORSPreflight(c fiber.Ctx) error {
	if c.Method() != fiber.MethodOptions || c.Get(fiber.HeaderAccessControlRequestMethod) == "" {
		return c.Next()
	}
	c.Set(fiber.HeaderAccessControlAllowMethods, webCORSAllowedMethods)
	c.Set(fiber.HeaderAccessControlAllowHeaders, webCORSAllowedHeaders)
	c.Set(fiber.HeaderAccessControlMaxAge, webCORSMaxAge)
	return c.SendStatus(http.StatusNoContent)
}
