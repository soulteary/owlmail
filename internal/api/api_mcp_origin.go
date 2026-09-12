package api

import (
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// isMCPPath reports whether a request path reaches the MCP handler. The router
// is neither strict about a trailing slash nor case sensitive, so every
// spelling it dispatches must be recognized by the middleware that treats the
// endpoint specially. A byte-exact test would let "/MCP" reach the handler
// while the middleware still treated it as an ordinary route.
func (api *API) isMCPPath(path string) bool {
	route := api.route("/mcp")
	return strings.EqualFold(path, route) || strings.EqualFold(path, route+"/")
}

// isMCPPreflight reports a browser CORS preflight for the MCP endpoint. A
// preflight carries no credentials by design, so Basic Auth must not answer it
// with 401 before the endpoint's own origin policy can reply.
func (api *API) isMCPPreflight(c fiber.Ctx) bool {
	return api.mcpHandler != nil &&
		c.Method() == fiber.MethodOptions &&
		c.Get(fiber.HeaderOrigin) != "" &&
		c.Get(fiber.HeaderAccessControlRequestMethod) != "" &&
		api.isMCPPath(c.Path())
}

// SetMCPAllowedOrigins records extra browser origins accepted on the MCP
// endpoint, in addition to OwlMail's own browser-visible origins. A single
// "*" entry disables origin validation entirely and is an explicit,
// documented opt-out. It must be called before the API server starts.
func (api *API) SetMCPAllowedOrigins(origins []string) error {
	normalized, err := normalizeAllowedOrigins(origins, "MCP")
	if err != nil {
		return err
	}
	api.mcpAllowedOrigins = normalized
	return nil
}

// MCPAllowedOrigins returns the configured extra origins in the canonical form
// the guard compares against, which is also the form a browser sends. It is
// what an operator needs to read back when a request is refused; the values as
// written in configuration may be percent-encoded or spelled differently.
func (api *API) MCPAllowedOrigins() []string {
	origins := make([]string, len(api.mcpAllowedOrigins))
	copy(origins, api.mcpAllowedOrigins)
	return origins
}

// mcpOriginAllowList returns every origin accepted on the MCP endpoint: the
// configured extras plus the origins this listener answers on.
func (api *API) mcpOriginAllowList() []string {
	return api.originAllowList(api.mcpAllowedOrigins)
}

// mcpCORSAllowedHeaders are the request headers a browser MCP client sends:
// the Streamable HTTP session and protocol headers, resumption, negotiated
// content types, and Basic Auth.
const mcpCORSAllowedHeaders = "Authorization, Content-Type, Accept, Last-Event-ID, Mcp-Session-Id, Mcp-Protocol-Version"

// mcpCORSExposedHeaders are the response headers a browser MCP client must be
// able to read; without them a session cannot be established from a page.
const mcpCORSExposedHeaders = "Mcp-Session-Id, Mcp-Protocol-Version"

// mcpCORSAllowedMethods matches the documented MCP HTTP contract.
const mcpCORSAllowedMethods = "GET, POST, DELETE, OPTIONS"

// mcpCORSMaxAge caps how long a browser may reuse one preflight result.
const mcpCORSMaxAge = "600"

// mcpAllowsAnyOrigin reports the documented opt-out for this endpoint.
func (api *API) mcpAllowsAnyOrigin() bool {
	return allowsAnyOrigin(api.mcpAllowedOrigins)
}

// mcpOriginGuard is the MCP endpoint's complete browser policy: it validates
// the Origin header on every request, whether or not Web Basic Auth is
// configured, and answers allowed origins with an exact-origin CORS policy.
//
// Without it, any page the developer happens to visit can read the local test
// mailbox through /mcp, and an attacker who re-binds a hostname they control to
// the loopback address defeats a same-origin check that only compares Origin
// against the request's own Host header. Captured mail routinely holds
// password-reset links, verification codes, and tokens, so the MCP
// specification requires this check for local HTTP servers.
//
// On this path the guard also supersedes the global Web origin guard, which
// accepts any Origin that echoes the request's own Host -- what a re-bound
// hostname produces -- and consults a separate allow list. The list below is
// strictly narrower, and owning the whole policy here is what lets
// -mcp-allowed-origins actually work: an allowed origin needs matching CORS
// response headers, or the browser refuses to hand the response to the client.
// It also keeps -web-allowed-origins from opening this endpoint by accident.
//
// Requests without an Origin header are non-browser clients and stay allowed.
func (api *API) mcpOriginGuard() fiber.Handler {
	return func(c fiber.Ctx) error {
		// Every response from this path depends on the Origin header, the
		// refusals included, so a shared cache must never reuse one origin's
		// outcome for another.
		c.Response().Header.Add(fiber.HeaderVary, fiber.HeaderOrigin)

		// Deriving the allow list is pointless for the common case: every
		// non-browser client reaches this without an Origin header.
		origin := strings.TrimSpace(c.Get(fiber.HeaderOrigin))
		if origin == "" {
			return c.Next()
		}

		anyOrigin := api.mcpAllowsAnyOrigin()
		if !anyOrigin && !originAllowed(origin, api.mcpOriginAllowList()) {
			return c.Status(http.StatusForbidden).
				SendString("MCP request origin is not allowed; configure -mcp-allowed-origins to permit this browser origin")
		}

		if anyOrigin {
			// Validation is off, so no origin has been vouched for. Echoing the
			// caller and allowing credentials would hand every site a
			// credentialed grant -- strictly more than the wildcard CORS this
			// endpoint used to fall under, which browsers refuse to use with
			// credentials at all. The opt-out must not be an upgrade, so it
			// keeps that weaker, uncredentialed wildcard.
			c.Set(fiber.HeaderAccessControlAllowOrigin, allowAnyOrigin)
		} else {
			// The origin is one the operator named, so it is echoed exactly
			// rather than with a wildcard. That keeps the response unreadable by
			// any other site and is also what allows credentials to be sent.
			c.Set(fiber.HeaderAccessControlAllowOrigin, origin)
			c.Set(fiber.HeaderAccessControlAllowCredentials, "true")
		}
		c.Set(fiber.HeaderAccessControlExposeHeaders, mcpCORSExposedHeaders)

		if c.Method() == fiber.MethodOptions && c.Get(fiber.HeaderAccessControlRequestMethod) != "" {
			c.Set(fiber.HeaderAccessControlAllowMethods, mcpCORSAllowedMethods)
			c.Set(fiber.HeaderAccessControlAllowHeaders, mcpCORSAllowedHeaders)
			c.Set(fiber.HeaderAccessControlMaxAge, mcpCORSMaxAge)
			return c.SendStatus(http.StatusNoContent)
		}
		return c.Next()
	}
}
