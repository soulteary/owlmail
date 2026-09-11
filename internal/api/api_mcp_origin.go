package api

import (
	"fmt"
	"net"
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
// is not strict about a trailing slash, so both spellings must be recognised by
// the middleware that treats the endpoint specially.
func (api *API) isMCPPath(path string) bool {
	route := api.route("/mcp")
	return path == route || path == route+"/"
}

// originHost canonicalises a listen address for use in an origin. An operator
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
		add(scheme + "://" + net.JoinHostPort(host, strconv.Itoa(api.port)))
		// A browser omits the port when it is the scheme default, so the
		// port-less spelling of the same origin must match as well.
		if (scheme == "http" && api.port == 80) || (scheme == "https" && api.port == 443) {
			add(scheme + "://" + host)
		}
	}
	return allowed
}

func isWildcardBindHost(host string) bool {
	_, wildcard := wildcardBindHosts[host]
	return wildcard
}

// mcpOriginGuard validates the browser Origin header on every MCP request,
// whether or not Web Basic Auth is configured.
//
// Without it, any page the developer happens to visit can read the local test
// mailbox through /mcp: an unauthenticated deployment answers cross-origin
// requests with Access-Control-Allow-Origin: *, and an attacker who re-binds a
// hostname they control to the loopback address defeats a same-origin check
// that only compares Origin against the request's own Host header. Captured
// mail routinely holds password-reset links, verification codes, and tokens,
// so the MCP specification requires this check for local HTTP servers.
//
// On this path the guard also supersedes the global same-origin middleware,
// which only runs when Basic Auth is configured and accepts any Origin that
// echoes the request's own Host. The allow list below is strictly narrower, and
// routing /mcp through it alone keeps -mcp-allowed-origins meaningful on an
// authenticated deployment instead of being overruled before it is consulted.
//
// Requests without an Origin header are non-browser clients and stay allowed.
func (api *API) mcpOriginGuard() fiber.Handler {
	return func(c fiber.Ctx) error {
		// Deriving the allow list is pointless for the common case: every
		// non-browser client reaches this without an Origin header.
		if strings.TrimSpace(c.Get(fiber.HeaderOrigin)) == "" {
			return c.Next()
		}
		if originAllowed(c.Get(fiber.HeaderOrigin), api.mcpOriginAllowList()) {
			return c.Next()
		}
		return c.Status(http.StatusForbidden).
			SendString("MCP request origin is not allowed; configure -mcp-allowed-origins to permit this browser origin")
	}
}
