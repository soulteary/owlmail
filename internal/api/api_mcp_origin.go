package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// wildcardBindHosts are listen addresses that name every local interface
// rather than one browser-visible hostname. They cannot contribute a usable
// origin, so the loopback names are used instead.
var wildcardBindHosts = map[string]struct{}{
	"":        {},
	"0.0.0.0": {},
	"::":      {},
	"[::]":    {},
	"*":       {},
}

// loopbackOriginHosts are always accepted because OwlMail is documented as a
// loopback-first development and CI service.
var loopbackOriginHosts = []string{"localhost", "127.0.0.1", "::1"}

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

// originHost canonicalizes a listen address for use in an origin. An operator
// may spell an IPv6 address with brackets, and net.JoinHostPort adds its own,
// so they are stripped first rather than deriving "[[::1]]:1080".
func originHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	}
	return host
}

// SetMCPAllowedOrigins records extra browser origins accepted on the MCP
// endpoint, in addition to OwlMail's own browser-visible origins. A single
// "*" entry disables origin validation entirely and is an explicit,
// documented opt-out. It must be called before the API server starts.
func (api *API) SetMCPAllowedOrigins(origins []string) error {
	normalized := make([]string, 0, len(origins))
	for _, origin := range origins {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		if origin == mcpAllowAnyOrigin {
			normalized = append(normalized, mcpAllowAnyOrigin)
			continue
		}
		canonical, ok := normalizeOrigin(origin)
		if !ok {
			return fmt.Errorf("MCP allowed origin %q must be an absolute http or https origin", origin)
		}
		normalized = append(normalized, canonical)
	}
	api.mcpAllowedOrigins = normalized
	return nil
}

// mcpOriginAllowList returns every origin accepted on the MCP endpoint: the
// configured extras plus the origins this listener answers on. It is computed
// per request because the external scheme and base pathname are configured
// after the router is built, and only when a request actually carries an
// Origin header.
func (api *API) mcpOriginAllowList() []string {
	allowed := make([]string, 0, len(api.mcpAllowedOrigins)+8)
	seen := make(map[string]struct{}, len(api.mcpAllowedOrigins)+8)
	add := func(origin string) {
		if _, exists := seen[origin]; exists {
			return
		}
		seen[origin] = struct{}{}
		allowed = append(allowed, origin)
	}
	for _, origin := range api.mcpAllowedOrigins {
		add(origin)
	}

	scheme := api.requestScheme()
	hosts := make([]string, 0, len(loopbackOriginHosts)+1)
	if host := originHost(api.host); !isWildcardBindHost(host) {
		hosts = append(hosts, host)
	}
	hosts = append(hosts, loopbackOriginHosts...)

	for _, host := range hosts {
		add(canonicalOrigin(scheme, host, strconv.Itoa(api.port)))
	}
	return allowed
}

func isWildcardBindHost(host string) bool {
	_, wildcard := wildcardBindHosts[host]
	return wildcard
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

// mcpOriginGuard is the MCP endpoint's complete browser policy: it validates
// the Origin header on every request, whether or not Web Basic Auth is
// configured, and answers allowed origins with an exact-origin CORS policy.
//
// Without it, any page the developer happens to visit can read the local test
// mailbox through /mcp: an unauthenticated deployment answers cross-origin
// requests with Access-Control-Allow-Origin: *, and an attacker who re-binds a
// hostname they control to the loopback address defeats a same-origin check
// that only compares Origin against the request's own Host header. Captured
// mail routinely holds password-reset links, verification codes, and tokens,
// so the MCP specification requires this check for local HTTP servers.
//
// On this path the guard also supersedes both the wildcard CORS middleware and
// the global same-origin middleware. The wildcard would make every response
// readable by any site; the same-origin check only runs when Basic Auth is
// configured and accepts any Origin that echoes the request's own Host, which
// is what a re-bound hostname produces. The allow list below is strictly
// narrower than either, and owning the whole policy here is what lets
// -mcp-allowed-origins actually work: an allowed origin needs matching CORS
// response headers, or the browser refuses to hand the response to the client.
//
// Requests without an Origin header are non-browser clients and stay allowed.
// mcpAllowsAnyOrigin reports the documented opt-out, where the operator has
// turned origin validation off entirely rather than naming any origin.
func (api *API) mcpAllowsAnyOrigin() bool {
	for _, origin := range api.mcpAllowedOrigins {
		if origin == mcpAllowAnyOrigin {
			return true
		}
	}
	return false
}

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
			c.Set(fiber.HeaderAccessControlAllowOrigin, mcpAllowAnyOrigin)
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
